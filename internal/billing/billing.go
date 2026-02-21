package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func New(apiKey, baseURL string) *Client {
	if baseURL == "" {
		if strings.HasPrefix(apiKey, "creem_test_") {
			baseURL = "https://test-api.creem.io"
		} else {
			baseURL = "https://api.creem.io"
		}
	}
	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

type checkoutRequest struct {
	ProductID  string            `json:"product_id"`
	SuccessURL string            `json:"success_url"`
	Metadata   map[string]string `json:"metadata"`
}

type checkoutResponse struct {
	CheckoutURL string `json:"checkout_url"`
}

func (c *Client) CreateCheckout(ctx context.Context, productID, userID, successURL string) (string, error) {
	body, err := json.Marshal(checkoutRequest{
		ProductID:  productID,
		SuccessURL: successURL,
		Metadata:   map[string]string{"userId": userID},
	})
	if err != nil {
		return "", fmt.Errorf("marshal checkout request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/checkouts", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create checkout request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("checkout request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("creem checkout returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result checkoutResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode checkout response: %w", err)
	}
	return result.CheckoutURL, nil
}

type SubscriptionInfo struct {
	ID               string   `json:"id"`
	Status           string   `json:"status"`
	CurrentPeriodEnd string   `json:"current_period_end"`
	Customer         Customer `json:"customer"`
}

type Customer struct {
	PortalURL string `json:"portal_url"`
}

func (c *Client) GetSubscription(ctx context.Context, subscriptionID string) (*SubscriptionInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/subscriptions?subscription_id="+subscriptionID, nil)
	if err != nil {
		return nil, fmt.Errorf("create subscription request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("subscription request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("creem subscription returned %d: %s", resp.StatusCode, string(respBody))
	}

	var info SubscriptionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode subscription response: %w", err)
	}
	return &info, nil
}

func (c *Client) CancelSubscription(ctx context.Context, subscriptionID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/subscriptions/"+subscriptionID+"/cancel", nil)
	if err != nil {
		return fmt.Errorf("create cancel request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cancel request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("creem cancel returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
