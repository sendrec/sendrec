package video

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/validate"
)

type folderItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Position   int    `json:"position"`
	VideoCount int64  `json:"videoCount"`
	CreatedAt  string `json:"createdAt"`
}

const maxFoldersPerUser = 50

func (h *Handler) ListFolders(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	query := `SELECT f.id, f.name, f.position, f.created_at,
		        (SELECT COUNT(*) FROM videos v WHERE v.folder_id = f.id AND v.status != 'deleted') AS video_count
		 FROM folders f
		 WHERE 1=1`
	args := []any{}
	paramIdx := 1

	orgID := auth.OrgIDFromContext(r.Context())
	if orgID != "" {
		query += fmt.Sprintf(` AND f.organization_id = $%d`, paramIdx)
		args = append(args, orgID)
	} else {
		query += fmt.Sprintf(` AND f.user_id = $%d`, paramIdx)
		args = append(args, userID)
	}

	query += ` ORDER BY f.position, f.created_at`

	rows, err := h.db.Query(r.Context(), query, args...)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list folders")
		return
	}
	defer rows.Close()

	items := make([]folderItem, 0)
	for rows.Next() {
		var item folderItem
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Name, &item.Position, &createdAt, &item.VideoCount); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to scan folder")
			return
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		items = append(items, item)
	}

	httputil.WriteJSON(w, http.StatusOK, items)
}

type createFolderRequest struct {
	Name string `json:"name"`
}

func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req createFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "folder name is required")
		return
	}
	if msg := validate.FolderName(name); msg != "" {
		httputil.WriteError(w, http.StatusBadRequest, msg)
		return
	}

	orgID := auth.OrgIDFromContext(r.Context())

	var countQuery string
	var countArgs []any
	if orgID != "" {
		countQuery = `SELECT COUNT(*) FROM folders WHERE organization_id = $1`
		countArgs = []any{orgID}
	} else {
		countQuery = `SELECT COUNT(*) FROM folders WHERE user_id = $1`
		countArgs = []any{userID}
	}

	var count int
	if err := h.db.QueryRow(r.Context(), countQuery, countArgs...).Scan(&count); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to check folder limit")
		return
	}
	if count >= maxFoldersPerUser {
		httputil.WriteError(w, http.StatusForbidden, "maximum of 50 folders reached")
		return
	}

	var orgIDArg *string
	if orgID != "" {
		orgIDArg = &orgID
	}

	var item folderItem
	var createdAt time.Time
	var insertQuery string
	var insertArgs []any
	if orgID != "" {
		insertQuery = `INSERT INTO folders (user_id, organization_id, name, position)
		 VALUES ($1, $2, $3, (SELECT COALESCE(MAX(position), -1) + 1 FROM folders WHERE organization_id = $2))
		 RETURNING id, position, created_at`
		insertArgs = []any{userID, orgIDArg, name}
	} else {
		insertQuery = `INSERT INTO folders (user_id, organization_id, name, position)
		 VALUES ($1, $2, $3, (SELECT COALESCE(MAX(position), -1) + 1 FROM folders WHERE user_id = $1))
		 RETURNING id, position, created_at`
		insertArgs = []any{userID, orgIDArg, name}
	}

	err := h.db.QueryRow(r.Context(), insertQuery, insertArgs...).Scan(&item.ID, &item.Position, &createdAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httputil.WriteError(w, http.StatusConflict, "a folder with this name already exists")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create folder")
		return
	}

	item.Name = name
	item.CreatedAt = createdAt.Format(time.RFC3339)

	httputil.WriteJSON(w, http.StatusCreated, item)
}

type updateFolderRequest struct {
	Name     *string `json:"name"`
	Position *int    `json:"position"`
}

func (h *Handler) UpdateFolder(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	folderID := chi.URLParam(r, "id")

	var req updateFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == nil && req.Position == nil {
		httputil.WriteError(w, http.StatusBadRequest, "nothing to update")
		return
	}

	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			httputil.WriteError(w, http.StatusBadRequest, "folder name is required")
			return
		}
		if msg := validate.FolderName(trimmed); msg != "" {
			httputil.WriteError(w, http.StatusBadRequest, msg)
			return
		}
		req.Name = &trimmed
	}

	setClauses := []string{}
	args := []any{}
	paramIdx := 1

	if req.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", paramIdx))
		args = append(args, *req.Name)
		paramIdx++
	}
	if req.Position != nil {
		setClauses = append(setClauses, fmt.Sprintf("position = $%d", paramIdx))
		args = append(args, *req.Position)
		paramIdx++
	}

	orgID := auth.OrgIDFromContext(r.Context())
	var query string
	if orgID != "" {
		query = fmt.Sprintf("UPDATE folders SET %s WHERE id = $%d AND organization_id = $%d",
			strings.Join(setClauses, ", "), paramIdx, paramIdx+1)
		args = append(args, folderID, orgID)
	} else {
		query = fmt.Sprintf("UPDATE folders SET %s WHERE id = $%d AND user_id = $%d",
			strings.Join(setClauses, ", "), paramIdx, paramIdx+1)
		args = append(args, folderID, userID)
	}

	tag, err := h.db.Exec(r.Context(), query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httputil.WriteError(w, http.StatusConflict, "a folder with this name already exists")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update folder")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "folder not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	folderID := chi.URLParam(r, "id")

	orgID := auth.OrgIDFromContext(r.Context())
	var deleteQuery string
	var deleteArgs []any
	if orgID != "" {
		deleteQuery = `DELETE FROM folders WHERE id = $1 AND organization_id = $2`
		deleteArgs = []any{folderID, orgID}
	} else {
		deleteQuery = `DELETE FROM folders WHERE id = $1 AND user_id = $2`
		deleteArgs = []any{folderID, userID}
	}

	tag, err := h.db.Exec(r.Context(), deleteQuery, deleteArgs...)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to delete folder")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "folder not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type setVideoFolderRequest struct {
	FolderID *string `json:"folderId"`
}

func (h *Handler) SetVideoFolder(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req setVideoFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	orgID := auth.OrgIDFromContext(r.Context())

	var folderID *string
	if req.FolderID != nil && *req.FolderID != "" {
		var folderVerifyQuery string
		var folderVerifyArgs []any
		if orgID != "" {
			folderVerifyQuery = `SELECT id FROM folders WHERE id = $1 AND organization_id = $2`
			folderVerifyArgs = []any{*req.FolderID, orgID}
		} else {
			folderVerifyQuery = `SELECT id FROM folders WHERE id = $1 AND user_id = $2`
			folderVerifyArgs = []any{*req.FolderID, userID}
		}

		var id string
		err := h.db.QueryRow(r.Context(), folderVerifyQuery, folderVerifyArgs...).Scan(&id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				httputil.WriteError(w, http.StatusNotFound, "folder not found")
				return
			}
			httputil.WriteError(w, http.StatusInternalServerError, "failed to verify folder")
			return
		}
		folderID = req.FolderID
	}

	var updateQuery string
	var updateArgs []any
	if orgID != "" {
		updateQuery = `UPDATE videos SET folder_id = $1 WHERE id = $2 AND organization_id = $3 AND status != 'deleted'`
		updateArgs = []any{folderID, videoID, orgID}
	} else {
		updateQuery = `UPDATE videos SET folder_id = $1 WHERE id = $2 AND user_id = $3 AND status != 'deleted'`
		updateArgs = []any{folderID, videoID, userID}
	}

	result, err := h.db.Exec(r.Context(), updateQuery, updateArgs...)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update video folder")
		return
	}
	if result.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

