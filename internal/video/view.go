package video

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type viewParams struct {
	videoID          string
	ownerID          string
	ownerEmail       string
	ownerName        string
	title            string
	shareToken       string
	viewerUserID     string
	viewNotification *string
}

func (h *Handler) recordViewAsync(r *http.Request, p viewParams) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ip := clientIP(r)
		hash := viewerHash(ip, r.UserAgent())
		ref := categorizeReferrer(r.Header.Get("Referer"))
		browser := parseBrowser(r.UserAgent())
		device := parseDevice(r.UserAgent())
		var country, city string
		if h.geoResolver != nil {
			country, city = h.geoResolver.Lookup(ip)
		}
		if _, err := h.db.Exec(ctx,
			`INSERT INTO video_views (video_id, viewer_hash, referrer, browser, device, country, city)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			p.videoID, hash, ref, browser, device, country, city,
		); err != nil {
			slog.Error("failed to record view", "video_id", p.videoID, "error", err)
		}
		h.resolveAndNotify(ctx, p.videoID, p.ownerID, p.ownerEmail, p.ownerName, p.title, p.shareToken, p.viewerUserID, p.viewNotification)
	}()
}
