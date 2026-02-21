package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/auth"
)

func TestCheckoutHandler(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	creemServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/checkouts" {
			t.Errorf("expected /v1/checkouts, got %s", r.URL.Path)
		}

		var body checkoutRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.ProductID != "prod_pro" {
			t.Errorf("expected product_id prod_pro, got %s", body.ProductID)
		}
		if body.SuccessURL != "https://app.sendrec.eu/settings?billing=success" {
			t.Errorf("expected success_url https://app.sendrec.eu/settings?billing=success, got %s", body.SuccessURL)
		}
		if body.Metadata["userId"] != "user-123" {
			t.Errorf("expected metadata.userId user-123, got %s", body.Metadata["userId"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(checkoutResponse{
			CheckoutURL: "https://checkout.creem.io/pay/abc",
		})
	}))
	defer creemServer.Close()

	client := New("test-key", creemServer.URL)
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "webhook-secret")

	body := `{"plan":"pro"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/billing/checkout", strings.NewReader(body))
	req = req.WithContext(auth.ContextWithUserID(req.Context(), "user-123"))
	rec := httptest.NewRecorder()

	handlers.CreateCheckout(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["checkoutUrl"] != "https://checkout.creem.io/pay/abc" {
		t.Errorf("expected checkoutUrl https://checkout.creem.io/pay/abc, got %s", resp["checkoutUrl"])
	}
}

func TestCheckoutHandlerInvalidPlan(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "webhook-secret")

	body := `{"plan":"enterprise"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/billing/checkout", strings.NewReader(body))
	req = req.WithContext(auth.ContextWithUserID(req.Context(), "user-123"))
	rec := httptest.NewRecorder()

	handlers.CreateCheckout(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] == "" {
		t.Error("expected non-empty error message")
	}
}

func TestGetBillingFreeUser(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`SELECT subscription_plan, creem_subscription_id, creem_customer_id FROM users`).
		WithArgs("user-123").
		WillReturnRows(pgxmock.NewRows([]string{"subscription_plan", "creem_subscription_id", "creem_customer_id"}).
			AddRow("free", nil, nil))

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "webhook-secret")

	req := httptest.NewRequest(http.MethodGet, "/api/settings/billing", nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), "user-123"))
	rec := httptest.NewRecorder()

	handlers.GetBilling(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["plan"] != "free" {
		t.Errorf("expected plan free, got %v", resp["plan"])
	}
	if resp["subscriptionId"] != nil {
		t.Errorf("expected subscriptionId null, got %v", resp["subscriptionId"])
	}
	if resp["portalUrl"] != nil {
		t.Errorf("expected portalUrl null, got %v", resp["portalUrl"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestWebhookSubscriptionActive(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`UPDATE users SET subscription_plan`).
		WithArgs("pro", "sub_001", "cust_001", "user-456").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "webhook-secret")

	payload := map[string]interface{}{
		"event": "subscription.active",
		"object": map[string]interface{}{
			"id":      "sub_001",
			"product": map[string]interface{}{"id": "prod_pro"},
			"customer": map[string]interface{}{
				"id": "cust_001",
			},
			"metadata": map[string]interface{}{
				"userId": "user-456",
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)

	mac := hmac.New(sha256.New, []byte("webhook-secret"))
	mac.Write(payloadBytes)
	signature := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/creem", strings.NewReader(string(payloadBytes)))
	req.Header.Set("creem-signature", signature)
	rec := httptest.NewRecorder()

	handlers.Webhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestWebhookSubscriptionCanceled(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`UPDATE users SET subscription_plan`).
		WithArgs("free", "user-789").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "webhook-secret")

	payload := map[string]interface{}{
		"event": "subscription.canceled",
		"object": map[string]interface{}{
			"id":      "sub_002",
			"product": map[string]interface{}{"id": "prod_pro"},
			"customer": map[string]interface{}{
				"id": "cust_002",
			},
			"metadata": map[string]interface{}{
				"userId": "user-789",
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)

	mac := hmac.New(sha256.New, []byte("webhook-secret"))
	mac.Write(payloadBytes)
	signature := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/creem", strings.NewReader(string(payloadBytes)))
	req.Header.Set("creem-signature", signature)
	rec := httptest.NewRecorder()

	handlers.Webhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestWebhookInvalidSignature(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "webhook-secret")

	payload := `{"event":"subscription.active","object":{"id":"sub_001"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/creem", strings.NewReader(payload))
	req.Header.Set("creem-signature", "invalidsignature")
	rec := httptest.NewRecorder()

	handlers.Webhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rec.Code, rec.Body.String())
	}
}
