package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
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
	DeveloperEmail               string
	FromAddress                  string
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
	subject         string         // unexported; used only for sendmail fallback
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
	if c.config.DeveloperEmail != "" {
		return true
	}
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

func (c *Client) resolveRecipient(email string) string {
	if c.config.DeveloperEmail != "" {
		return c.config.DeveloperEmail
	}
	return email
}

func (c *Client) ensureSubscriber(ctx context.Context, email, name string) {
	body := subscriberRequest{Email: c.resolveRecipient(email), Name: name, Status: "enabled"}
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
	body.SubscriberEmail = c.resolveRecipient(body.SubscriberEmail)

	if c.config.BaseURL == "" {
		return c.sendViaSendmail(ctx, body.SubscriberEmail, body.subject, body.Body)
	}

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
		slog.Warn("listmonk request failed, falling back to sendmail", "error", err)
		return c.sendViaSendmail(ctx, body.SubscriberEmail, body.subject, body.Body)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("listmonk returned error, falling back to sendmail", "status", resp.StatusCode)
		return c.sendViaSendmail(ctx, body.SubscriberEmail, body.subject, body.Body)
	}

	return nil
}

func (c *Client) sendViaSendmail(ctx context.Context, to, subject, htmlBody string) error {
	if _, err := exec.LookPath("sendmail"); err != nil {
		slog.Warn("sendmail not available, skipping email", "to", to, "subject", subject)
		return nil
	}

	from := c.config.FromAddress
	if from == "" {
		from = "noreply@sendrec.eu"
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		from, to, subject, htmlBody)

	cmd := exec.CommandContext(ctx, "sendmail", "-t")
	cmd.Stdin = strings.NewReader(msg)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sendmail: %w", err)
	}
	return nil
}

