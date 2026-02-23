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
	Allowlist         []string
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
	TemplateID      int            `json:"template_id"`
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

	if c.config.CommentTemplateID == 0 {
		slog.Warn("comment template ID not set, skipping comment notification", "video_title", videoTitle)
		return nil
	}

	if !c.isAllowed(toEmail) {
		return nil
	}

	c.ensureSubscriber(ctx, toEmail, toName)

	body := txRequest{
		SubscriberEmail: toEmail,
		TemplateID:      c.config.CommentTemplateID,
		Data: map[string]any{
			"name":          toName,
			"videoTitle":    videoTitle,
			"commentAuthor": commentAuthor,
			"commentBody":   commentBody,
			"watchURL":      watchURL,
		},
		ContentType: "html",
	}

	return c.sendTx(ctx, body)
}

func (c *Client) SendViewNotification(ctx context.Context, toEmail, toName, videoTitle, watchURL string, viewCount int) error {
	if c.config.BaseURL == "" {
		slog.Warn("email not configured, view notification skipped", "video_title", videoTitle)
		return nil
	}

	if c.config.ViewTemplateID == 0 {
		slog.Warn("view template ID not set, skipping view notification", "video_title", videoTitle)
		return nil
	}

	if !c.isAllowed(toEmail) {
		return nil
	}

	c.ensureSubscriber(ctx, toEmail, toName)

	body := txRequest{
		SubscriberEmail: toEmail,
		TemplateID:      c.config.ViewTemplateID,
		Data: map[string]any{
			"name":       toName,
			"videoTitle": videoTitle,
			"watchURL":   watchURL,
			"viewCount":  strconv.Itoa(viewCount),
			"isDigest":   "false",
		},
		ContentType: "html",
	}

	return c.sendTx(ctx, body)
}

func (c *Client) SendConfirmation(ctx context.Context, toEmail, toName, confirmLink string) error {
	if c.config.BaseURL == "" {
		slog.Warn("email not configured, confirmation email skipped", "recipient", toEmail)
		return nil
	}

	if c.config.ConfirmTemplateID == 0 {
		slog.Warn("confirm template ID not set, skipping confirmation email", "recipient", toEmail)
		return nil
	}

	// Confirmation emails bypass the allowlist — they must always be sent,
	// even on staging/preview, so new users can complete registration.
	c.ensureSubscriber(ctx, toEmail, toName)

	body := txRequest{
		SubscriberEmail: toEmail,
		TemplateID:      c.config.ConfirmTemplateID,
		Data: map[string]any{
			"confirmLink": confirmLink,
			"name":        toName,
		},
		ContentType: "html",
	}

	return c.sendTx(ctx, body)
}

func (c *Client) SendDigestNotification(ctx context.Context, toEmail, toName string, videos []DigestVideoSummary) error {
	if c.config.BaseURL == "" {
		slog.Warn("email not configured, digest notification skipped")
		return nil
	}

	if c.config.ViewTemplateID == 0 {
		slog.Warn("view template ID not set, skipping digest notification")
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

	body := txRequest{
		SubscriberEmail: toEmail,
		TemplateID:      c.config.ViewTemplateID,
		Data: map[string]any{
			"name":          toName,
			"isDigest":      "true",
			"totalViews":    strconv.Itoa(totalViews),
			"totalComments": strconv.Itoa(totalComments),
			"videos":        videos,
		},
		ContentType: "html",
	}

	return c.sendTx(ctx, body)
}
