package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/email"
)

func TestSendViewNotification_PostsCorrectPayload(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	var mu sync.Mutex
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &receivedBody)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mock.ExpectQuery(`SELECT np\.slack_webhook_url FROM notification_preferences np JOIN users u ON u\.id = np\.user_id WHERE u\.email = \$1 AND np\.slack_webhook_url IS NOT NULL`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"slack_webhook_url"}).AddRow(server.URL))

	client := New(mock)
	err = client.SendViewNotification(context.Background(), "alice@example.com", "Alice", "Demo Video", "https://app.sendrec.eu/watch/abc123", 5)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if receivedBody == nil {
		t.Fatal("expected HTTP request to Slack webhook, got none")
	}

	blocks, ok := receivedBody["blocks"].([]any)
	if !ok || len(blocks) < 2 {
		t.Fatalf("expected at least 2 blocks, got %v", receivedBody)
	}

	section := blocks[0].(map[string]any)
	if section["type"] != "section" {
		t.Errorf("expected first block type 'section', got %v", section["type"])
	}
	text := section["text"].(map[string]any)
	if text["type"] != "mrkdwn" {
		t.Errorf("expected mrkdwn type, got %v", text["type"])
	}
	mrkdwn := text["text"].(string)
	if mrkdwn != ":eyes: *Someone viewed your video*\n<https://app.sendrec.eu/watch/abc123|Demo Video>" {
		t.Errorf("unexpected view text: %q", mrkdwn)
	}

	contextBlock := blocks[1].(map[string]any)
	if contextBlock["type"] != "context" {
		t.Errorf("expected second block type 'context', got %v", contextBlock["type"])
	}
	elements := contextBlock["elements"].([]any)
	elem := elements[0].(map[string]any)
	if elem["text"] != "5 views so far" {
		t.Errorf("unexpected context text: %v", elem["text"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSendViewNotification_SingleView(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	var mu sync.Mutex
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &receivedBody)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mock.ExpectQuery(`SELECT np\.slack_webhook_url FROM notification_preferences np JOIN users u ON u\.id = np\.user_id WHERE u\.email = \$1 AND np\.slack_webhook_url IS NOT NULL`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"slack_webhook_url"}).AddRow(server.URL))

	client := New(mock)
	err = client.SendViewNotification(context.Background(), "alice@example.com", "Alice", "Demo Video", "https://app.sendrec.eu/watch/abc123", 1)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	blocks := receivedBody["blocks"].([]any)
	contextBlock := blocks[1].(map[string]any)
	elements := contextBlock["elements"].([]any)
	elem := elements[0].(map[string]any)
	if elem["text"] != "1 view so far" {
		t.Errorf("expected singular 'view', got: %v", elem["text"])
	}
}

