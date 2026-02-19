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
		if r.URL.Path == "/api/subscribers" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"id":1}}`))
			return
		}
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
		if r.URL.Path == "/api/subscribers" {
			w.WriteHeader(http.StatusOK)
			return
		}
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
		if r.URL.Path == "/api/subscribers" {
			w.WriteHeader(http.StatusOK)
			return
		}
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
		if r.URL.Path == "/api/subscribers" {
			w.WriteHeader(http.StatusOK)
			return
		}
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

func TestSendViewNotification_Success(t *testing.T) {
	var received txRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/subscribers" {
			w.WriteHeader(http.StatusOK)
			return
		}
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
		BaseURL:        srv.URL,
		Username:       "user",
		Password:       "pass",
		ViewTemplateID: 99,
	})

	err := client.SendViewNotification(context.Background(),
		"alice@example.com", "Alice", "My Video", "https://example.com/watch/abc", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received.TemplateID != 99 {
		t.Errorf("expected template_id=99, got %d", received.TemplateID)
	}
	if received.Data["videoTitle"] != "My Video" {
		t.Errorf("expected videoTitle=My Video, got %s", received.Data["videoTitle"])
	}
	if received.Data["viewCount"] != "5" {
		t.Errorf("expected viewCount=5, got %s", received.Data["viewCount"])
	}
	if received.Data["isDigest"] != "false" {
		t.Errorf("expected isDigest=false, got %s", received.Data["isDigest"])
	}
}

func TestSendViewNotification_SkipsWhenTemplateIDZero(t *testing.T) {
	serverHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:        srv.URL,
		Username:       "user",
		Password:       "pass",
		ViewTemplateID: 0,
	})

	err := client.SendViewNotification(context.Background(),
		"alice@example.com", "Alice", "My Video", "https://example.com/watch/abc", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if serverHit {
		t.Error("expected no HTTP request when ViewTemplateID is zero")
	}
}

func TestSendDigestNotification_Success(t *testing.T) {
	var receivedRaw json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/subscribers" {
			w.WriteHeader(http.StatusOK)
			return
		}
		body, _ := io.ReadAll(r.Body)
		receivedRaw = body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:        srv.URL,
		Username:       "user",
		Password:       "pass",
		ViewTemplateID: 99,
	})

	videos := []DigestVideoSummary{
		{Title: "Video One", ViewCount: 10, CommentCount: 2, WatchURL: "https://example.com/watch/abc"},
		{Title: "Video Two", ViewCount: 3, CommentCount: 1, WatchURL: "https://example.com/watch/def"},
	}

	err := client.SendDigestNotification(context.Background(), "alice@example.com", "Alice", videos)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed struct {
		TemplateID int `json:"template_id"`
		Data       struct {
			IsDigest      string               `json:"isDigest"`
			TotalViews    string               `json:"totalViews"`
			TotalComments string               `json:"totalComments"`
			Videos        []DigestVideoSummary `json:"videos"`
		} `json:"data"`
	}
	if err := json.Unmarshal(receivedRaw, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.TemplateID != 99 {
		t.Errorf("expected template_id=99, got %d", parsed.TemplateID)
	}
	if parsed.Data.IsDigest != "true" {
		t.Errorf("expected isDigest=true, got %s", parsed.Data.IsDigest)
	}
	if parsed.Data.TotalViews != "13" {
		t.Errorf("expected totalViews=13, got %s", parsed.Data.TotalViews)
	}
	if parsed.Data.TotalComments != "3" {
		t.Errorf("expected totalComments=3, got %s", parsed.Data.TotalComments)
	}
	if len(parsed.Data.Videos) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(parsed.Data.Videos))
	}
	if parsed.Data.Videos[0].Title != "Video One" {
		t.Errorf("expected first video title 'Video One', got %q", parsed.Data.Videos[0].Title)
	}
	if parsed.Data.Videos[0].ViewCount != 10 {
		t.Errorf("expected first video 10 views, got %d", parsed.Data.Videos[0].ViewCount)
	}
	if parsed.Data.Videos[0].CommentCount != 2 {
		t.Errorf("expected first video 2 comments, got %d", parsed.Data.Videos[0].CommentCount)
	}
}

func TestAllowlist_EmptyAllowsAll(t *testing.T) {
	serverHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:        srv.URL,
		Username:       "user",
		Password:       "pass",
		ViewTemplateID: 99,
	})

	_ = client.SendViewNotification(context.Background(),
		"anyone@anywhere.com", "Anyone", "Video", "https://example.com/watch/abc", 1)

	if !serverHit {
		t.Error("expected email to be sent when allowlist is empty")
	}
}

func TestAllowlist_BlocksNonMatchingEmail(t *testing.T) {
	serverHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:        srv.URL,
		Username:       "user",
		Password:       "pass",
		ViewTemplateID: 99,
		Allowlist:      []string{"@sendrec.eu"},
	})

	err := client.SendViewNotification(context.Background(),
		"stranger@example.com", "Stranger", "Video", "https://example.com/watch/abc", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if serverHit {
		t.Error("expected email to be blocked for non-matching address")
	}
}

func TestAllowlist_AllowsDomainMatch(t *testing.T) {
	serverHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:        srv.URL,
		Username:       "user",
		Password:       "pass",
		ViewTemplateID: 99,
		Allowlist:      []string{"@sendrec.eu"},
	})

	_ = client.SendViewNotification(context.Background(),
		"alice@sendrec.eu", "Alice", "Video", "https://example.com/watch/abc", 1)

	if !serverHit {
		t.Error("expected email to be sent for domain match")
	}
}

func TestAllowlist_AllowsExactEmailMatch(t *testing.T) {
	serverHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:        srv.URL,
		Username:       "user",
		Password:       "pass",
		ViewTemplateID: 99,
		Allowlist:      []string{"alice@example.com"},
	})

	_ = client.SendViewNotification(context.Background(),
		"alice@example.com", "Alice", "Video", "https://example.com/watch/abc", 1)

	if !serverHit {
		t.Error("expected email to be sent for exact email match")
	}
}

func TestAllowlist_MultipleDomains(t *testing.T) {
	serverHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:        srv.URL,
		Username:       "user",
		Password:       "pass",
		ViewTemplateID: 99,
		Allowlist:      []string{"@sendrec.eu", "bob@test.com"},
	})

	_ = client.SendViewNotification(context.Background(),
		"bob@test.com", "Bob", "Video", "https://example.com/watch/abc", 1)

	if !serverHit {
		t.Error("expected email to be sent for exact match in multi-entry allowlist")
	}
}

func TestAllowlist_BlocksAllSendMethods(t *testing.T) {
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
		TemplateID:        1,
		CommentTemplateID: 2,
		ViewTemplateID:    3,
		Allowlist:         []string{"@sendrec.eu"},
	})

	_ = client.SendPasswordReset(context.Background(),
		"blocked@example.com", "Blocked", "https://example.com/reset")
	if serverHit {
		t.Error("expected SendPasswordReset to be blocked")
	}

	serverHit = false
	_ = client.SendCommentNotification(context.Background(),
		"blocked@example.com", "Blocked", "Video", "Bob", "Nice!", "https://example.com/watch/abc")
	if serverHit {
		t.Error("expected SendCommentNotification to be blocked")
	}

	serverHit = false
	_ = client.SendViewNotification(context.Background(),
		"blocked@example.com", "Blocked", "Video", "https://example.com/watch/abc", 1)
	if serverHit {
		t.Error("expected SendViewNotification to be blocked")
	}
}

func TestAllowlist_CaseInsensitive(t *testing.T) {
	serverHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(Config{
		BaseURL:        srv.URL,
		Username:       "user",
		Password:       "pass",
		ViewTemplateID: 99,
		Allowlist:      []string{"@SendRec.EU"},
	})

	_ = client.SendViewNotification(context.Background(),
		"Alice@sendrec.eu", "Alice", "Video", "https://example.com/watch/abc", 1)

	if !serverHit {
		t.Error("expected case-insensitive domain match")
	}
}

func TestSendConfirmation_BypassesAllowlist(t *testing.T) {
	var received txRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/subscribers" {
			w.WriteHeader(http.StatusOK)
			return
		}
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
		ConfirmTemplateID: 7,
		Allowlist:         []string{"@sendrec.eu"},
	})

	err := client.SendConfirmation(context.Background(),
		"stranger@example.com", "Stranger", "https://app.sendrec.eu/confirm-email?token=abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received.TemplateID != 7 {
		t.Errorf("expected template_id=7, got %d", received.TemplateID)
	}
	if received.SubscriberEmail != "stranger@example.com" {
		t.Errorf("expected subscriber email stranger@example.com, got %q", received.SubscriberEmail)
	}
	confirmLink, ok := received.Data["confirmLink"]
	if !ok || confirmLink != "https://app.sendrec.eu/confirm-email?token=abc123" {
		t.Errorf("expected confirmLink in data, got %v", received.Data)
	}
}

func TestSendConfirmation_SkipsWhenTemplateIDZero(t *testing.T) {
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
		ConfirmTemplateID: 0,
	})

	err := client.SendConfirmation(context.Background(),
		"alice@example.com", "Alice", "https://example.com/confirm-email?token=abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if serverHit {
		t.Error("expected no HTTP request when ConfirmTemplateID is zero")
	}
}

func TestParseAllowlist(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"@sendrec.eu", []string{"@sendrec.eu"}},
		{"@sendrec.eu,alice@test.com", []string{"@sendrec.eu", "alice@test.com"}},
		{"  @sendrec.eu , alice@test.com  ", []string{"@sendrec.eu", "alice@test.com"}},
	}

	for _, tt := range tests {
		result := ParseAllowlist(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("ParseAllowlist(%q): got %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("ParseAllowlist(%q)[%d]: got %q, want %q", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}
