package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/email"
)

// Client sends Slack notifications via incoming webhooks.
type Client struct {
	db   database.DBTX
	http *http.Client
}

// New creates a Slack webhook client.
func New(db database.DBTX) *Client {
	return &Client{
		db:   db,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

type block struct {
	Type     string  `json:"type"`
	Text     *text   `json:"text,omitempty"`
	Elements []text  `json:"elements,omitempty"`
}

type text struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type payload struct {
	Blocks []block `json:"blocks"`
}

func (c *Client) lookupWebhookURL(ctx context.Context, userEmail string) (string, error) {
	var webhookURL string
	err := c.db.QueryRow(ctx,
		`SELECT np.slack_webhook_url FROM notification_preferences np JOIN users u ON u.id = np.user_id WHERE u.email = $1 AND np.slack_webhook_url IS NOT NULL`,
		userEmail,
	).Scan(&webhookURL)
	if err != nil {
		return "", err
	}
	return webhookURL, nil
}

func (c *Client) postMessage(ctx context.Context, webhookURL string, p payload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send slack message: %w", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) SendViewNotification(ctx context.Context, toEmail, toName, videoTitle, watchURL string, viewCount int) error {
	webhookURL, err := c.lookupWebhookURL(ctx, toEmail)
	if err != nil {
		log.Printf("slack: no webhook for %s: %v", toEmail, err)
		return nil
	}

	viewWord := "views"
	if viewCount == 1 {
		viewWord = "view"
	}

	p := payload{
		Blocks: []block{
			{
				Type: "section",
				Text: &text{
					Type: "mrkdwn",
					Text: fmt.Sprintf(":eyes: *Someone viewed your video*\n<%s|%s>", watchURL, videoTitle),
				},
			},
			{
				Type: "context",
				Elements: []text{
					{
						Type: "mrkdwn",
						Text: fmt.Sprintf("%d %s so far", viewCount, viewWord),
					},
				},
			},
		},
	}

	if err := c.postMessage(ctx, webhookURL, p); err != nil {
		log.Printf("slack: failed to send view notification: %v", err)
	}
	return nil
}

func (c *Client) SendCommentNotification(ctx context.Context, toEmail, toName, videoTitle, commentAuthor, commentBody, watchURL string) error {
	webhookURL, err := c.lookupWebhookURL(ctx, toEmail)
	if err != nil {
		log.Printf("slack: no webhook for %s: %v", toEmail, err)
		return nil
	}

	p := payload{
		Blocks: []block{
			{
				Type: "section",
				Text: &text{
					Type: "mrkdwn",
					Text: fmt.Sprintf(":speech_balloon: *New comment on your video*\n<%s|%s>", watchURL, videoTitle),
				},
			},
			{
				Type: "section",
				Text: &text{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*%s* said:\n> %s", commentAuthor, commentBody),
				},
			},
		},
	}

	if err := c.postMessage(ctx, webhookURL, p); err != nil {
		log.Printf("slack: failed to send comment notification: %v", err)
	}
	return nil
}

func (c *Client) SendDigestNotification(ctx context.Context, toEmail, toName string, videos []email.DigestVideoSummary) error {
	webhookURL, err := c.lookupWebhookURL(ctx, toEmail)
	if err != nil {
		log.Printf("slack: no webhook for %s: %v", toEmail, err)
		return nil
	}

	var lines []string
	for _, v := range videos {
		line := fmt.Sprintf("\u2022 <%s|%s> \u2014 %d views", v.WatchURL, v.Title, v.ViewCount)
		if v.CommentCount > 0 {
			line += fmt.Sprintf(", %d comments", v.CommentCount)
		}
		lines = append(lines, line)
	}

	p := payload{
		Blocks: []block{
			{
				Type: "section",
				Text: &text{
					Type: "mrkdwn",
					Text: ":bar_chart: *Daily video digest*\n" + strings.Join(lines, "\n"),
				},
			},
		},
	}

	if err := c.postMessage(ctx, webhookURL, p); err != nil {
		log.Printf("slack: failed to send digest notification: %v", err)
	}
	return nil
}

// SendTestMessage posts a test message directly to the given webhook URL without DB lookup.
func SendTestMessage(ctx context.Context, webhookURL string) error {
	p := payload{
		Blocks: []block{
			{
				Type: "section",
				Text: &text{
					Type: "mrkdwn",
					Text: ":white_check_mark: *SendRec is connected!*\nSlack notifications are working. You'll receive messages here when someone views or comments on your videos.",
				},
			},
		},
	}

	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send slack test message: %w", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}

	return nil
}
