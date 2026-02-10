package email

import (
	"context"
	"encoding/json"
	"io"
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

func TestSendCommentNotification_UsesCommentTemplateID(t *testing.T) {
	var received txRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:           srv.URL,
		Username:          "user",
		Password:          "pass",
		TemplateID:        10,
		CommentTemplateID: 42,
	})

	err := client.SendCommentNotification(context.Background(),
		"alice@example.com", "Alice", "My Video", "Bob", "Nice video!", "https://example.com/watch/abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received.TemplateID != 42 {
		t.Errorf("expected template_id=42, got %d", received.TemplateID)
	}
}

func TestSendCommentNotification_LogsWarningWhenTemplateIDZero(t *testing.T) {
	serverHit := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:           srv.URL,
		Username:          "user",
		Password:          "pass",
		TemplateID:        10,
		CommentTemplateID: 0,
	})

	err := client.SendCommentNotification(context.Background(),
		"alice@example.com", "Alice", "My Video", "Bob", "Nice video!", "https://example.com/watch/abc")
	if err != nil {
		t.Fatalf("expected nil error when CommentTemplateID is zero, got: %v", err)
	}

	if serverHit {
		t.Error("expected no HTTP request when CommentTemplateID is zero")
	}
}

func TestSendPasswordReset_StillUsesTemplateID(t *testing.T) {
	var received txRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:           srv.URL,
		Username:          "user",
		Password:          "pass",
		TemplateID:        10,
		CommentTemplateID: 42,
	})

	err := client.SendPasswordReset(context.Background(),
		"alice@example.com", "Alice", "https://example.com/reset/token123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received.TemplateID != 10 {
		t.Errorf("expected template_id=10, got %d", received.TemplateID)
	}
}
