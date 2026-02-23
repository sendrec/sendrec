package video

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
)

const maxPlaylistsFreeTier = 3
const maxPlaylistTitleLength = 200
const maxPlaylistDescriptionLength = 2000

type playlistItem struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	IsShared    bool    `json:"isShared"`
	ShareToken  *string `json:"shareToken,omitempty"`
	ShareURL    *string `json:"shareUrl,omitempty"`
	Position    int     `json:"position"`
	VideoCount  int64   `json:"videoCount"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

type playlistVideo struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Duration     int     `json:"duration"`
	ShareToken   string  `json:"shareToken"`
	ShareURL     string  `json:"shareUrl"`
	Status       string  `json:"status"`
	Position     int     `json:"position"`
	ThumbnailURL *string `json:"thumbnailUrl"`
	CreatedAt    string  `json:"createdAt"`
}

type playlistDetail struct {
	playlistItem
	RequireEmail bool            `json:"requireEmail"`
	Videos       []playlistVideo `json:"videos"`
}

type createPlaylistRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
}

func (h *Handler) CreatePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req createPlaylistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		httputil.WriteError(w, http.StatusBadRequest, "playlist title is required")
		return
	}
	if len(title) > maxPlaylistTitleLength {
		httputil.WriteError(w, http.StatusBadRequest, "playlist title must be 200 characters or less")
		return
	}

	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		if len(trimmed) > maxPlaylistDescriptionLength {
			httputil.WriteError(w, http.StatusBadRequest, "playlist description must be 2000 characters or less")
			return
		}
		if trimmed == "" {
			req.Description = nil
		} else {
			req.Description = &trimmed
		}
	}

	var count int
	if err := h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM playlists WHERE user_id = $1`,
		userID,
	).Scan(&count); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to check playlist limit")
		return
	}

	plan, _ := h.getUserPlan(r.Context(), userID)
	if plan == "free" && count >= maxPlaylistsFreeTier {
		httputil.WriteError(w, http.StatusForbidden, "free plan limited to 3 playlists, upgrade to create more")
		return
	}

	var item playlistItem
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(r.Context(),
		`INSERT INTO playlists (user_id, title, description, position)
		 VALUES ($1, $2, $3, (SELECT COALESCE(MAX(position), -1) + 1 FROM playlists WHERE user_id = $1))
		 RETURNING id, position, created_at, updated_at`,
		userID, title, req.Description,
	).Scan(&item.ID, &item.Position, &createdAt, &updatedAt)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create playlist")
		return
	}

	item.Title = title
	item.Description = req.Description
	item.CreatedAt = createdAt.Format(time.RFC3339)
	item.UpdatedAt = updatedAt.Format(time.RFC3339)

	httputil.WriteJSON(w, http.StatusCreated, item)
}

func (h *Handler) ListPlaylists(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	rows, err := h.db.Query(r.Context(),
		`SELECT p.id, p.title, p.description, p.is_shared, p.share_token, p.position, p.created_at, p.updated_at,
		        (SELECT COUNT(*) FROM playlist_videos pv WHERE pv.playlist_id = p.id) AS video_count
		 FROM playlists p
		 WHERE p.user_id = $1
		 ORDER BY p.position, p.created_at`,
		userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list playlists")
		return
	}
	defer rows.Close()

	items := make([]playlistItem, 0)
	for rows.Next() {
		var item playlistItem
		var createdAt, updatedAt time.Time
		var shareToken *string
		if err := rows.Scan(&item.ID, &item.Title, &item.Description, &item.IsShared, &shareToken, &item.Position, &createdAt, &updatedAt, &item.VideoCount); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to scan playlist")
			return
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		item.UpdatedAt = updatedAt.Format(time.RFC3339)
		item.ShareToken = shareToken
		if shareToken != nil {
			shareURL := h.baseURL + "/watch/playlist/" + *shareToken
			item.ShareURL = &shareURL
		}
		items = append(items, item)
	}

	httputil.WriteJSON(w, http.StatusOK, items)
}

