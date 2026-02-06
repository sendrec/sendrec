package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendPasswordReset_Success(t *testing.T) {
	var receivedBody txRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tx" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			t.Errorf("unexpected auth: %s:%s", user, pass)
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": true}`))
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:    srv.URL,
		Username:   "admin",
		Password:   "secret",
		TemplateID: 5,
	})

	err := client.SendPasswordReset(context.Background(), "alice@example.com", "Alice", "https://app.sendrec.eu/reset-password?token=abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedBody.SubscriberEmail != "alice@example.com" {
		t.Errorf("expected subscriber email %q, got %q", "alice@example.com", receivedBody.SubscriberEmail)
	}
	if receivedBody.TemplateID != 5 {
		t.Errorf("expected template ID 5, got %d", receivedBody.TemplateID)
	}
	resetLink, ok := receivedBody.Data["resetLink"]
	if !ok || resetLink != "https://app.sendrec.eu/reset-password?token=abc123" {
		t.Errorf("expected resetLink in data, got %v", receivedBody.Data)
	}
}

func TestSendPasswordReset_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:    srv.URL,
		Username:   "admin",
		Password:   "secret",
		TemplateID: 5,
	})

	err := client.SendPasswordReset(context.Background(), "alice@example.com", "Alice", "https://example.com/reset")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestSendPasswordReset_NoBaseURL(t *testing.T) {
	client := New(Config{})

	// Should not error — just logs to stdout
	err := client.SendPasswordReset(context.Background(), "alice@example.com", "Alice", "https://example.com/reset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
