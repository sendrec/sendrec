package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateCheckout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/checkouts" {
			t.Errorf("expected /v1/checkouts, got %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key test-key, got %s", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		var body checkoutRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.ProductID != "prod_123" {
			t.Errorf("expected product_id prod_123, got %s", body.ProductID)
		}
		if body.SuccessURL != "https://app.sendrec.eu/settings" {
			t.Errorf("expected success_url https://app.sendrec.eu/settings, got %s", body.SuccessURL)
		}
		if body.Metadata["userId"] != "user-abc" {
			t.Errorf("expected metadata.userId user-abc, got %s", body.Metadata["userId"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(checkoutResponse{
			CheckoutURL: "https://checkout.creem.io/pay/xyz",
		})
	}))
	defer server.Close()

	client := New("test-key", server.URL)
	url, err := client.CreateCheckout(context.Background(), "prod_123", "user-abc", "https://app.sendrec.eu/settings")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://checkout.creem.io/pay/xyz" {
		t.Errorf("expected checkout URL https://checkout.creem.io/pay/xyz, got %s", url)
	}
}

func TestCreateCheckoutError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid product_id"}`))
	}))
	defer server.Close()

	client := New("test-key", server.URL)
	_, err := client.CreateCheckout(context.Background(), "bad-id", "user-abc", "https://example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != `creem checkout returned 400: {"error":"invalid product_id"}` {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestCancelSubscription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/subscriptions/sub_456/cancel" {
			t.Errorf("expected /v1/subscriptions/sub_456/cancel, got %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key test-key, got %s", r.Header.Get("x-api-key"))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New("test-key", server.URL)
	err := client.CancelSubscription(context.Background(), "sub_456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelSubscriptionAlreadyCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":400,"error":"Bad Request","message":"Subscription already canceled"}`))
	}))
	defer server.Close()

	client := New("test-key", server.URL)
	err := client.CancelSubscription(context.Background(), "sub_456")
	if err != nil {
		t.Fatalf("expected nil error for already canceled subscription, got: %v", err)
	}
}

func TestGetPortalURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/subscriptions" {
			t.Errorf("expected /v1/subscriptions, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("subscription_id") != "sub_789" {
			t.Errorf("expected subscription_id=sub_789, got %s", r.URL.Query().Get("subscription_id"))
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key test-key, got %s", r.Header.Get("x-api-key"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SubscriptionInfo{
			ID:               "sub_789",
			Status:           "active",
			CurrentPeriodEnd: "2026-03-21T00:00:00Z",
			Customer: Customer{
				PortalURL: "https://creem.io/portal/cust_abc",
			},
		})
	}))
	defer server.Close()

	client := New("test-key", server.URL)
	info, err := client.GetSubscription(context.Background(), "sub_789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ID != "sub_789" {
		t.Errorf("expected id sub_789, got %s", info.ID)
	}
	if info.Status != "active" {
		t.Errorf("expected status active, got %s", info.Status)
	}
	if info.CurrentPeriodEnd != "2026-03-21T00:00:00Z" {
		t.Errorf("expected current_period_end 2026-03-21T00:00:00Z, got %s", info.CurrentPeriodEnd)
	}
	if info.Customer.PortalURL != "https://creem.io/portal/cust_abc" {
		t.Errorf("expected portal_url https://creem.io/portal/cust_abc, got %s", info.Customer.PortalURL)
	}
}

func TestNew_TestKeySelectsTestURL(t *testing.T) {
	client := New("creem_test_abc", "")
	if client.baseURL != "https://test-api.creem.io" {
		t.Errorf("expected baseURL https://test-api.creem.io, got %s", client.baseURL)
	}
}

func TestNew_ProductionKeySelectsProductionURL(t *testing.T) {
	client := New("creem_live_abc", "")
	if client.baseURL != "https://api.creem.io" {
		t.Errorf("expected baseURL https://api.creem.io, got %s", client.baseURL)
	}
}

func TestNew_CustomURLOverrides(t *testing.T) {
	client := New("creem_test_abc", "https://custom.example.com")
	if client.baseURL != "https://custom.example.com" {
		t.Errorf("expected baseURL https://custom.example.com, got %s", client.baseURL)
	}
}

func TestGetSubscription_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	client := New("test-key", server.URL)
	_, err := client.GetSubscription(context.Background(), "sub_err")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCancelSubscription_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	client := New("test-key", server.URL)
	err := client.CancelSubscription(context.Background(), "sub_err")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
