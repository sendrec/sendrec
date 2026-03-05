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

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/auth"
)

func signPayload(t *testing.T, payload interface{}) ([]byte, string) {
	t.Helper()
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte("webhook-secret"))
	mac.Write(payloadBytes)
	signature := hex.EncodeToString(mac.Sum(nil))
	return payloadBytes, signature
}

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
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

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
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

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
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

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

	mock.ExpectExec(`INSERT INTO creem_webhook_events`).
		WithArgs("evt_001", "subscription.active", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectExec(`UPDATE users SET subscription_plan`).
		WithArgs("pro", "sub_001", "cust_001", "user-456").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

	payload := map[string]interface{}{
		"id":         "evt_001",
		"eventType":  "subscription.active",
		"created_at": 1728734325927,
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
	payloadBytes, signature := signPayload(t, payload)

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

func TestWebhookSubscriptionCanceled_GracePeriod(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO creem_webhook_events`).
		WithArgs("evt_002", "subscription.canceled", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// No DB update expected — canceled keeps Pro until expiry
	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

	payload := map[string]interface{}{
		"id":         "evt_002",
		"eventType":  "subscription.canceled",
		"created_at": 1728734325927,
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
	payloadBytes, signature := signPayload(t, payload)

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

func TestWebhookSubscriptionExpired(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO creem_webhook_events`).
		WithArgs("evt_003", "subscription.expired", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectExec(`UPDATE users SET subscription_plan`).
		WithArgs("free", "user-789").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

	payload := map[string]interface{}{
		"id":         "evt_003",
		"eventType":  "subscription.expired",
		"created_at": 1728734325927,
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
	payloadBytes, signature := signPayload(t, payload)

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
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

	payload := `{"eventType":"subscription.active","object":{"id":"sub_001"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/creem", strings.NewReader(payload))
	req.Header.Set("creem-signature", "invalidsignature")
	rec := httptest.NewRecorder()

	handlers.Webhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookDuplicateEventIgnored(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO creem_webhook_events`).
		WithArgs("evt_dup", "subscription.active", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 0))

	// No UPDATE expected — duplicate event is silently ignored

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

	payload := map[string]interface{}{
		"id":         "evt_dup",
		"eventType":  "subscription.active",
		"created_at": 1728734325927,
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
	payloadBytes, signature := signPayload(t, payload)

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

func TestWebhookRefundCreated(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO creem_webhook_events`).
		WithArgs("evt_refund", "refund.created", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectExec(`UPDATE users SET subscription_plan`).
		WithArgs("free", "user-456").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

	payload := map[string]interface{}{
		"id":         "evt_refund",
		"eventType":  "refund.created",
		"created_at": 1728734325927,
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
	payloadBytes, signature := signPayload(t, payload)

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

func TestWebhookDisputeCreated(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO creem_webhook_events`).
		WithArgs("evt_dispute", "dispute.created", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// No UPDATE expected — dispute only logs, no plan change

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

	payload := map[string]interface{}{
		"id":         "evt_dispute",
		"eventType":  "dispute.created",
		"created_at": 1728734325927,
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
	payloadBytes, signature := signPayload(t, payload)

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

func injectUserID(userID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.ContextWithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func TestGetOrgBilling_ShowsInheritedFlag(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs("org-123", "user-123").
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("owner"))

	ownerUserID := "owner-user-1"
	mock.ExpectQuery(`SELECT subscription_plan, creem_subscription_id, creem_customer_id, plan_inherited_from FROM organizations WHERE id = \$1`).
		WithArgs("org-123").
		WillReturnRows(pgxmock.NewRows([]string{"subscription_plan", "creem_subscription_id", "creem_customer_id", "plan_inherited_from"}).
			AddRow("pro", nil, nil, &ownerUserID))

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

	router := chi.NewRouter()
	router.With(injectUserID("user-123")).Get("/api/organizations/{orgId}/billing", handlers.GetOrgBilling)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/organizations/org-123/billing", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["plan"] != "pro" {
		t.Errorf("expected plan pro, got %v", resp["plan"])
	}
	if resp["planInherited"] != true {
		t.Errorf("expected planInherited true, got %v", resp["planInherited"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestGetOrgBilling_NoInheritanceFlag(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`SELECT role FROM organization_members WHERE organization_id = \$1 AND user_id = \$2`).
		WithArgs("org-456", "user-123").
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("member"))

	mock.ExpectQuery(`SELECT subscription_plan, creem_subscription_id, creem_customer_id, plan_inherited_from FROM organizations WHERE id = \$1`).
		WithArgs("org-456").
		WillReturnRows(pgxmock.NewRows([]string{"subscription_plan", "creem_subscription_id", "creem_customer_id", "plan_inherited_from"}).
			AddRow("business", nil, nil, nil))

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

	router := chi.NewRouter()
	router.With(injectUserID("user-123")).Get("/api/organizations/{orgId}/billing", handlers.GetOrgBilling)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/organizations/org-456/billing", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["plan"] != "business" {
		t.Errorf("expected plan business, got %v", resp["plan"])
	}
	if resp["planInherited"] != false {
		t.Errorf("expected planInherited false, got %v", resp["planInherited"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestWebhookActivated_PropagatesInheritedOrgs(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO creem_webhook_events`).
		WithArgs("evt_prop_01", "subscription.active", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// Personal branch: no existing subscription
	mock.ExpectQuery(`SELECT creem_subscription_id FROM users WHERE id = \$1`).
		WithArgs("user-owner").
		WillReturnRows(pgxmock.NewRows([]string{"creem_subscription_id"}).AddRow(nil))

	mock.ExpectExec(`UPDATE users SET subscription_plan`).
		WithArgs("pro", "sub_prop_01", "cust_prop_01", "user-owner").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// Propagate plan to grandfathered orgs
	mock.ExpectExec(`UPDATE organizations SET subscription_plan = \$1, updated_at = now\(\) WHERE plan_inherited_from = \$2`).
		WithArgs("pro", "user-owner").
		WillReturnResult(pgxmock.NewResult("UPDATE", 2))

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

	payload := map[string]interface{}{
		"id":         "evt_prop_01",
		"eventType":  "subscription.active",
		"created_at": 1728734325927,
		"object": map[string]interface{}{
			"id":      "sub_prop_01",
			"product": map[string]interface{}{"id": "prod_pro"},
			"customer": map[string]interface{}{
				"id": "cust_prop_01",
			},
			"metadata": map[string]interface{}{
				"userId": "user-owner",
			},
		},
	}
	payloadBytes, signature := signPayload(t, payload)

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

func TestWebhookCanceled_DowngradesInheritedOrgs(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO creem_webhook_events`).
		WithArgs("evt_exp_01", "subscription.expired", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// Personal branch: downgrade user to free
	mock.ExpectExec(`UPDATE users SET subscription_plan`).
		WithArgs("free", "user-downgrade").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// Downgrade grandfathered orgs
	mock.ExpectExec(`UPDATE organizations SET subscription_plan = 'free', updated_at = now\(\) WHERE plan_inherited_from = \$1`).
		WithArgs("user-downgrade").
		WillReturnResult(pgxmock.NewResult("UPDATE", 3))

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

	payload := map[string]interface{}{
		"id":         "evt_exp_01",
		"eventType":  "subscription.expired",
		"created_at": 1728734325927,
		"object": map[string]interface{}{
			"id":      "sub_exp_01",
			"product": map[string]interface{}{"id": "prod_pro"},
			"customer": map[string]interface{}{
				"id": "cust_exp_01",
			},
			"metadata": map[string]interface{}{
				"userId": "user-downgrade",
			},
		},
	}
	payloadBytes, signature := signPayload(t, payload)

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

func TestOrgSubscription_ClearsInheritance(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO creem_webhook_events`).
		WithArgs("evt_org_01", "subscription.active", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// Org branch: check for old subscription
	mock.ExpectQuery(`SELECT creem_subscription_id FROM organizations WHERE id = \$1`).
		WithArgs("org-direct").
		WillReturnRows(pgxmock.NewRows([]string{"creem_subscription_id"}).AddRow(nil))

	// Org update clears plan_inherited_from
	mock.ExpectExec(`UPDATE organizations SET subscription_plan = \$1, creem_subscription_id = \$2, creem_customer_id = \$3, plan_inherited_from = NULL, updated_at = now\(\) WHERE id = \$4`).
		WithArgs("pro", "sub_org_01", "cust_org_01", "org-direct").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	client := New("test-key", "https://api.creem.io")
	handlers := NewHandlers(mock, client, "https://app.sendrec.eu", "prod_pro", "", "", "", "webhook-secret")

	payload := map[string]interface{}{
		"id":         "evt_org_01",
		"eventType":  "subscription.active",
		"created_at": 1728734325927,
		"object": map[string]interface{}{
			"id":      "sub_org_01",
			"product": map[string]interface{}{"id": "prod_pro"},
			"customer": map[string]interface{}{
				"id": "cust_org_01",
			},
			"metadata": map[string]interface{}{
				"userId": "user-org-owner",
				"orgId":  "org-direct",
			},
		},
	}
	payloadBytes, signature := signPayload(t, payload)

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
