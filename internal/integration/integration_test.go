package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/auth"
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		fields, _ := body["fields"].(map[string]any)
		if fields["summary"] != "My Video" {
			t.Errorf("expected summary 'My Video', got %v", fields["summary"])
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-123", "self": server.URL + "/rest/api/3/issue/10001"})
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
			_ = json.NewEncoder(w).Encode(map[string]any{"accountId": "123"})
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["title"] != "My Video" {
			t.Errorf("expected title 'My Video', got %v", body["title"])
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"html_url": "https://github.com/sendrec/sendrec/issues/42", "number": 42})
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
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "testuser"})
		case "/repos/sendrec/sendrec":
			_ = json.NewEncoder(w).Encode(map[string]any{"full_name": "sendrec/sendrec"})
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

// --- Handler tests ---

func setupHandler(t *testing.T) (*Handler, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(mock, DeriveKey("test-secret"), "https://app.sendrec.eu")
	return h, mock
}

func withUserCtx(r *http.Request, userID string) *http.Request {
	return r.WithContext(auth.ContextWithUserID(r.Context(), userID))
}

func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestListIntegrations(t *testing.T) {
	h, mock := setupHandler(t)
	defer mock.Close()

	rows := pgxmock.NewRows([]string{"id", "provider", "config", "created_at", "updated_at"}).
		AddRow("int-1", "github", []byte(`{"token":"not-encrypted","owner":"org","repo":"repo"}`), time.Now(), time.Now())
	mock.ExpectQuery("SELECT id, provider, config, created_at, updated_at FROM user_integrations WHERE user_id").
		WithArgs("user-1").
		WillReturnRows(rows)

	req := httptest.NewRequest("GET", "/api/settings/integrations", nil)
	req = withUserCtx(req, "user-1")
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSaveIntegration(t *testing.T) {
	h, mock := setupHandler(t)
	defer mock.Close()

	mock.ExpectExec("INSERT INTO user_integrations").
		WithArgs("user-1", "github", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := `{"token":"ghp_test123","owner":"sendrec","repo":"sendrec"}`
	req := httptest.NewRequest("PUT", "/api/settings/integrations/github", bytes.NewBufferString(body))
	req = withUserCtx(req, "user-1")
	req = withChiParam(req, "provider", "github")
	w := httptest.NewRecorder()
	h.Save(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteIntegration(t *testing.T) {
	h, mock := setupHandler(t)
	defer mock.Close()

	mock.ExpectExec("DELETE FROM user_integrations WHERE user_id").
		WithArgs("user-1", "github").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	req := httptest.NewRequest("DELETE", "/api/settings/integrations/github", nil)
	req = withUserCtx(req, "user-1")
	req = withChiParam(req, "provider", "github")
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestSaveInvalidProvider(t *testing.T) {
	h, mock := setupHandler(t)
	defer mock.Close()

	req := httptest.NewRequest("PUT", "/api/settings/integrations/notion", bytes.NewBufferString(`{}`))
	req = withUserCtx(req, "user-1")
	req = withChiParam(req, "provider", "notion")
	w := httptest.NewRecorder()
	h.Save(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSaveMissingRequiredFields(t *testing.T) {
	h, mock := setupHandler(t)
	defer mock.Close()

	body := `{"token":"ghp_test"}`
	req := httptest.NewRequest("PUT", "/api/settings/integrations/github", bytes.NewBufferString(body))
	req = withUserCtx(req, "user-1")
	req = withChiParam(req, "provider", "github")
	w := httptest.NewRecorder()
	h.Save(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", w.Code)
	}
}
