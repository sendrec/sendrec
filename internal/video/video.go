package video

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/httputil"
)

type ObjectStorage interface {
	GenerateUploadURL(ctx context.Context, key string, contentType string, contentLength int64, expiry time.Duration) (string, error)
	GenerateDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	DeleteObject(ctx context.Context, key string) error
}

type Handler struct {
	db      database.DBTX
	storage ObjectStorage
	baseURL string
}

func NewHandler(db database.DBTX, s ObjectStorage, baseURL string) *Handler {
	return &Handler{db: db, storage: s, baseURL: baseURL}
}

type createRequest struct {
	Title    string `json:"title"`
	Duration int    `json:"duration"`
	FileSize int64  `json:"fileSize"`
}

type createResponse struct {
	ID         string `json:"id"`
	UploadURL  string `json:"uploadUrl"`
	ShareToken string `json:"shareToken"`
}

type listItem struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	Duration   int    `json:"duration"`
	ShareToken string `json:"shareToken"`
	ShareURL   string `json:"shareUrl"`
	CreatedAt  string `json:"createdAt"`
}

type updateRequest struct {
	Status string `json:"status"`
	Title  string `json:"title"`
}

type watchResponse struct {
	Title    string `json:"title"`
	VideoURL string `json:"videoUrl"`
	Duration int    `json:"duration"`
	Creator  string `json:"creator"`
	CreatedAt string `json:"createdAt"`
}

func generateShareToken() (string, error) {
	b := make([]byte, 9)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	title := req.Title
	if title == "" {
		title = "Untitled Recording"
	}

	shareToken, err := generateShareToken()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate share token")
		return
	}

	fileKey := fmt.Sprintf("recordings/%s/%s.webm", userID, shareToken)

	var videoID string
	err = h.db.QueryRow(r.Context(),
		`INSERT INTO videos (user_id, title, duration, file_size, file_key, share_token)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		userID, title, req.Duration, req.FileSize, fileKey, shareToken,
	).Scan(&videoID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create video")
		return
	}

	uploadURL, err := h.storage.GenerateUploadURL(r.Context(), fileKey, "video/webm", req.FileSize, 30*time.Minute)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate upload URL")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, createResponse{
		ID:         videoID,
		UploadURL:  uploadURL,
		ShareToken: shareToken,
	})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Status != "" && req.Status != "ready" {
		httputil.WriteError(w, http.StatusBadRequest, "status can only be set to ready")
		return
	}

	if req.Status == "" && req.Title == "" {
		httputil.WriteError(w, http.StatusBadRequest, "nothing to update")
		return
	}

	if req.Status == "ready" {
		tag, err := h.db.Exec(r.Context(),
			`UPDATE videos SET status = $1, updated_at = now()
			 WHERE id = $2 AND user_id = $3 AND status = 'uploading'`,
			req.Status, videoID, userID,
		)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update video")
			return
		}
		if tag.RowsAffected() == 0 {
			httputil.WriteError(w, http.StatusNotFound, "video not found")
			return
		}
	}

	if req.Title != "" {
		tag, err := h.db.Exec(r.Context(),
			`UPDATE videos SET title = $1, updated_at = now()
			 WHERE id = $2 AND user_id = $3 AND status != 'deleted'`,
			req.Title, videoID, userID,
		)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update video")
			return
		}
		if tag.RowsAffected() == 0 {
			httputil.WriteError(w, http.StatusNotFound, "video not found")
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

const defaultPageSize = 50
const maxPageSize = 100

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	limit := defaultPageSize
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	offset := 0
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o > 0 {
		offset = o
	}

	rows, err := h.db.Query(r.Context(),
		`SELECT id, title, status, duration, share_token, created_at
		 FROM videos
		 WHERE user_id = $1 AND status != 'deleted'
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list videos")
		return
	}
	defer rows.Close()

	items := []listItem{}
	for rows.Next() {
		var item listItem
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Title, &item.Status, &item.Duration, &item.ShareToken, &createdAt); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to scan video")
			return
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		item.ShareURL = h.baseURL + "/watch/" + item.ShareToken
		items = append(items, item)
	}

	httputil.WriteJSON(w, http.StatusOK, items)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var fileKey string
	err := h.db.QueryRow(r.Context(),
		`UPDATE videos SET status = 'deleted', updated_at = now()
		 WHERE id = $1 AND user_id = $2 AND status != 'deleted'
		 RETURNING file_key`,
		videoID, userID,
	).Scan(&fileKey)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	go func() {
		_ = h.storage.DeleteObject(context.Background(), fileKey)
	}()

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Watch(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var title string
	var duration int
	var fileKey string
	var creator string
	var createdAt time.Time

	err := h.db.QueryRow(r.Context(),
		`SELECT v.title, v.duration, v.file_key, u.name, v.created_at
		 FROM videos v
		 JOIN users u ON u.id = v.user_id
		 WHERE v.share_token = $1 AND v.status = 'ready'`,
		shareToken,
	).Scan(&title, &duration, &fileKey, &creator, &createdAt)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	videoURL, err := h.storage.GenerateDownloadURL(r.Context(), fileKey, 1*time.Hour)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate video URL")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, watchResponse{
		Title:     title,
		VideoURL:  videoURL,
		Duration:  duration,
		Creator:   creator,
		CreatedAt: createdAt.Format(time.RFC3339),
	})
}

