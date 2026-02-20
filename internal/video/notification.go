package video

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/slack"
)

const (
	notificationModeOff              = "off"
	notificationModeViewsOnly        = "views_only"
	notificationModeCommentsOnly     = "comments_only"
	notificationModeViewsAndComments = "views_and_comments"
	notificationModeDigest           = "digest"
)

const (
	viewNotificationOff    = "off"
	viewNotificationEvery  = "every"
	viewNotificationDigest = "digest"
)

var validNotificationModes = map[string]bool{
	notificationModeOff:              true,
	notificationModeViewsOnly:        true,
	notificationModeCommentsOnly:     true,
	notificationModeViewsAndComments: true,
	notificationModeDigest:           true,
}

var validViewNotificationModes = map[string]bool{
	viewNotificationOff:    true,
	viewNotificationEvery:  true,
	viewNotificationDigest: true,
}

type notificationPreferencesResponse struct {
	NotificationMode string  `json:"notificationMode"`
	SlackWebhookUrl  *string `json:"slackWebhookUrl"`
}

type setNotificationPreferencesRequest struct {
	NotificationMode string  `json:"notificationMode"`
	SlackWebhookUrl  *string `json:"slackWebhookUrl,omitempty"`
}

func (h *Handler) GetNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var notificationMode string
	var slackWebhookUrl *string
	err := h.db.QueryRow(r.Context(),
		`SELECT view_notification, slack_webhook_url FROM notification_preferences WHERE user_id = $1`,
		userID,
	).Scan(&notificationMode, &slackWebhookUrl)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notificationMode = notificationModeOff
		} else {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to get preferences")
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, notificationPreferencesResponse{
		NotificationMode: normalizeAccountNotificationMode(notificationMode),
		SlackWebhookUrl:  slackWebhookUrl,
	})
}

const maxSlackWebhookURLLength = 500

func (h *Handler) PutNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req setNotificationPreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !validNotificationModes[req.NotificationMode] {
		httputil.WriteError(w, http.StatusBadRequest, "invalid notification mode")
		return
	}

	var slackWebhookUrl *string
	if req.SlackWebhookUrl != nil {
		trimmed := strings.TrimSpace(*req.SlackWebhookUrl)
		if trimmed != "" {
			if !strings.HasPrefix(trimmed, "https://hooks.slack.com/") {
				httputil.WriteError(w, http.StatusBadRequest, "Slack webhook URL must start with https://hooks.slack.com/")
				return
			}
			if len(trimmed) > maxSlackWebhookURLLength {
				httputil.WriteError(w, http.StatusBadRequest, "Slack webhook URL must be 500 characters or fewer")
				return
			}
			slackWebhookUrl = &trimmed
		}
	}

	if _, err := h.db.Exec(r.Context(),
		`INSERT INTO notification_preferences (user_id, view_notification, slack_webhook_url)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE SET view_notification = $2, slack_webhook_url = $3, updated_at = now()`,
		userID, req.NotificationMode, slackWebhookUrl,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to save preferences")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TestSlackWebhook(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var webhookURL *string
	err := h.db.QueryRow(r.Context(),
		`SELECT slack_webhook_url FROM notification_preferences WHERE user_id = $1`,
		userID,
	).Scan(&webhookURL)
	if err != nil || webhookURL == nil {
		httputil.WriteError(w, http.StatusBadRequest, "no Slack webhook URL configured")
		return
	}

	if err := slack.SendTestMessage(r.Context(), *webhookURL); err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "failed to send test message to Slack")
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

func normalizeAccountNotificationMode(mode string) string {
	switch mode {
	case notificationModeOff,
		notificationModeViewsOnly,
		notificationModeCommentsOnly,
		notificationModeViewsAndComments,
		notificationModeDigest:
		return mode
	case viewNotificationEvery, "first":
		// Legacy view-only modes from older versions.
		return notificationModeViewsOnly
	default:
		return notificationModeOff
	}
}

func normalizeViewNotificationMode(mode string) string {
	switch mode {
	case viewNotificationOff, viewNotificationEvery, viewNotificationDigest:
		return mode
	case "first":
		// Legacy per-video mode from older versions.
		return viewNotificationEvery
	default:
		return viewNotificationOff
	}
}

func sendsImmediateViewNotification(accountMode string) bool {
	return accountMode == notificationModeViewsOnly || accountMode == notificationModeViewsAndComments
}

func sendsImmediateCommentNotification(accountMode string) bool {
	return accountMode == notificationModeCommentsOnly || accountMode == notificationModeViewsAndComments
}

func (h *Handler) accountNotificationMode(ctx context.Context, ownerID string) string {
	if h.db == nil {
		return notificationModeOff
	}
	var mode string
	err := h.db.QueryRow(ctx,
		`SELECT view_notification FROM notification_preferences WHERE user_id = $1`,
		ownerID,
	).Scan(&mode)
	if err != nil {
		return notificationModeOff
	}
	return normalizeAccountNotificationMode(mode)
}

func (h *Handler) shouldSendImmediateCommentNotification(ctx context.Context, ownerID string) bool {
	return sendsImmediateCommentNotification(h.accountNotificationMode(ctx, ownerID))
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

func (h *Handler) resolveAndNotify(ctx context.Context, videoID, ownerID, ownerEmail, ownerName, videoTitle, shareToken, viewerUserID string, videoViewNotification *string) {
	if h.viewNotifier == nil {
		return
	}

	if viewerUserID != "" && viewerUserID == ownerID {
		return
	}

	mode := viewNotificationOff
	if videoViewNotification != nil {
		mode = normalizeViewNotificationMode(*videoViewNotification)
	} else {
		if sendsImmediateViewNotification(h.accountNotificationMode(ctx, ownerID)) {
			mode = viewNotificationEvery
		}
	}

	if mode != viewNotificationEvery {
		return
	}

	watchURL := h.baseURL + "/watch/" + shareToken
	if err := h.viewNotifier.SendViewNotification(ctx, ownerEmail, ownerName, videoTitle, watchURL, 1); err != nil {
		log.Printf("failed to send view notification for %s: %v", videoID, err)
	}
}
