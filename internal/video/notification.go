package video

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
)

var validViewNotificationModes = map[string]bool{
	"off":    true,
	"every":  true,
	"first":  true,
	"digest": true,
}

type notificationPreferencesResponse struct {
	ViewNotification string `json:"viewNotification"`
}

type setNotificationPreferencesRequest struct {
	ViewNotification string `json:"viewNotification"`
}

func (h *Handler) GetNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var viewNotification string
	err := h.db.QueryRow(r.Context(),
		`SELECT view_notification FROM notification_preferences WHERE user_id = $1`,
		userID,
	).Scan(&viewNotification)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			viewNotification = "off"
		} else {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to get preferences")
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, notificationPreferencesResponse{
		ViewNotification: viewNotification,
	})
}

func (h *Handler) PutNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req setNotificationPreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !validViewNotificationModes[req.ViewNotification] {
		httputil.WriteError(w, http.StatusBadRequest, "invalid notification mode")
		return
	}

	if _, err := h.db.Exec(r.Context(),
		`INSERT INTO notification_preferences (user_id, view_notification)
		 VALUES ($1, $2)
		 ON CONFLICT (user_id) DO UPDATE SET view_notification = $2, updated_at = now()`,
		userID, req.ViewNotification,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to save preferences")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type setVideoNotificationRequest struct {
	ViewNotification *string `json:"viewNotification"`
}

func (h *Handler) SetVideoNotification(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req setVideoNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ViewNotification != nil && !validViewNotificationModes[*req.ViewNotification] {
		httputil.WriteError(w, http.StatusBadRequest, "invalid notification mode")
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET view_notification = $1 WHERE id = $2 AND user_id = $3 AND status != 'deleted'`,
		req.ViewNotification, videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "could not update notification setting")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) viewerUserIDFromRequest(r *http.Request) string {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		return ""
	}
	claims, err := auth.ValidateToken(h.hmacSecret, cookie.Value)
	if err != nil || claims.TokenType != "refresh" {
		return ""
	}
	return claims.UserID
}

func (h *Handler) resolveAndNotify(videoID, ownerID, ownerEmail, ownerName, videoTitle, shareToken, viewerUserID string, videoViewNotification *string) {
	if h.viewNotifier == nil {
		return
	}

	if viewerUserID != "" && viewerUserID == ownerID {
		return
	}

	mode := "off"
	if videoViewNotification != nil {
		mode = *videoViewNotification
	} else {
		var accountMode string
		err := h.db.QueryRow(context.Background(),
			`SELECT view_notification FROM notification_preferences WHERE user_id = $1`,
			ownerID,
		).Scan(&accountMode)
		if err == nil {
			mode = accountMode
		}
	}

	if mode == "off" || mode == "digest" {
		return
	}

	if mode == "first" {
		// Note: small race window — two simultaneous viewers could both see viewCount==1.
		// Acceptable at current scale; use advisory lock or sent-flag column if needed later.
		var viewCount int64
		err := h.db.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM video_views WHERE video_id = $1`,
			videoID,
		).Scan(&viewCount)
		if err != nil {
			log.Printf("failed to count views for first notification: %v", err)
			return
		}
		if viewCount != 1 {
			return
		}
	}

	watchURL := h.baseURL + "/watch/" + shareToken
	if err := h.viewNotifier.SendViewNotification(context.Background(), ownerEmail, ownerName, videoTitle, watchURL, 1); err != nil {
		log.Printf("failed to send view notification for %s: %v", videoID, err)
	}
}
