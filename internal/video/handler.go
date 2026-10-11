package video

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/email"
	"github.com/sendrec/sendrec/internal/plans"
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

type GeoResolver interface {
	Lookup(ip string) (country, city string)
}

type Handler struct {
	db                      database.DBTX
	storage                 ObjectStorage
	baseURL                 string
	maxUploadBytes          int64
	maxVideosPerMonth       int
	maxVideoDurationSeconds int
	maxPlaylists            int
	maxOrgsOwned            int
	hmacSecret              string
	secureCookies           bool
	commentNotifier         CommentNotifier
	viewNotifier            ViewNotifier
	slackNotifier           SlackNotifier
	brandingEnabled         bool
	analyticsScript         string
	billingEnabled          bool
	aiEnabled               bool
	transcriptionEnabled    bool
	noiseReductionFilter    string
	webhookClient           *webhook.Client
	geoResolver             GeoResolver
	subscriptionCanceler    SubscriptionCanceler
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
		maxOrgsOwned:            plans.Free.MaxOrgsOwned,
		hmacSecret:              hmacSecret,
		secureCookies:           secureCookies,
	}
}

// SetMaxOrgsOwned is the workspace cap reported to the dashboard. It must match
// the one organization.Create enforces; see organization.Handler.SetMaxOrgsOwned.
func (h *Handler) SetMaxOrgsOwned(n int) {
	h.maxOrgsOwned = n
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

// SetBillingEnabled marks the hosted service, where free viewers can be
// invited to sign up. Self-hosted installs leave it off.
func (h *Handler) SetBillingEnabled(enabled bool) {
	h.billingEnabled = enabled
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

func (h *Handler) SetNoiseReductionFilter(filter string) {
	h.noiseReductionFilter = filter
}

func (h *Handler) SetWebhookClient(c *webhook.Client) {
	h.webhookClient = c
}

func (h *Handler) SetGeoResolver(r GeoResolver) {
	h.geoResolver = r
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

// replacementFileKey names a new object next to key for a rewritten copy of
// the video. A rewrite never goes over the object the row points at: until the
// row is switched, that object is the only copy of the video. Any earlier
// rewrite suffix is dropped so repeated edits don't grow the key.
func replacementFileKey(key, ext string) string {
	dir, name := path.Split(key)
	stem, _, _ := strings.Cut(strings.TrimSuffix(name, path.Ext(name)), ".")
	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	return dir + stem + "." + hex.EncodeToString(suffix) + ext
}

func webcamFileKey(userID, shareToken, contentType string) string {
	ext := extensionForContentType(contentType)
	return fmt.Sprintf("recordings/%s/%s_webcam%s", userID, shareToken, ext)
}