func (h *Handler) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	playlistID := chi.URLParam(r, "id")

	var detail playlistDetail
	var createdAt, updatedAt time.Time
	var shareToken *string
	err := h.db.QueryRow(r.Context(),
		`SELECT p.id, p.title, p.description, p.is_shared, p.share_token, p.require_email, p.position, p.created_at, p.updated_at
		 FROM playlists p
		 WHERE p.id = $1 AND p.user_id = $2`,
		playlistID, userID,
	).Scan(&detail.ID, &detail.Title, &detail.Description, &detail.IsShared, &shareToken, &detail.RequireEmail, &detail.Position, &createdAt, &updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			httputil.WriteError(w, http.StatusNotFound, "playlist not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get playlist")
		return
	}

	detail.CreatedAt = createdAt.Format(time.RFC3339)
	detail.UpdatedAt = updatedAt.Format(time.RFC3339)
	detail.ShareToken = shareToken
	if shareToken != nil {
		shareURL := h.baseURL + "/watch/playlist/" + *shareToken
		detail.ShareURL = &shareURL
	}

	rows, err := h.db.Query(r.Context(),
		`SELECT v.id, v.title, v.duration, v.share_token, v.status, v.created_at,
		        v.thumbnail_key, pv.position
		 FROM playlist_videos pv
		 JOIN videos v ON v.id = pv.video_id AND v.status != 'deleted'
		 WHERE pv.playlist_id = $1
		 ORDER BY pv.position, v.created_at`,
		playlistID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list playlist videos")
		return
	}
	defer rows.Close()

	detail.Videos = make([]playlistVideo, 0)
	for rows.Next() {
		var v playlistVideo
		var videoCreatedAt time.Time
		var thumbnailKey *string
		if err := rows.Scan(&v.ID, &v.Title, &v.Duration, &v.ShareToken, &v.Status, &videoCreatedAt, &thumbnailKey, &v.Position); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to scan playlist video")
			return
		}
		v.CreatedAt = videoCreatedAt.Format(time.RFC3339)
		v.ShareURL = h.baseURL + "/watch/" + v.ShareToken
		if thumbnailKey != nil {
			thumbURL, err := h.storage.GenerateDownloadURL(r.Context(), *thumbnailKey, 1*time.Hour)
			if err == nil {
				v.ThumbnailURL = &thumbURL
			}
		}
		detail.Videos = append(detail.Videos, v)
	}

	// Count videos for the response
	detail.VideoCount = int64(len(detail.Videos))

	httputil.WriteJSON(w, http.StatusOK, detail)
}

type updatePlaylistRequest struct {
	Title         *string `json:"title"`
	Description   *string `json:"description"`
	Position      *int    `json:"position"`
	IsShared      *bool   `json:"isShared"`
	SharePassword *string `json:"sharePassword"`
	RequireEmail  *bool   `json:"requireEmail"`
}

