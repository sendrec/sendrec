package video

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAIClient_GenerateSummary(t *testing.T) {
	summaryJSON := `{"summary":"This video covers Go testing patterns.","chapters":[{"title":"Introduction to Testing","start":0},{"title":"Writing Table Tests","start":45.5},{"title":"Mocking Dependencies","start":120}]}`

	var receivedModel string
	var receivedAuth string
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedContentType = r.Header.Get("Content-Type")

		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		_ = json.Unmarshal(body, &req)
		receivedModel = req.Model

		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: summaryJSON}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewAIClient(server.URL, "test-api-key", "gpt-4")
	result, err := client.GenerateSummary(context.Background(), "00:00 Hello world 00:45 Testing patterns")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Summary != "This video covers Go testing patterns." {
		t.Errorf("summary = %q, want %q", result.Summary, "This video covers Go testing patterns.")
	}

	if len(result.Chapters) != 3 {
		t.Fatalf("chapter count = %d, want 3", len(result.Chapters))
	}

	if result.Chapters[0].Title != "Introduction to Testing" {
		t.Errorf("chapter[0].title = %q, want %q", result.Chapters[0].Title, "Introduction to Testing")
	}
	if result.Chapters[0].Start != 0 {
		t.Errorf("chapter[0].start = %f, want 0", result.Chapters[0].Start)
	}
	if result.Chapters[1].Start != 45.5 {
		t.Errorf("chapter[1].start = %f, want 45.5", result.Chapters[1].Start)
	}

	if receivedAuth != "Bearer test-api-key" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer test-api-key")
	}
	if receivedContentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", receivedContentType, "application/json")
	}
	if receivedModel != "gpt-4" {
		t.Errorf("model = %q, want %q", receivedModel, "gpt-4")
	}
}

func TestAIClient_GenerateSummary_MarkdownFence(t *testing.T) {
	fencedJSON := "```json\n{\"summary\":\"A fenced response.\",\"chapters\":[{\"title\":\"Start Here\",\"start\":0}]}\n```"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: fencedJSON}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewAIClient(server.URL, "key", "model")
	result, err := client.GenerateSummary(context.Background(), "transcript")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Summary != "A fenced response." {
		t.Errorf("summary = %q, want %q", result.Summary, "A fenced response.")
	}

	if len(result.Chapters) != 1 {
		t.Fatalf("chapter count = %d, want 1", len(result.Chapters))
	}

	if result.Chapters[0].Title != "Start Here" {
		t.Errorf("chapter[0].title = %q, want %q", result.Chapters[0].Title, "Start Here")
	}
}

func TestAIClient_GenerateSummary_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: "this is not json at all"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewAIClient(server.URL, "key", "model")
	_, err := client.GenerateSummary(context.Background(), "transcript")

	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestAIClient_GenerateSummary_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewAIClient(server.URL, "key", "model")
	_, err := client.GenerateSummary(context.Background(), "transcript")

	if err == nil {
		t.Fatal("expected error for empty choices, got nil")
	}

	expected := "AI API returned empty choices"
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestAIClient_GenerateSummary_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer server.Close()

	client := NewAIClient(server.URL, "bad-key", "model")
	_, err := client.GenerateSummary(context.Background(), "transcript")

	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}

	if got := err.Error(); !contains(got, "401") {
		t.Errorf("error = %q, want it to contain %q", got, "401")
	}
}

func TestIsAutoGeneratedTitle(t *testing.T) {
	tests := []struct {
		title string
		want  bool
	}{
		{"Recording 2/20/2026 3:45:12 PM", true},
		{"Recording 1/1/2025", true},
		{"Untitled Recording", true},
		{"Untitled Video", true},
		{"My Custom Title", false},
		{"Recording notes about project", false},
		{"", false},
		{"Product Demo", false},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := isAutoGeneratedTitle(tt.title)
			if got != tt.want {
				t.Errorf("isAutoGeneratedTitle(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}

func TestGenerateTitle(t *testing.T) {
	var receivedModel string
	var receivedMessages []chatMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		_ = json.Unmarshal(body, &req)
		receivedModel = req.Model
		receivedMessages = req.Messages

		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: "Go Testing Patterns Overview"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewAIClient(server.URL, "test-key", "gpt-4")
	title, err := client.GenerateTitle(context.Background(), "[00:00] Hello world\n[00:45] Testing patterns")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if title != "Go Testing Patterns Overview" {
		t.Errorf("title = %q, want %q", title, "Go Testing Patterns Overview")
	}

	if receivedModel != "gpt-4" {
		t.Errorf("model = %q, want %q", receivedModel, "gpt-4")
	}

	if len(receivedMessages) != 2 {
		t.Fatalf("message count = %d, want 2", len(receivedMessages))
	}

	if receivedMessages[0].Role != "system" {
		t.Errorf("messages[0].role = %q, want %q", receivedMessages[0].Role, "system")
	}

	if receivedMessages[0].Content != titleSystemPrompt {
		t.Errorf("messages[0].content = %q, want titleSystemPrompt", receivedMessages[0].Content)
	}
}

func TestGenerateTitleTrimsQuotes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: `"Product Demo Walkthrough"`}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewAIClient(server.URL, "key", "model")
	title, err := client.GenerateTitle(context.Background(), "transcript")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if title != "Product Demo Walkthrough" {
		t.Errorf("title = %q, want %q", title, "Product Demo Walkthrough")
	}
}

func TestGenerateTitleTruncates(t *testing.T) {
	longTitle := ""
	for i := 0; i < 250; i++ {
		longTitle += "a"
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: longTitle}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewAIClient(server.URL, "key", "model")
	title, err := client.GenerateTitle(context.Background(), "transcript")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(title) != 200 {
		t.Errorf("title length = %d, want 200", len(title))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
