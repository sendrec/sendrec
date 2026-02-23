package video

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/slack"
	"github.com/sendrec/sendrec/internal/webhook"
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
	WebhookUrl       *string `json:"webhookUrl"`
	WebhookSecret    *string `json:"webhookSecret"`
}

type setNotificationPreferencesRequest struct {
	NotificationMode string  `json:"notificationMode"`
	SlackWebhookUrl  *string `json:"slackWebhookUrl,omitempty"`
	WebhookUrl       *string `json:"webhookUrl,omitempty"`
}

func (h *Handler) GetNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var notificationMode string
	var slackWebhookUrl, webhookUrl, webhookSecret *string
	err := h.db.QueryRow(r.Context(),
		`SELECT view_notification, slack_webhook_url, webhook_url, webhook_secret FROM notification_preferences WHERE user_id = $1`,
		userID,
	).Scan(&notificationMode, &slackWebhookUrl, &webhookUrl, &webhookSecret)
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
		WebhookUrl:       webhookUrl,
		WebhookSecret:    webhookSecret,
	})
}

const maxSlackWebhookURLLength = 500
const maxWebhookURLLength = 500

func isValidWebhookURL(u string) bool {
	if strings.HasPrefix(u, "https://") {
		return true
	}
	if strings.HasPrefix(u, "http://localhost") || strings.HasPrefix(u, "http://127.0.0.1") {
		return true
	}
	return false
}

func generateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

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

	var webhookUrl *string
	var webhookSecret string
	if req.WebhookUrl != nil {
		trimmed := strings.TrimSpace(*req.WebhookUrl)
		if trimmed != "" {
			if !isValidWebhookURL(trimmed) {
				httputil.WriteError(w, http.StatusBadRequest, "webhook URL must use HTTPS (HTTP allowed only for localhost)")
				return
			}
			if len(trimmed) > maxWebhookURLLength {
				httputil.WriteError(w, http.StatusBadRequest, "webhook URL must be 500 characters or fewer")
				return
			}
			webhookUrl = &trimmed
			secret, err := generateWebhookSecret()
			if err != nil {
				httputil.WriteError(w, http.StatusInternalServerError, "failed to generate webhook secret")
				return
			}
			webhookSecret = secret
		}
	}

	if _, err := h.db.Exec(r.Context(),
		`INSERT INTO notification_preferences (user_id, view_notification, slack_webhook_url, webhook_url, webhook_secret)
		 VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5, ''), NULL))
		 ON CONFLICT (user_id) DO UPDATE SET view_notification = $2, slack_webhook_url = $3, webhook_url = $4,
		 webhook_secret = COALESCE(notification_preferences.webhook_secret, NULLIF($5, '')), updated_at = now()`,
		userID, req.NotificationMode, slackWebhookUrl, webhookUrl, webhookSecret,
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

func (h *Handler) RegenerateWebhookSecret(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	secret, err := generateWebhookSecret()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate secret")
		return
	}
	tag, err := h.db.Exec(r.Context(),
		`UPDATE notification_preferences SET webhook_secret = $1, updated_at = now() WHERE user_id = $2 AND webhook_url IS NOT NULL`,
		secret, userID,
	)
	if err != nil || tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusBadRequest, "no webhook URL configured")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"webhookSecret": secret})
}

func (h *Handler) TestWebhook(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	var webhookURL, webhookSecret *string
	err := h.db.QueryRow(r.Context(),
		`SELECT webhook_url, webhook_secret FROM notification_preferences WHERE user_id = $1`,
		userID,
	).Scan(&webhookURL, &webhookSecret)
	if err != nil || webhookURL == nil || webhookSecret == nil {
		httputil.WriteError(w, http.StatusBadRequest, "no webhook URL configured")
		return
	}
	event := webhook.Event{
		Name:      "webhook.test",
		Timestamp: time.Now().UTC(),
		Data:      map[string]any{"message": "Webhook is working!"},
	}
	if err := h.webhookClient.Dispatch(r.Context(), userID, *webhookURL, *webhookSecret, event); err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "failed to deliver test webhook")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type deliveryResponse struct {
	ID           string          `json:"id"`
	Event        string          `json:"event"`
	Payload      json.RawMessage `json:"payload"`
	StatusCode   *int            `json:"statusCode"`
	ResponseBody string          `json:"responseBody"`
	Attempt      int             `json:"attempt"`
	CreatedAt    string          `json:"createdAt"`
}

func (h *Handler) ListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	rows, err := h.db.Query(r.Context(),
		`SELECT id, event, payload, status_code, response_body, attempt, created_at
		 FROM webhook_deliveries WHERE user_id = $1 ORDER BY created_at DESC LIMIT 50`,
		userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to fetch deliveries")
		return
	}
	defer rows.Close()

	var deliveries []deliveryResponse
	for rows.Next() {
		var d deliveryResponse
		var createdAt time.Time
		if err := rows.Scan(&d.ID, &d.Event, &d.Payload, &d.StatusCode, &d.ResponseBody, &d.Attempt, &createdAt); err != nil {
			continue
		}
		d.CreatedAt = createdAt.Format(time.RFC3339)
		deliveries = append(deliveries, d)
	}
	if deliveries == nil {
		deliveries = []deliveryResponse{}
	}
	httputil.WriteJSON(w, http.StatusOK, deliveries)
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
	if viewerUserID != "" && viewerUserID == ownerID {
		return
	}

	watchURL := h.baseURL + "/watch/" + shareToken

	// Slack: always send (Slack client gates on webhook URL presence in DB)
	if h.slackNotifier != nil {
		if err := h.slackNotifier.SendViewNotification(ctx, ownerEmail, ownerName, videoTitle, watchURL, 1); err != nil {
			slog.Error("notification: failed to send Slack view notification", "video_id", videoID, "error", err)
		}
	}

	// Webhook: fire-and-forget for video.viewed
	h.dispatchWebhook(ownerID, webhook.Event{
		Name:      "video.viewed",
		Timestamp: time.Now().UTC(),
		Data: map[string]any{
			"videoId":  videoID,
			"title":    videoTitle,
			"watchUrl": watchURL,
		},
	})

	// Email: gated on notification mode
	if h.viewNotifier == nil {
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

	if err := h.viewNotifier.SendViewNotification(ctx, ownerEmail, ownerName, videoTitle, watchURL, 1); err != nil {
		slog.Error("notification: failed to send view notification", "video_id", videoID, "error", err)
	}
}
