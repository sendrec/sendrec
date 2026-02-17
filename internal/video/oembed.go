package video

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/httputil"
)

type oEmbedResponse struct {
	Type         string `json:"type"`
	Version      string `json:"version"`
	Title        string `json:"title"`
	Duration     int    `json:"duration"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	AuthorName   string `json:"authorName"`
	WatchURL     string `json:"watchUrl"`
	HTML         string `json:"html"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	CreatedAt    string `json:"createdAt"`
}

func (h *Handler) OEmbed(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var title string
	var duration int
	var authorName string
	var createdAt time.Time
	var shareExpiresAt *time.Time
	var thumbnailKey *string

	err := h.db.QueryRow(r.Context(),
		`SELECT v.title, v.duration, u.name, v.created_at, v.share_expires_at, v.thumbnail_key
		 FROM videos v
		 JOIN users u ON u.id = v.user_id
		 WHERE v.share_token = $1 AND v.status IN ('ready', 'processing')`,
		shareToken,
	).Scan(&title, &duration, &authorName, &createdAt, &shareExpiresAt, &thumbnailKey)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	if shareExpiresAt != nil && time.Now().After(*shareExpiresAt) {
		httputil.WriteError(w, http.StatusGone, "link expired")
		return
	}

	var thumbnailURL string
	if thumbnailKey != nil {
		if u, err := h.storage.GenerateDownloadURL(r.Context(), *thumbnailKey, 1*time.Hour); err == nil {
			thumbnailURL = u
		}
	}

	embedURL := h.baseURL + "/embed/" + shareToken
	iframeHTML := `<iframe src="` + embedURL + `" width="640" height="360" frameborder="0" allowfullscreen></iframe>`

	httputil.WriteJSON(w, http.StatusOK, oEmbedResponse{
		Type:         "video",
		Version:      "1.0",
		Title:        title,
		Duration:     duration,
		ThumbnailURL: thumbnailURL,
		AuthorName:   authorName,
		WatchURL:     h.baseURL + "/watch/" + shareToken,
		HTML:         iframeHTML,
		Width:        640,
		Height:       360,
		CreatedAt:    createdAt.Format(time.RFC3339),
	})
}
