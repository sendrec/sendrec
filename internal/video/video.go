package video

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
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
	HeadObject(ctx context.Context, key string) (int64, string, error)
	DownloadToFile(ctx context.Context, key string, destPath string) error
	UploadFile(ctx context.Context, key string, filePath string, contentType string) error
}

type Handler struct {
	db             database.DBTX
	storage        ObjectStorage
	baseURL        string
	maxUploadBytes int64
}

func videoFileKey(userID, shareToken string) string {
	return fmt.Sprintf("recordings/%s/%s.webm", userID, shareToken)
}

func NewHandler(db database.DBTX, s ObjectStorage, baseURL string, maxUploadBytes int64) *Handler {
	return &Handler{db: db, storage: s, baseURL: baseURL, maxUploadBytes: maxUploadBytes}
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
	ID              string `json:"id"`
	Title           string `json:"title"`
	Status          string `json:"status"`
	Duration        int    `json:"duration"`
	ShareToken      string `json:"shareToken"`
	ShareURL        string `json:"shareUrl"`
	CreatedAt       string `json:"createdAt"`
	ShareExpiresAt  string `json:"shareExpiresAt"`
	ViewCount       int64  `json:"viewCount"`
	UniqueViewCount int64  `json:"uniqueViewCount"`
	ThumbnailURL    string `json:"thumbnailUrl,omitempty"`
}

type updateRequest struct {
	Status string `json:"status"`
	Title  string `json:"title"`
}

type watchResponse struct {
	Title        string `json:"title"`
	VideoURL     string `json:"videoUrl"`
	Duration     int    `json:"duration"`
	Creator      string `json:"creator"`
	CreatedAt    string `json:"createdAt"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
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

	if req.FileSize <= 0 {
		httputil.WriteError(w, http.StatusBadRequest, "fileSize must be positive")
		return
	}

	if h.maxUploadBytes > 0 && req.FileSize > h.maxUploadBytes {
		httputil.WriteError(w, http.StatusBadRequest, "file too large")
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

	fileKey := videoFileKey(userID, shareToken)

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
		var fileKey string
		var fileSize int64
		var shareToken string
		err := h.db.QueryRow(r.Context(),
			`SELECT file_key, file_size, share_token FROM videos
			 WHERE id = $1 AND user_id = $2 AND status = 'uploading'`,
			videoID, userID,
		).Scan(&fileKey, &fileSize, &shareToken)
		if err != nil {
			httputil.WriteError(w, http.StatusNotFound, "video not found")
			return
		}

		size, contentType, err := h.storage.HeadObject(r.Context(), fileKey)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "could not verify upload")
			return
		}
		if size <= 0 || (h.maxUploadBytes > 0 && size > h.maxUploadBytes) {
			httputil.WriteError(w, http.StatusBadRequest, "uploaded file invalid size")
			return
		}
		if fileSize > 0 && size != fileSize {
			httputil.WriteError(w, http.StatusBadRequest, "uploaded file size mismatch")
			return
		}
		if contentType != "video/webm" {
			httputil.WriteError(w, http.StatusBadRequest, "uploaded file invalid type")
			return
		}

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

		go GenerateThumbnail(
			context.Background(),
			h.db, h.storage,
			videoID, fileKey,
			thumbnailFileKey(userID, shareToken),
		)
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
		`SELECT v.id, v.title, v.status, v.duration, v.share_token, v.created_at, v.share_expires_at,
		    (SELECT COUNT(*) FROM video_views vv WHERE vv.video_id = v.id) AS view_count,
		    (SELECT COUNT(DISTINCT vv.viewer_hash) FROM video_views vv WHERE vv.video_id = v.id) AS unique_view_count,
		    v.thumbnail_key
		 FROM videos v
		 WHERE v.user_id = $1 AND v.status != 'deleted'
		 ORDER BY v.created_at DESC
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
		var shareExpiresAt time.Time
		var thumbnailKey *string
		if err := rows.Scan(&item.ID, &item.Title, &item.Status, &item.Duration, &item.ShareToken, &createdAt, &shareExpiresAt, &item.ViewCount, &item.UniqueViewCount, &thumbnailKey); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to scan video")
			return
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		item.ShareExpiresAt = shareExpiresAt.Format(time.RFC3339)
		item.ShareURL = h.baseURL + "/watch/" + item.ShareToken
		if thumbnailKey != nil {
			thumbURL, err := h.storage.GenerateDownloadURL(r.Context(), *thumbnailKey, 1*time.Hour)
			if err == nil {
				item.ThumbnailURL = thumbURL
			}
		}
		items = append(items, item)
	}

	httputil.WriteJSON(w, http.StatusOK, items)
}