func (h *Handler) UpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	playlistID := chi.URLParam(r, "id")

	var req updatePlaylistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Title == nil && req.Description == nil && req.Position == nil && req.IsShared == nil && req.SharePassword == nil && req.RequireEmail == nil {
		httputil.WriteError(w, http.StatusBadRequest, "nothing to update")
		return
	}

	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		if trimmed == "" {
			httputil.WriteError(w, http.StatusBadRequest, "playlist title is required")
			return
		}
		if len(trimmed) > maxPlaylistTitleLength {
			httputil.WriteError(w, http.StatusBadRequest, "playlist title must be 200 characters or less")
			return
		}
		req.Title = &trimmed
	}

	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		if len(trimmed) > maxPlaylistDescriptionLength {
			httputil.WriteError(w, http.StatusBadRequest, "playlist description must be 2000 characters or less")
			return
		}
	}

	setClauses := []string{}
	args := []any{}
	paramIdx := 1

	if req.Title != nil {
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", paramIdx))
		args = append(args, *req.Title)
		paramIdx++
	}
	if req.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", paramIdx))
		args = append(args, *req.Description)
		paramIdx++
	}
	if req.Position != nil {
		setClauses = append(setClauses, fmt.Sprintf("position = $%d", paramIdx))
		args = append(args, *req.Position)
		paramIdx++
	}
	if req.IsShared != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_shared = $%d", paramIdx))
		args = append(args, *req.IsShared)
		paramIdx++

		if *req.IsShared {
			token, err := generateShareToken()
			if err != nil {
				httputil.WriteError(w, http.StatusInternalServerError, "failed to generate share token")
				return
			}
			setClauses = append(setClauses, fmt.Sprintf("share_token = $%d", paramIdx))
			args = append(args, token)
			paramIdx++
		}
	}
	if req.SharePassword != nil {
		hash, err := hashSharePassword(*req.SharePassword)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}
		setClauses = append(setClauses, fmt.Sprintf("share_password = $%d", paramIdx))
		args = append(args, hash)
		paramIdx++
	}
	if req.RequireEmail != nil {
		setClauses = append(setClauses, fmt.Sprintf("require_email = $%d", paramIdx))
		args = append(args, *req.RequireEmail)
		paramIdx++
	}

	query := fmt.Sprintf("UPDATE playlists SET %s WHERE id = $%d AND user_id = $%d",
		strings.Join(setClauses, ", "), paramIdx, paramIdx+1)
	args = append(args, playlistID, userID)

	tag, err := h.db.Exec(r.Context(), query, args...)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update playlist")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "playlist not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeletePlaylist(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	playlistID := chi.URLParam(r, "id")

	tag, err := h.db.Exec(r.Context(),
		`DELETE FROM playlists WHERE id = $1 AND user_id = $2`,
		playlistID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to delete playlist")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "playlist not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type addPlaylistVideosRequest struct {
	VideoIDs []string `json:"videoIds"`
}

type reorderItem struct {
	VideoID  string `json:"videoId"`
	Position int    `json:"position"`
}

type reorderPlaylistVideosRequest struct {
	Items []reorderItem `json:"items"`
}

func (h *Handler) AddPlaylistVideos(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	playlistID := chi.URLParam(r, "id")

	var req addPlaylistVideosRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.VideoIDs) == 0 {
		httputil.WriteError(w, http.StatusBadRequest, "at least 1 video ID is required")
		return
	}

	var exists bool
	if err := h.db.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM playlists WHERE id = $1 AND user_id = $2)`,
		playlistID, userID,
	).Scan(&exists); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to verify playlist ownership")
		return
	}
	if !exists {
		httputil.WriteError(w, http.StatusNotFound, "playlist not found")
		return
	}

	for _, videoID := range req.VideoIDs {
		var videoExists bool
		if err := h.db.QueryRow(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM videos WHERE id = $1 AND user_id = $2 AND status != 'deleted')`,
			videoID, userID,
		).Scan(&videoExists); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to verify video ownership")
			return
		}
		if !videoExists {
			httputil.WriteError(w, http.StatusNotFound, fmt.Sprintf("video %s not found", videoID))
			return
		}

		if _, err := h.db.Exec(r.Context(),
			`INSERT INTO playlist_videos (playlist_id, video_id, position)
			 VALUES ($1, $2, (SELECT COALESCE(MAX(position), -1) + 1 FROM playlist_videos WHERE playlist_id = $1))
			 ON CONFLICT (playlist_id, video_id) DO NOTHING`,
			playlistID, videoID,
		); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to add video to playlist")
			return
		}
	}

	if _, err := h.db.Exec(r.Context(),
		`UPDATE playlists SET updated_at = now() WHERE id = $1`,
		playlistID,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update playlist")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemovePlaylistVideo(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	playlistID := chi.URLParam(r, "id")
	videoID := chi.URLParam(r, "videoId")

	tag, err := h.db.Exec(r.Context(),
		`DELETE FROM playlist_videos WHERE playlist_id = $1 AND video_id = $2
		 AND playlist_id IN (SELECT id FROM playlists WHERE id = $1 AND user_id = $3)`,
		playlistID, videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to remove video from playlist")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not in playlist")
		return
	}

	if _, err := h.db.Exec(r.Context(),
		`UPDATE playlists SET updated_at = now() WHERE id = $1`,
		playlistID,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update playlist")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ReorderPlaylistVideos(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	playlistID := chi.URLParam(r, "id")

	var req reorderPlaylistVideosRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var exists bool
	if err := h.db.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM playlists WHERE id = $1 AND user_id = $2)`,
		playlistID, userID,
	).Scan(&exists); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to verify playlist ownership")
		return
	}
	if !exists {
		httputil.WriteError(w, http.StatusNotFound, "playlist not found")
		return
	}

	for _, item := range req.Items {
		if _, err := h.db.Exec(r.Context(),
			`UPDATE playlist_videos SET position = $1 WHERE playlist_id = $2 AND video_id = $3`,
			item.Position, playlistID, item.VideoID,
		); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to reorder playlist videos")
			return
		}
	}

	if _, err := h.db.Exec(r.Context(),
		`UPDATE playlists SET updated_at = now() WHERE id = $1`,
		playlistID,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to update playlist")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
