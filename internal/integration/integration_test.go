package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJiraCreateIssue(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Basic ") {
			t.Errorf("expected Basic auth, got %q", authHeader)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		fields, _ := body["fields"].(map[string]any)
		if fields["summary"] != "My Video" {
			t.Errorf("expected summary 'My Video', got %v", fields["summary"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-123", "self": server.URL + "/rest/api/3/issue/10001"})
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, "user@example.com", "api-token", "PROJ")
	resp, err := client.CreateIssue(context.Background(), CreateIssueRequest{
		Title: "My Video", Description: "Video: https://app.sendrec.eu/watch/abc", VideoURL: "https://app.sendrec.eu/watch/abc",
	})
	if err != nil {
		t.Fatalf("CreateIssue failed: %v", err)
	}
	if resp.IssueKey != "PROJ-123" {
		t.Errorf("unexpected key: %s", resp.IssueKey)
	}
	if resp.IssueURL == "" {
		t.Error("IssueURL should not be empty")
	}
}

func TestJiraValidateConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/myself" {
			json.NewEncoder(w).Encode(map[string]any{"accountId": "123"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, "user@example.com", "api-token", "PROJ")
	if err := client.ValidateConfig(context.Background()); err != nil {
		t.Errorf("ValidateConfig should succeed: %v", err)
	}
}

func TestJiraValidateConfigBadAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, "user@example.com", "bad-token", "PROJ")
	if err := client.ValidateConfig(context.Background()); err == nil {
		t.Error("ValidateConfig should fail with bad auth")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := DeriveKey("test-jwt-secret")
	plaintext := "my-api-token-12345"

	encrypted, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if encrypted == plaintext {
		t.Error("encrypted text should differ from plaintext")
	}

	decrypted, err := Decrypt(key, encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	key := DeriveKey("test-jwt-secret")
	c1, _ := Encrypt(key, "same-token")
	c2, _ := Encrypt(key, "same-token")
	if c1 == c2 {
		t.Error("two encryptions of same plaintext should differ (unique nonce)")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1 := DeriveKey("secret-one")
	key2 := DeriveKey("secret-two")
	encrypted, _ := Encrypt(key1, "my-token")
	_, err := Decrypt(key2, encrypted)
	if err == nil {
		t.Error("decryption with wrong key should fail")
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"abcdefghij", "abcd******"},
		{"abc", "***"},
		{"", ""},
	}
	for _, tc := range tests {
		got := MaskToken(tc.input)
		if got != tc.expected {
			t.Errorf("MaskToken(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestGitHubCreateIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/sendrec/sendrec/issues" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer ghp_test" {
			t.Errorf("unexpected auth: %s", r.Header.Get("Authorization"))
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["title"] != "My Video" {
			t.Errorf("expected title 'My Video', got %v", body["title"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"html_url": "https://github.com/sendrec/sendrec/issues/42", "number": 42})
	}))
	defer server.Close()

	client := NewGitHubClient("ghp_test", "sendrec", "sendrec")
	client.baseURL = server.URL

	resp, err := client.CreateIssue(context.Background(), CreateIssueRequest{
		Title: "My Video", Description: "Video: https://app.sendrec.eu/watch/abc", VideoURL: "https://app.sendrec.eu/watch/abc",
	})
	if err != nil {
		t.Fatalf("CreateIssue failed: %v", err)
	}
	if resp.IssueURL != "https://github.com/sendrec/sendrec/issues/42" {
		t.Errorf("unexpected URL: %s", resp.IssueURL)
	}
	if resp.IssueKey != "#42" {
		t.Errorf("unexpected key: %s", resp.IssueKey)
	}
}

func TestGitHubValidateConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			json.NewEncoder(w).Encode(map[string]any{"login": "testuser"})
		case "/repos/sendrec/sendrec":
			json.NewEncoder(w).Encode(map[string]any{"full_name": "sendrec/sendrec"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewGitHubClient("ghp_test", "sendrec", "sendrec")
	client.baseURL = server.URL
	if err := client.ValidateConfig(context.Background()); err != nil {
		t.Errorf("ValidateConfig should succeed: %v", err)
	}
}

func TestGitHubValidateConfigBadToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewGitHubClient("bad-token", "sendrec", "sendrec")
	client.baseURL = server.URL
	if err := client.ValidateConfig(context.Background()); err == nil {
		t.Error("ValidateConfig should fail with bad token")
	}
}
