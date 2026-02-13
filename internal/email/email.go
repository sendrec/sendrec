package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	SubscriberEmail string            `json:"subscriber_email"`
	TemplateID      int               `json:"template_id"`
	Data            map[string]string `json:"data"`
	ContentType     string            `json:"content_type"`
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
	log.Printf("email blocked by allowlist: %s", recipientEmail)
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

func (c *Client) SendPasswordReset(ctx context.Context, toEmail, toName, resetLink string) error {
	if c.config.BaseURL == "" {
		log.Printf("email not configured — password reset requested for %s (link not logged for security)", toEmail)
		return nil
	}

	if !c.isAllowed(toEmail) {
		return nil
	}

	c.ensureSubscriber(ctx, toEmail, toName)

	body := txRequest{
		SubscriberEmail: toEmail,
		TemplateID:      c.config.TemplateID,
		Data: map[string]string{
			"resetLink": resetLink,
			"name":      toName,
		},
		ContentType: "html",
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
		return fmt.Errorf("send email: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("listmonk returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) SendCommentNotification(ctx context.Context, toEmail, toName, videoTitle, commentAuthor, commentBody, watchURL string) error {
	if c.config.BaseURL == "" {
		log.Printf("email not configured — new comment on %q by %s", videoTitle, commentAuthor)
		return nil
	}

	if c.config.CommentTemplateID == 0 {
		log.Printf("LISTMONK_COMMENT_TEMPLATE_ID not set — skipping comment notification for %q", videoTitle)
		return nil
	}

	if !c.isAllowed(toEmail) {
		return nil
	}

	c.ensureSubscriber(ctx, toEmail, toName)

	body := txRequest{
		SubscriberEmail: toEmail,
		TemplateID:      c.config.CommentTemplateID,
		Data: map[string]string{
			"name":          toName,
			"videoTitle":    videoTitle,
			"commentAuthor": commentAuthor,
			"commentBody":   commentBody,
			"watchURL":      watchURL,
		},
		ContentType: "html",
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
		return fmt.Errorf("send comment notification: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("listmonk returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) SendViewNotification(ctx context.Context, toEmail, toName, videoTitle, watchURL string, viewCount int, isDigest bool) error {
	if c.config.BaseURL == "" {
		log.Printf("email not configured — view notification for %q skipped", videoTitle)
		return nil
	}

	if c.config.ViewTemplateID == 0 {
		log.Printf("LISTMONK_VIEW_TEMPLATE_ID not set — skipping view notification for %q", videoTitle)
		return nil
	}

	if !c.isAllowed(toEmail) {
		return nil
	}

	c.ensureSubscriber(ctx, toEmail, toName)

	body := txRequest{
		SubscriberEmail: toEmail,
		TemplateID:      c.config.ViewTemplateID,
		Data: map[string]string{
			"name":       toName,
			"videoTitle": videoTitle,
			"watchURL":   watchURL,
			"viewCount":  strconv.Itoa(viewCount),
			"isDigest":   strconv.FormatBool(isDigest),
		},
		ContentType: "html",
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
		return fmt.Errorf("send view notification: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("listmonk returned status %d", resp.StatusCode)
	}

	return nil
}
