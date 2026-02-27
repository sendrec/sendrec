package video

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/validate"
)

type tagItem struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Color      *string `json:"color"`
	VideoCount int64   `json:"videoCount"`
	CreatedAt  string  `json:"createdAt"`
}

const maxTagsPerUser = 100

var colorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	rows, err := h.db.Query(r.Context(),
		`SELECT t.id, t.name, t.color, t.created_at,
		        (SELECT COUNT(*) FROM video_tags vt JOIN videos v ON v.id = vt.video_id WHERE vt.tag_id = t.id AND v.status != 'deleted') AS video_count
		 FROM tags t
		 WHERE t.user_id = $1
		 ORDER BY t.name`,
		userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list tags")
		return
	}
	defer rows.Close()

	items := make([]tagItem, 0)
	for rows.Next() {
		var item tagItem
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Name, &item.Color, &createdAt, &item.VideoCount); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to scan tag")
			return
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		items = append(items, item)
	}

	httputil.WriteJSON(w, http.StatusOK, items)
}

type createTagRequest struct {
	Name  string  `json:"name"`
	Color *string `json:"color"`
}

func (h *Handler) CreateTag(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req createTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "tag name is required")
		return
	}
	if msg := validate.TagName(name); msg != "" {
		httputil.WriteError(w, http.StatusBadRequest, msg)
		return
	}

	if req.Color != nil && !colorRegex.MatchString(*req.Color) {
		httputil.WriteError(w, http.StatusBadRequest, "color must be a valid hex color (e.g. #ff0000)")
		return
	}

	var count int
	if err := h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM tags WHERE user_id = $1`,
		userID,
	).Scan(&count); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to check tag limit")
		return
	}
	if count >= maxTagsPerUser {
		httputil.WriteError(w, http.StatusForbidden, "maximum of 100 tags reached")
		return
	}

	var item tagItem
	var createdAt time.Time
	err := h.db.QueryRow(r.Context(),
		`INSERT INTO tags (user_id, name, color)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		userID, name, req.Color,
	).Scan(&item.ID, &createdAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httputil.WriteError(w, http.StatusConflict, "a tag with this name already exists")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create tag")
		return
	}

	item.Name = name
	item.Color = req.Color
	item.CreatedAt = createdAt.Format(time.RFC3339)

	httputil.WriteJSON(w, http.StatusCreated, item)
}

type updateTagRequest struct {
	Name  *string `json:"name"`
	Color *string `json:"color"`
}

func (h *Handler) UpdateTag(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	tagID := chi.URLParam(r, "id")

	var req updateTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == nil && req.Color == nil {
		httputil.WriteError(w, http.StatusBadRequest, "nothing to update")
		return
	}

	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			httputil.WriteError(w, http.StatusBadRequest, "tag name is required")
			return
		}
		if msg := validate.TagName(trimmed); msg != "" {
			httputil.WriteError(w, http.StatusBadRequest, msg)
			return
		}
		req.Name = &trimmed
	}

	if req.Color != nil && !colorRegex.MatchString(*req.Color) {
		httputil.WriteError(w, http.StatusBadRequest, "color must be a valid hex color (e.g. #ff0000)")
		return
	}

	setClauses := []string{}
	args := []any{}
	paramIdx := 1

	if req.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", paramIdx))
		args = append(args, *req.Name)
		paramIdx++
	}
	if req.Color != nil {
		setClauses = append(setClauses, fmt.Sprintf("color = $%d", paramIdx))
		args = append(args, *req.Color)
		paramIdx++
	}

	query := fmt.Sprintf("UPDATE tags SET %s WHERE id = $%d AND user_id = $%d",
		strings.Join(setClauses, ", "), paramIdx, paramIdx+1)
	args = append(args, tagID, userID)

	result, err := h.db.Exec(r.Context(), query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httputil.WriteError(w, http.StatusConflict, "a tag with this name already exists")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update tag")
		return
	}
	if result.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "tag not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	tagID := chi.URLParam(r, "id")

	result, err := h.db.Exec(r.Context(),
		`DELETE FROM tags WHERE id = $1 AND user_id = $2`,
		tagID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to delete tag")
		return
	}
	if result.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "tag not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

const maxTagsPerVideo = 10

type setVideoTagsRequest struct {
	TagIDs []string `json:"tagIds"`
}

func (h *Handler) SetVideoTags(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req setVideoTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TagIDs == nil {
		req.TagIDs = []string{}
	}

	if len(req.TagIDs) > maxTagsPerVideo {
		httputil.WriteError(w, http.StatusBadRequest, "maximum 10 tags per video")
		return
	}

	var id string
	err := h.db.QueryRow(r.Context(),
		`SELECT id FROM videos WHERE id = $1 AND user_id = $2 AND status != 'deleted'`,
		videoID, userID,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "video not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to verify video")
		return
	}

	if len(req.TagIDs) > 0 {
		var count int
		err := h.db.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM tags WHERE id = ANY($1) AND user_id = $2`,
			req.TagIDs, userID,
		).Scan(&count)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to verify tags")
			return
		}
		if count != len(req.TagIDs) {
			httputil.WriteError(w, http.StatusBadRequest, "one or more tags not found")
			return
		}
	}

	if _, err := h.db.Exec(r.Context(),
		`DELETE FROM video_tags WHERE video_id = $1`,
		videoID,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update video tags")
		return
	}

	for _, tagID := range req.TagIDs {
		if _, err := h.db.Exec(r.Context(),
			`INSERT INTO video_tags (video_id, tag_id) VALUES ($1, $2)`,
			videoID, tagID,
		); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update video tags")
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
