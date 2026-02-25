package video

import (
	"context"
	"fmt"
	"time"

	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/email"
	"github.com/sendrec/sendrec/internal/webhook"
)

type ObjectStorage interface {
	GenerateUploadURL(ctx context.Context, key string, contentType string, contentLength int64, expiry time.Duration) (string, error)
	GenerateDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	GenerateDownloadURLWithDisposition(ctx context.Context, key string, filename string, expiry time.Duration) (string, error)
	DeleteObject(ctx context.Context, key string) error
	HeadObject(ctx context.Context, key string) (int64, string, error)
	DownloadToFile(ctx context.Context, key string, destPath string) error
	UploadFile(ctx context.Context, key string, filePath string, contentType string) error
}

type CommentNotifier interface {
	SendCommentNotification(ctx context.Context, toEmail, toName, videoTitle, commentAuthor, commentBody, watchURL string) error
}

type ViewNotifier interface {
	SendViewNotification(ctx context.Context, toEmail, toName, videoTitle, watchURL string, viewCount int) error
	SendDigestNotification(ctx context.Context, toEmail, toName string, videos []email.DigestVideoSummary) error
}

// SlackNotifier sends Slack webhook notifications independently of the email notification mode.
type SlackNotifier interface {
	SendViewNotification(ctx context.Context, toEmail, toName, videoTitle, watchURL string, viewCount int) error
	SendCommentNotification(ctx context.Context, toEmail, toName, videoTitle, commentAuthor, commentBody, watchURL string) error
}

type Handler struct {
	db                      database.DBTX
	storage                 ObjectStorage
	baseURL                 string
	maxUploadBytes          int64
	maxVideosPerMonth       int
	maxVideoDurationSeconds int
	maxPlaylists            int
	hmacSecret              string
	secureCookies           bool
	commentNotifier         CommentNotifier
	viewNotifier            ViewNotifier
	slackNotifier           SlackNotifier
	brandingEnabled         bool
	analyticsScript         string
	aiEnabled               bool
	transcriptionEnabled    bool
	webhookClient           *webhook.Client
}

func NewHandler(db database.DBTX, s ObjectStorage, baseURL string, maxUploadBytes int64, maxVideosPerMonth int, maxVideoDurationSeconds int, maxPlaylists int, hmacSecret string, secureCookies bool) *Handler {
	return &Handler{
		db:                      db,
		storage:                 s,
		baseURL:                 baseURL,
		maxUploadBytes:          maxUploadBytes,
		maxVideosPerMonth:       maxVideosPerMonth,
		maxVideoDurationSeconds: maxVideoDurationSeconds,
		maxPlaylists:            maxPlaylists,
		hmacSecret:              hmacSecret,
		secureCookies:           secureCookies,
	}
}

func (h *Handler) SetCommentNotifier(n CommentNotifier) {
	h.commentNotifier = n
}

func (h *Handler) SetViewNotifier(n ViewNotifier) {
	h.viewNotifier = n
}

func (h *Handler) SetSlackNotifier(n SlackNotifier) {
	h.slackNotifier = n
}

func (h *Handler) SetBrandingEnabled(enabled bool) {
	h.brandingEnabled = enabled
}

func (h *Handler) SetAnalyticsScript(script string) {
	h.analyticsScript = script
}

func (h *Handler) SetAIEnabled(enabled bool) {
	h.aiEnabled = enabled
}

func (h *Handler) SetTranscriptionEnabled(enabled bool) {
	h.transcriptionEnabled = enabled
}

func (h *Handler) SetWebhookClient(c *webhook.Client) {
	h.webhookClient = c
}

func extensionForContentType(ct string) string {
	switch ct {
	case "video/mp4":
		return ".mp4"
	case "video/quicktime":
		return ".mov"
	default:
		return ".webm"
	}
}

func videoFileKey(userID, shareToken, contentType string) string {
	return fmt.Sprintf("recordings/%s/%s%s", userID, shareToken, extensionForContentType(contentType))
}

func webcamFileKey(userID, shareToken, contentType string) string {
	ext := extensionForContentType(contentType)
	return fmt.Sprintf("recordings/%s/%s_webcam%s", userID, shareToken, ext)
}