func deleteWithRetry(ctx context.Context, storage ObjectStorage, key string, maxAttempts int) error {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
		lastErr = storage.DeleteObject(ctx, key)
		if lastErr == nil {
			return nil
		}
		log.Printf("delete attempt %d/%d failed for %s: %v", attempt+1, maxAttempts, key, lastErr)
	}
	return fmt.Errorf("all %d delete attempts failed for %s: %w", maxAttempts, key, lastErr)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var fileKey string
	var thumbnailKey *string
	err := h.db.QueryRow(r.Context(),
		`UPDATE videos SET status = 'deleted', updated_at = now()
		 WHERE id = $1 AND user_id = $2 AND status != 'deleted'
		 RETURNING file_key, thumbnail_key`,
		videoID, userID,
	).Scan(&fileKey, &thumbnailKey)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	go func() {
		ctx := context.Background()
		if err := deleteWithRetry(ctx, h.storage, fileKey, 3); err != nil {
			log.Printf("all delete retries failed for %s: %v", fileKey, err)
			return
		}
		if thumbnailKey != nil {
			if err := deleteWithRetry(ctx, h.storage, *thumbnailKey, 3); err != nil {
				log.Printf("thumbnail delete failed for %s: %v", *thumbnailKey, err)
			}
		}
		if _, err := h.db.Exec(ctx,
			`UPDATE videos SET file_purged_at = now() WHERE file_key = $1`,
			fileKey,
		); err != nil {
			log.Printf("failed to mark file_purged_at for %s: %v", fileKey, err)
		}
	}()

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Watch(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var videoID string
	var title string
	var duration int
	var fileKey string
	var creator string
	var createdAt time.Time
	var shareExpiresAt time.Time

	var thumbnailKey *string

	err := h.db.QueryRow(r.Context(),
		`SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key
		 FROM videos v
		 JOIN users u ON u.id = v.user_id
		 WHERE v.share_token = $1 AND v.status = 'ready'`,
		shareToken,
	).Scan(&videoID, &title, &duration, &fileKey, &creator, &createdAt, &shareExpiresAt, &thumbnailKey)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	if time.Now().After(shareExpiresAt) {
		httputil.WriteError(w, http.StatusGone, "link expired")
		return
	}

	go func() {
		ip := clientIP(r)
		hash := viewerHash(ip, r.UserAgent())
		if _, err := h.db.Exec(context.Background(),
			`INSERT INTO video_views (video_id, viewer_hash) VALUES ($1, $2)`,
			videoID, hash,
		); err != nil {
			log.Printf("failed to record view for %s: %v", videoID, err)
		}
	}()

	videoURL, err := h.storage.GenerateDownloadURL(r.Context(), fileKey, 1*time.Hour)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate video URL")
		return
	}

	var thumbnailURL string
	if thumbnailKey != nil {
		if u, err := h.storage.GenerateDownloadURL(r.Context(), *thumbnailKey, 1*time.Hour); err == nil {
			thumbnailURL = u
		}
	}

	httputil.WriteJSON(w, http.StatusOK, watchResponse{
		Title:        title,
		VideoURL:     videoURL,
		Duration:     duration,
		Creator:      creator,
		CreatedAt:    createdAt.Format(time.RFC3339),
		ThumbnailURL: thumbnailURL,
	})
}

func viewerHash(ip, userAgent string) string {
	h := sha256.Sum256([]byte(ip + "|" + userAgent))
	return fmt.Sprintf("%x", h[:8])
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if first, _, ok := strings.Cut(forwarded, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(forwarded)
	}
	return r.RemoteAddr
}

func (h *Handler) Extend(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET share_expires_at = now() + INTERVAL '7 days', updated_at = now()
		 WHERE id = $1 AND user_id = $2 AND status != 'deleted'`,
		videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to extend share link")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