func (c *Client) SendPasswordReset(ctx context.Context, toEmail, toName, resetLink string) error {
	if !c.isAllowed(toEmail) {
		return nil
	}

	if c.config.BaseURL != "" {
		c.ensureSubscriber(ctx, toEmail, toName)
	}

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"resetLink": resetLink,
			"name":      toName,
		},
		ContentType: "html",
		subject:     "Reset your password",
		Body: fmt.Sprintf(
			`<p>Hi %s,</p><p>Click the link below to reset your password:</p><p><a href="%s">Reset password</a></p>`,
			toName, resetLink,
		),
	}

	if c.config.TemplateID != 0 {
		tx.TemplateID = c.config.TemplateID
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendCommentNotification(ctx context.Context, toEmail, toName, videoTitle, commentAuthor, commentBody, watchURL string) error {
	if !c.isAllowed(toEmail) {
		return nil
	}

	if c.config.BaseURL != "" {
		c.ensureSubscriber(ctx, toEmail, toName)
	}

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
		subject:     "New comment on your video",
		Body: fmt.Sprintf(
			`<p>Hi %s,</p><p><strong>%s</strong> commented on your video <strong>%s</strong>:</p><blockquote>%s</blockquote><p><a href="%s">View video</a></p>`,
			toName, commentAuthor, videoTitle, commentBody, watchURL,
		),
	}

	if c.config.CommentTemplateID != 0 {
		tx.TemplateID = c.config.CommentTemplateID
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendViewNotification(ctx context.Context, toEmail, toName, videoTitle, watchURL string, viewCount int) error {
	if !c.isAllowed(toEmail) {
		return nil
	}

	if c.config.BaseURL != "" {
		c.ensureSubscriber(ctx, toEmail, toName)
	}

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
		subject:     "Your video was viewed",
		Body: fmt.Sprintf(
			`<p>Hi %s,</p><p>Your video <strong>%s</strong> has been viewed %d time(s).</p><p><a href="%s">View video</a></p>`,
			toName, videoTitle, viewCount, watchURL,
		),
	}

	if c.config.ViewTemplateID != 0 {
		tx.TemplateID = c.config.ViewTemplateID
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendConfirmation(ctx context.Context, toEmail, toName, confirmLink string) error {
	if c.config.BaseURL != "" {
		c.ensureSubscriber(ctx, toEmail, toName)
	}

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"confirmLink": confirmLink,
			"name":        toName,
		},
		ContentType: "html",
		subject:     "Confirm your email",
		Body: fmt.Sprintf(
			`<p>Hi %s,</p><p>Please confirm your email address by clicking the link below:</p><p><a href="%s">Confirm email</a></p>`,
			toName, confirmLink,
		),
	}

	if c.config.ConfirmTemplateID != 0 {
		tx.TemplateID = c.config.ConfirmTemplateID
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendWelcome(ctx context.Context, toEmail, toName, dashboardURL string) error {
	if c.config.BaseURL != "" {
		c.ensureSubscriber(ctx, toEmail, toName)
	}

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"name":         toName,
			"dashboardURL": dashboardURL,
		},
		ContentType: "html",
		subject:     "Welcome to SendRec",
		Body: fmt.Sprintf(
			`<p>Hi %s,</p><p>Welcome to SendRec! Your account is ready.</p><p><a href="%s">Go to dashboard</a></p>`,
			toName, dashboardURL,
		),
	}

	if c.config.WelcomeTemplateID != 0 {
		tx.TemplateID = c.config.WelcomeTemplateID
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendOnboardingDay2(ctx context.Context, toEmail, toName, dashboardURL string) error {
	if c.config.BaseURL != "" {
		c.ensureSubscriber(ctx, toEmail, toName)
	}

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"name":         toName,
			"dashboardURL": dashboardURL,
		},
		ContentType: "html",
		subject:     "Ready to share your first video?",
		Body: fmt.Sprintf(
			`<p>Hi %s,</p><p>Ready to share your first video? Record and share in seconds.</p><p><a href="%s">Get started</a></p>`,
			toName, dashboardURL,
		),
	}

	if c.config.OnboardingDay2TemplateID != 0 {
		tx.TemplateID = c.config.OnboardingDay2TemplateID
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendOnboardingDay7(ctx context.Context, toEmail, toName, dashboardURL string) error {
	if c.config.BaseURL != "" {
		c.ensureSubscriber(ctx, toEmail, toName)
	}

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"name":         toName,
			"dashboardURL": dashboardURL,
		},
		ContentType: "html",
		subject:     "Unlock more with SendRec Pro",
		Body: fmt.Sprintf(
			`<p>Hi %s,</p><p>Unlock more with SendRec Pro — longer recordings, custom branding, and more.</p><p><a href="%s">Learn more</a></p>`,
			toName, dashboardURL,
		),
	}

	if c.config.OnboardingDay7TemplateID != 0 {
		tx.TemplateID = c.config.OnboardingDay7TemplateID
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendDigestNotification(ctx context.Context, toEmail, toName string, videos []DigestVideoSummary) error {
	if !c.isAllowed(toEmail) {
		return nil
	}

	if c.config.BaseURL != "" {
		c.ensureSubscriber(ctx, toEmail, toName)
	}

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
		subject:     "Your weekly video digest",
		Body: fmt.Sprintf(
			`<p>Hi %s,</p><p>Your videos received %d view(s) and %d comment(s) this week.</p>`,
			toName, totalViews, totalComments,
		),
	}

	if c.config.ViewTemplateID != 0 {
		tx.TemplateID = c.config.ViewTemplateID
	}

	return c.sendTx(ctx, tx)
}

func (c *Client) SendOrgInvite(ctx context.Context, toEmail, orgName, inviterName, acceptLink string) error {
	if c.config.BaseURL != "" {
		c.ensureSubscriber(ctx, toEmail, "")
	}

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"orgName":     orgName,
			"inviterName": inviterName,
			"acceptLink":  acceptLink,
		},
		ContentType: "html",
		subject:     fmt.Sprintf("Join %s on SendRec", orgName),
		Body: fmt.Sprintf(
			`<p>Hi,</p><p><strong>%s</strong> has invited you to join <strong>%s</strong> on SendRec.</p><p><a href="%s">Accept invitation</a></p>`,
			inviterName, orgName, acceptLink,
		),
	}

	if c.config.OrgInviteTemplateID != 0 {
		tx.TemplateID = c.config.OrgInviteTemplateID
	}

	return c.sendTx(ctx, tx)
}

// RetentionVideoSummary represents a single video in a retention warning email.
type RetentionVideoSummary struct {
	Title    string `json:"title"`
	WatchURL string `json:"watchURL"`
}

func (c *Client) SendRetentionWarning(ctx context.Context, toEmail string, videos []RetentionVideoSummary, expiryDate string) error {
	if c.config.BaseURL != "" {
		c.ensureSubscriber(ctx, toEmail, "")
	}

	var titles []string
	for _, v := range videos {
		titles = append(titles, v.Title)
	}

	tx := txRequest{
		SubscriberEmail: toEmail,
		Data: map[string]any{
			"videos":     videos,
			"expiryDate": expiryDate,
		},
		ContentType: "html",
		subject:     "Videos scheduled for deletion",
		Body: fmt.Sprintf(
			`<p>Hi,</p><p>The following videos will be deleted on <strong>%s</strong>: %s.</p><p>Upgrade your plan to keep them.</p>`,
			expiryDate, strings.Join(titles, ", "),
		),
	}

	if c.config.RetentionWarningTemplateID != 0 {
		tx.TemplateID = c.config.RetentionWarningTemplateID
	}

	return c.sendTx(ctx, tx)
}