func TestSendViewNotification_NoWebhook_Skips(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	httpCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mock.ExpectQuery(`SELECT np\.slack_webhook_url FROM notification_preferences np JOIN users u ON u\.id = np\.user_id WHERE u\.email = \$1 AND np\.slack_webhook_url IS NOT NULL`).
		WithArgs("bob@example.com").
		WillReturnError(pgx.ErrNoRows)

	client := New(mock)
	err = client.SendViewNotification(context.Background(), "bob@example.com", "Bob", "My Video", "https://app.sendrec.eu/watch/xyz", 3)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if httpCalled {
		t.Error("expected no HTTP call when no webhook URL found")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSendCommentNotification_PostsCorrectPayload(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	var mu sync.Mutex
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &receivedBody)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mock.ExpectQuery(`SELECT np\.slack_webhook_url FROM notification_preferences np JOIN users u ON u\.id = np\.user_id WHERE u\.email = \$1 AND np\.slack_webhook_url IS NOT NULL`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"slack_webhook_url"}).AddRow(server.URL))

	client := New(mock)
	err = client.SendCommentNotification(context.Background(), "alice@example.com", "Alice", "Demo Video", "Bob", "Great video!", "https://app.sendrec.eu/watch/abc123")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if receivedBody == nil {
		t.Fatal("expected HTTP request to Slack webhook, got none")
	}

	blocks, ok := receivedBody["blocks"].([]any)
	if !ok || len(blocks) < 2 {
		t.Fatalf("expected at least 2 blocks, got %v", receivedBody)
	}

	section := blocks[0].(map[string]any)
	text := section["text"].(map[string]any)
	mrkdwn := text["text"].(string)
	if mrkdwn != ":speech_balloon: *New comment on your video*\n<https://app.sendrec.eu/watch/abc123|Demo Video>" {
		t.Errorf("unexpected comment header: %q", mrkdwn)
	}

	commentSection := blocks[1].(map[string]any)
	commentText := commentSection["text"].(map[string]any)
	commentMrkdwn := commentText["text"].(string)
	if commentMrkdwn != "*Bob* said:\n> Great video!" {
		t.Errorf("unexpected comment body: %q", commentMrkdwn)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSendTestMessage_PostsCorrectPayload(t *testing.T) {
	var mu sync.Mutex
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &receivedBody)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := SendTestMessage(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if receivedBody == nil {
		t.Fatal("expected HTTP request, got none")
	}

	blocks, ok := receivedBody["blocks"].([]any)
	if !ok || len(blocks) < 1 {
		t.Fatalf("expected at least 1 block, got %v", receivedBody)
	}

	section := blocks[0].(map[string]any)
	text := section["text"].(map[string]any)
	mrkdwn := text["text"].(string)
	expected := ":white_check_mark: *SendRec is connected!*\nSlack notifications are working. You'll receive messages here when someone views or comments on your videos."
	if mrkdwn != expected {
		t.Errorf("unexpected test message: %q", mrkdwn)
	}
}

func TestSendDigestNotification_PostsCorrectPayload(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	var mu sync.Mutex
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &receivedBody)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mock.ExpectQuery(`SELECT np\.slack_webhook_url FROM notification_preferences np JOIN users u ON u\.id = np\.user_id WHERE u\.email = \$1 AND np\.slack_webhook_url IS NOT NULL`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"slack_webhook_url"}).AddRow(server.URL))

	client := New(mock)
	videos := []email.DigestVideoSummary{
		{Title: "Video A", ViewCount: 5, CommentCount: 2, WatchURL: "https://app.sendrec.eu/watch/aaa"},
		{Title: "Video B", ViewCount: 3, CommentCount: 0, WatchURL: "https://app.sendrec.eu/watch/bbb"},
	}
	err = client.SendDigestNotification(context.Background(), "alice@example.com", "Alice", videos)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if receivedBody == nil {
		t.Fatal("expected HTTP request to Slack webhook, got none")
	}

	blocks, ok := receivedBody["blocks"].([]any)
	if !ok || len(blocks) < 1 {
		t.Fatalf("expected at least 1 block, got %v", receivedBody)
	}

	section := blocks[0].(map[string]any)
	text := section["text"].(map[string]any)
	mrkdwn := text["text"].(string)

	expected := ":bar_chart: *Daily video digest*\n" +
		"\u2022 <https://app.sendrec.eu/watch/aaa|Video A> \u2014 5 views, 2 comments\n" +
		"\u2022 <https://app.sendrec.eu/watch/bbb|Video B> \u2014 3 views"
	if mrkdwn != expected {
		t.Errorf("unexpected digest text:\ngot:  %q\nwant: %q", mrkdwn, expected)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSendViewNotification_SlackError_ReturnsNil(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	mock.ExpectQuery(`SELECT np\.slack_webhook_url FROM notification_preferences np JOIN users u ON u\.id = np\.user_id WHERE u\.email = \$1 AND np\.slack_webhook_url IS NOT NULL`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"slack_webhook_url"}).AddRow(server.URL))

	client := New(mock)
	err = client.SendViewNotification(context.Background(), "alice@example.com", "Alice", "My Video", "https://app.sendrec.eu/watch/abc", 1)
	if err != nil {
		t.Fatalf("expected nil error on Slack failure, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
