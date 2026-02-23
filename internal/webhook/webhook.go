package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/sendrec/sendrec/internal/database"
)

const maxResponseBodyBytes = 1024

// Event represents a webhook event to dispatch.
type Event struct {
	Name      string         `json:"event"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

// Client dispatches webhook events with retries and delivery logging.
type Client struct {
	db          database.DBTX
	http        *http.Client
	retryDelays []time.Duration
}

// New creates a webhook client.
func New(db database.DBTX) *Client {
	return &Client{
		db:          db,
		http:        &http.Client{Timeout: 10 * time.Second},
		retryDelays: []time.Duration{1 * time.Second, 4 * time.Second},
	}
}

// SignPayload computes HMAC-SHA256 of the payload using the secret.
func SignPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Dispatch sends an event to the webhook URL with up to 3 attempts.
// Each attempt is logged to webhook_deliveries.
func (c *Client) Dispatch(ctx context.Context, userID, webhookURL, secret string, event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	signature := SignPayload(secret, body)
	maxAttempts := 1 + len(c.retryDelays)
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		statusCode, respBody, err := c.doPost(ctx, webhookURL, body, signature)
		c.logDelivery(ctx, userID, event.Name, body, statusCode, respBody, attempt)

		if err == nil && statusCode != nil && *statusCode >= 200 && *statusCode < 300 {
			return nil
		}

		if err != nil {
			lastErr = err
		} else if statusCode != nil {
			lastErr = fmt.Errorf("webhook returned status %d", *statusCode)
		}

		if attempt < maxAttempts {
			select {
			case <-time.After(c.retryDelays[attempt-1]):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return lastErr
}

func (c *Client) doPost(ctx context.Context, url string, body []byte, signature string) (*int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err.Error(), err
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, int64(maxResponseBodyBytes)+1))
	respBody := string(respBytes)
	if len(respBody) > maxResponseBodyBytes {
		respBody = respBody[:maxResponseBodyBytes]
	}

	return &resp.StatusCode, respBody, nil
}

func (c *Client) logDelivery(ctx context.Context, userID, event string, payload []byte, statusCode *int, responseBody string, attempt int) {
	if _, err := c.db.Exec(ctx,
		`INSERT INTO webhook_deliveries (user_id, event, payload, status_code, response_body, attempt)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, event, payload, statusCode, responseBody, attempt,
	); err != nil {
		slog.Error("webhook: failed to log delivery", "user_id", userID, "error", err)
	}
}

// LookupConfig fetches the webhook URL and secret for a user by email.
func (c *Client) LookupConfig(ctx context.Context, userEmail string) (webhookURL, secret string, err error) {
	var url, sec *string
	err = c.db.QueryRow(ctx,
		`SELECT np.webhook_url, np.webhook_secret
		 FROM notification_preferences np
		 JOIN users u ON u.id = np.user_id
		 WHERE u.email = $1 AND np.webhook_url IS NOT NULL`,
		userEmail,
	).Scan(&url, &sec)
	if err != nil {
		return "", "", err
	}
	if url == nil || sec == nil {
		return "", "", fmt.Errorf("no webhook configured")
	}
	return *url, *sec, nil
}

// LookupConfigByUserID fetches the webhook URL and secret for a user by ID.
func (c *Client) LookupConfigByUserID(ctx context.Context, userID string) (webhookURL, secret string, err error) {
	var url, sec *string
	err = c.db.QueryRow(ctx,
		`SELECT webhook_url, webhook_secret
		 FROM notification_preferences
		 WHERE user_id = $1 AND webhook_url IS NOT NULL`,
		userID,
	).Scan(&url, &sec)
	if err != nil {
		return "", "", err
	}
	if url == nil || sec == nil {
		return "", "", fmt.Errorf("no webhook configured")
	}
	return *url, *sec, nil
}
