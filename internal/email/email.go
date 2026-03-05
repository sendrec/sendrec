package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BaseURL           string
	Username          string
	Password          string
	TemplateID        int
	CommentTemplateID int
	ViewTemplateID    int
	ConfirmTemplateID int
	WelcomeTemplateID        int
	OnboardingDay2TemplateID int
	OnboardingDay7TemplateID int
	OrgInviteTemplateID          int
	RetentionWarningTemplateID   int
	Allowlist                    []string
}

type Client struct {
	config Config
	http   *http.Client
}

func New(cfg Config) *Client {
	return &Client{
		config: cfg,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

type txRequest struct {
	SubscriberEmail string         `json:"subscriber_email"`
	TemplateID      int            `json:"template_id,omitempty"`
	Body            string         `json:"body,omitempty"`
	Data            map[string]any `json:"data"`
	ContentType     string         `json:"content_type"`
}

// DigestVideoSummary represents a single video in a digest email.
type DigestVideoSummary struct {
	Title        string `json:"title"`
	ViewCount    int    `json:"viewCount"`
	CommentCount int    `json:"commentCount"`
	WatchURL     string `json:"watchURL"`
}

type subscriberRequest struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ParseAllowlist parses a comma-separated string of email addresses and
// @domain entries into a slice suitable for Config.Allowlist.
func ParseAllowlist(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func (c *Client) isAllowed(recipientEmail string) bool {
	if len(c.config.Allowlist) == 0 {
		return true
	}
	lower := strings.ToLower(recipientEmail)
	for _, entry := range c.config.Allowlist {
		entryLower := strings.ToLower(entry)
		if strings.HasPrefix(entryLower, "@") {
			if strings.HasSuffix(lower, entryLower) {
				return true
			}
		} else if lower == entryLower {
			return true
		}
	}
	slog.Warn("email blocked by allowlist", "recipient", recipientEmail)
	return false
}

func (c *Client) ensureSubscriber(ctx context.Context, email, name string) {
	body := subscriberRequest{Email: email, Name: name, Status: "enabled"}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/api/subscribers", bytes.NewReader(jsonBody))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.Username, c.config.Password)
	resp, err := c.http.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

func (c *Client) sendTx(ctx context.Context, body txRequest) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal email request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/api/tx", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create email request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.Username, c.config.Password)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("listmonk returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) SendPasswordReset(ctx context.Context, toEmail, toName, resetLink string) error {
	if c.config.BaseURL == "" {
		slog.Warn("email not configured, password reset requested", "recipient", toEmail)
		return nil
	}

	if !c.isAllowed(toEmail) {
		return nil
	}

	c.ensureSubscriber(ctx, toEmail, toName)

	body := txRequest{
		SubscriberEmail: toEmail,
		TemplateID:      c.config.TemplateID,
		Data: map[string]any{
			"resetLink": resetLink,
			"name":      toName,
		},
		ContentType: "html",
	}

	return c.sendTx(ctx, body)
}

func (c *Client) SendCommentNotification(ctx context.Context, toEmail, toName, videoTitle, commentAuthor, commentBody, watchURL string) error {
	if c.config.BaseURL == "" {
		slog.Warn("email not configured, comment notification skipped", "video_title", videoTitle, "comment_author", commentAuthor)
		return nil
	}

	if !c.isAllowed(toEmail) {
		return nil
	}

	c.ensureSubscriber(ctx, toEmail, toName)

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"name":          toName,
			"videoTitle":    videoTitle,
			"commentAuthor": commentAuthor,
			"commentBody":   commentBody,
			"watchURL":      watchURL,
		},
		ContentType: "html",
	}

	if c.config.CommentTemplateID != 0 {
		tx.TemplateID = c.config.CommentTemplateID
	} else {
		tx.Body = fmt.Sprintf(
			`<p>Hi %s,</p><p><strong>%s</strong> commented on your video <strong>%s</strong>:</p><blockquote>%s</blockquote><p><a href="%s">View video</a></p>`,
			toName, commentAuthor, videoTitle, commentBody, watchURL,
		)
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendViewNotification(ctx context.Context, toEmail, toName, videoTitle, watchURL string, viewCount int) error {
	if c.config.BaseURL == "" {
		slog.Warn("email not configured, view notification skipped", "video_title", videoTitle)
		return nil
	}

	if !c.isAllowed(toEmail) {
		return nil
	}

	c.ensureSubscriber(ctx, toEmail, toName)

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"name":       toName,
			"videoTitle": videoTitle,
			"watchURL":   watchURL,
			"viewCount":  strconv.Itoa(viewCount),
			"isDigest":   "false",
		},
		ContentType: "html",
	}

	if c.config.ViewTemplateID != 0 {
		tx.TemplateID = c.config.ViewTemplateID
	} else {
		tx.Body = fmt.Sprintf(
			`<p>Hi %s,</p><p>Your video <strong>%s</strong> has been viewed %d time(s).</p><p><a href="%s">View video</a></p>`,
			toName, videoTitle, viewCount, watchURL,
		)
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendConfirmation(ctx context.Context, toEmail, toName, confirmLink string) error {
	if c.config.BaseURL == "" {
		slog.Warn("email not configured, confirmation email skipped", "recipient", toEmail)
		return nil
	}

	// Confirmation emails bypass the allowlist — they must always be sent,
	// even on staging/preview, so new users can complete registration.
	c.ensureSubscriber(ctx, toEmail, toName)

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"confirmLink": confirmLink,
			"name":        toName,
		},
		ContentType: "html",
	}

	if c.config.ConfirmTemplateID != 0 {
		tx.TemplateID = c.config.ConfirmTemplateID
	} else {
		tx.Body = fmt.Sprintf(
			`<p>Hi %s,</p><p>Please confirm your email address by clicking the link below:</p><p><a href="%s">Confirm email</a></p>`,
			toName, confirmLink,
		)
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendWelcome(ctx context.Context, toEmail, toName, dashboardURL string) error {
	if c.config.BaseURL == "" {
		slog.Warn("email not configured, welcome email skipped", "recipient", toEmail)
		return nil
	}

	// Welcome emails bypass the allowlist — they are part of the core
	// onboarding flow and must always be sent after email confirmation.
	c.ensureSubscriber(ctx, toEmail, toName)

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"name":         toName,
			"dashboardURL": dashboardURL,
		},
		ContentType: "html",
	}

	if c.config.WelcomeTemplateID != 0 {
		tx.TemplateID = c.config.WelcomeTemplateID
	} else {
		tx.Body = fmt.Sprintf(
			`<p>Hi %s,</p><p>Welcome to SendRec! Your account is ready.</p><p><a href="%s">Go to dashboard</a></p>`,
			toName, dashboardURL,
		)
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendOnboardingDay2(ctx context.Context, toEmail, toName, dashboardURL string) error {
	if c.config.BaseURL == "" {
		slog.Warn("email not configured, onboarding day 2 email skipped", "recipient", toEmail)
		return nil
	}

	// Onboarding emails bypass the allowlist.
	c.ensureSubscriber(ctx, toEmail, toName)

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"name":         toName,
			"dashboardURL": dashboardURL,
		},
		ContentType: "html",
	}

	if c.config.OnboardingDay2TemplateID != 0 {
		tx.TemplateID = c.config.OnboardingDay2TemplateID
	} else {
		tx.Body = fmt.Sprintf(
			`<p>Hi %s,</p><p>Ready to share your first video? Record and share in seconds.</p><p><a href="%s">Get started</a></p>`,
			toName, dashboardURL,
		)
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendOnboardingDay7(ctx context.Context, toEmail, toName, dashboardURL string) error {
	if c.config.BaseURL == "" {
		slog.Warn("email not configured, onboarding day 7 email skipped", "recipient", toEmail)
		return nil
	}

	// Onboarding emails bypass the allowlist.
	c.ensureSubscriber(ctx, toEmail, toName)

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"name":         toName,
			"dashboardURL": dashboardURL,
		},
		ContentType: "html",
	}

	if c.config.OnboardingDay7TemplateID != 0 {
		tx.TemplateID = c.config.OnboardingDay7TemplateID
	} else {
		tx.Body = fmt.Sprintf(
			`<p>Hi %s,</p><p>Unlock more with SendRec Pro — longer recordings, custom branding, and more.</p><p><a href="%s">Learn more</a></p>`,
			toName, dashboardURL,
		)
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendDigestNotification(ctx context.Context, toEmail, toName string, videos []DigestVideoSummary) error {
	if c.config.BaseURL == "" {
		slog.Warn("email not configured, digest notification skipped")
		return nil
	}

	if !c.isAllowed(toEmail) {
		return nil
	}

	c.ensureSubscriber(ctx, toEmail, toName)

	totalViews := 0
	totalComments := 0
	for _, v := range videos {
		totalViews += v.ViewCount
		totalComments += v.CommentCount
	}

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"name":          toName,
			"isDigest":      "true",
			"totalViews":    strconv.Itoa(totalViews),
			"totalComments": strconv.Itoa(totalComments),
			"videos":        videos,
		},
		ContentType: "html",
	}

	if c.config.ViewTemplateID != 0 {
		tx.TemplateID = c.config.ViewTemplateID
	} else {
		tx.Body = fmt.Sprintf(
			`<p>Hi %s,</p><p>Your videos received %d view(s) and %d comment(s) this week.</p>`,
			toName, totalViews, totalComments,
		)
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendOrgInvite(ctx context.Context, toEmail, orgName, inviterName, acceptLink string) error {
	if c.config.BaseURL == "" {
		slog.Warn("email not configured, org invite email skipped", "recipient", toEmail)
		return nil
	}

	// Invite emails bypass the allowlist — they must always be sent
	// so invited users can join the organization.
	c.ensureSubscriber(ctx, toEmail, "")

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"orgName":     orgName,
			"inviterName": inviterName,
			"acceptLink":  acceptLink,
		},
		ContentType: "html",
	}

	if c.config.OrgInviteTemplateID != 0 {
		tx.TemplateID = c.config.OrgInviteTemplateID
	} else {
		tx.Body = fmt.Sprintf(
			`<p>Hi,</p><p><strong>%s</strong> has invited you to join <strong>%s</strong> on SendRec.</p><p><a href="%s">Accept invitation</a></p>`,
			inviterName, orgName, acceptLink,
		)
	}

	return c.sendTx(ctx, tx)
}

// RetentionVideoSummary represents a single video in a retention warning email.
type RetentionVideoSummary struct {
	Title    string `json:"title"`
	WatchURL string `json:"watchURL"`
}

func (c *Client) SendRetentionWarning(ctx context.Context, toEmail string, videos []RetentionVideoSummary, expiryDate string) error {
	if c.config.BaseURL == "" {
		slog.Warn("email not configured, retention warning email skipped", "recipient", toEmail)
		return nil
	}

	// Retention warning emails bypass the allowlist — they are critical
	// notifications that must always be sent before video deletion.
	c.ensureSubscriber(ctx, toEmail, "")

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"videos":     videos,
			"expiryDate": expiryDate,
		},
		ContentType: "html",
	}

	if c.config.RetentionWarningTemplateID != 0 {
		tx.TemplateID = c.config.RetentionWarningTemplateID
	} else {
		var titles []string
		for _, v := range videos {
			titles = append(titles, v.Title)
		}
		tx.Body = fmt.Sprintf(
			`<p>Hi,</p><p>The following videos will be deleted on <strong>%s</strong>: %s.</p><p>Upgrade your plan to keep them.</p>`,
			expiryDate, strings.Join(titles, ", "),
		)
	}

	return c.sendTx(ctx, tx)
}
