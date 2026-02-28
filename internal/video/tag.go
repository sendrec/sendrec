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

	query := `SELECT t.id, t.name, t.color, t.created_at,
		        (SELECT COUNT(*) FROM video_tags vt JOIN videos v ON v.id = vt.video_id WHERE vt.tag_id = t.id AND v.status != 'deleted') AS video_count
		 FROM tags t
		 WHERE 1=1`
	args := []any{}
	paramIdx := 1

	orgID := auth.OrgIDFromContext(r.Context())
	if orgID != "" {
		query += fmt.Sprintf(` AND t.organization_id = $%d`, paramIdx)
		args = append(args, orgID)
	} else {
		query += fmt.Sprintf(` AND t.user_id = $%d`, paramIdx)
		args = append(args, userID)
	}

	query += ` ORDER BY t.name`

	rows, err := h.db.Query(r.Context(), query, args...)
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

	orgID := auth.OrgIDFromContext(r.Context())

	var countQuery string
	var countArgs []any
	if orgID != "" {
		countQuery = `SELECT COUNT(*) FROM tags WHERE organization_id = $1`
		countArgs = []any{orgID}
	} else {
		countQuery = `SELECT COUNT(*) FROM tags WHERE user_id = $1`
		countArgs = []any{userID}
	}

	var count int
	if err := h.db.QueryRow(r.Context(), countQuery, countArgs...).Scan(&count); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to check tag limit")
		return
	}
	if count >= maxTagsPerUser {
		httputil.WriteError(w, http.StatusForbidden, "maximum of 100 tags reached")
		return
	}

	var orgIDArg *string
	if orgID != "" {
		orgIDArg = &orgID
	}

	var item tagItem
	var createdAt time.Time
	err := h.db.QueryRow(r.Context(),
		`INSERT INTO tags (user_id, organization_id, name, color)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		userID, orgIDArg, name, req.Color,
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

	orgID := auth.OrgIDFromContext(r.Context())
	var query string
	if orgID != "" {
		query = fmt.Sprintf("UPDATE tags SET %s WHERE id = $%d AND organization_id = $%d",
			strings.Join(setClauses, ", "), paramIdx, paramIdx+1)
		args = append(args, tagID, orgID)
	} else {
		query = fmt.Sprintf("UPDATE tags SET %s WHERE id = $%d AND user_id = $%d",
			strings.Join(setClauses, ", "), paramIdx, paramIdx+1)
		args = append(args, tagID, userID)
	}

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

	orgID := auth.OrgIDFromContext(r.Context())
	var deleteQuery string
	var deleteArgs []any
	if orgID != "" {
		deleteQuery = `DELETE FROM tags WHERE id = $1 AND organization_id = $2`
		deleteArgs = []any{tagID, orgID}
	} else {
		deleteQuery = `DELETE FROM tags WHERE id = $1 AND user_id = $2`
		deleteArgs = []any{tagID, userID}
	}

	result, err := h.db.Exec(r.Context(), deleteQuery, deleteArgs...)
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

	orgID := auth.OrgIDFromContext(r.Context())

	var videoVerifyQuery string
	var videoVerifyArgs []any
	if orgID != "" {
		videoVerifyQuery = `SELECT id FROM videos WHERE id = $1 AND organization_id = $2 AND status != 'deleted'`
		videoVerifyArgs = []any{videoID, orgID}
	} else {
		videoVerifyQuery = `SELECT id FROM videos WHERE id = $1 AND user_id = $2 AND status != 'deleted'`
		videoVerifyArgs = []any{videoID, userID}
	}

	var id string
	err := h.db.QueryRow(r.Context(), videoVerifyQuery, videoVerifyArgs...).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "video not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to verify video")
		return
	}

	if len(req.TagIDs) > 0 {
		var tagVerifyQuery string
		var tagVerifyArgs []any
		if orgID != "" {
			tagVerifyQuery = `SELECT COUNT(*) FROM tags WHERE id = ANY($1) AND organization_id = $2`
			tagVerifyArgs = []any{req.TagIDs, orgID}
		} else {
			tagVerifyQuery = `SELECT COUNT(*) FROM tags WHERE id = ANY($1) AND user_id = $2`
			tagVerifyArgs = []any{req.TagIDs, userID}
		}

		var count int
		err := h.db.QueryRow(r.Context(), tagVerifyQuery, tagVerifyArgs...).Scan(&count)
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
