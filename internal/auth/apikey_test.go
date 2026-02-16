package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestGenerateAPIKey(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	defer mock.Close()

	userID := "user-uuid-1"
	createdAt := time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM api_keys`).
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`INSERT INTO api_keys`).
		WithArgs(userID, pgxmock.AnyArg(), "My Nextcloud").
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow("key-uuid-1", createdAt))

	handler := GenerateAPIKey(mock)

	body := `{"name":"My Nextcloud"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/api-keys", strings.NewReader(body))
	req = req.WithContext(ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp generateAPIKeyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.ID != "key-uuid-1" {
		t.Errorf("expected id %q, got %q", "key-uuid-1", resp.ID)
	}
	if resp.Name != "My Nextcloud" {
		t.Errorf("expected name %q, got %q", "My Nextcloud", resp.Name)
	}
	if !strings.HasPrefix(resp.Key, "sr_") {
		t.Errorf("expected key to start with sr_, got %q", resp.Key)
	}
	if len(resp.Key) != 67 {
		t.Errorf("expected key length 67, got %d", len(resp.Key))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestGenerateAPIKey_NameTooLong(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	defer mock.Close()

	handler := GenerateAPIKey(mock)

	longName := strings.Repeat("a", 101)
	body := `{"name":"` + longName + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/api-keys", strings.NewReader(body))
	req = req.WithContext(ContextWithUserID(req.Context(), "user-uuid-1"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestGenerateAPIKey_Limit(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	defer mock.Close()

	userID := "user-uuid-1"

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM api_keys`).
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(10))

	handler := GenerateAPIKey(mock)

	body := `{"name":"eleventh key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/api-keys", strings.NewReader(body))
	req = req.WithContext(ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(errResp.Error, "maximum") {
		t.Errorf("expected error about maximum keys, got %q", errResp.Error)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestListAPIKeys(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	defer mock.Close()

	userID := "user-uuid-1"
	createdAt := time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC)
	lastUsedAt := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT id, name, created_at, last_used_at FROM api_keys`).
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "created_at", "last_used_at"}).
			AddRow("key-1", "My Nextcloud", createdAt, &lastUsedAt).
			AddRow("key-2", "Other Key", createdAt, (*time.Time)(nil)),
		)

	handler := ListAPIKeys(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/api-keys", nil)
	req = req.WithContext(ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var items []apiKeyItem
	if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].ID != "key-1" {
		t.Errorf("expected first key id %q, got %q", "key-1", items[0].ID)
	}
	if items[0].LastUsedAt == nil {
		t.Error("expected first key to have lastUsedAt")
	}
	if items[1].LastUsedAt != nil {
		t.Error("expected second key to have nil lastUsedAt")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestDeleteAPIKey(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	defer mock.Close()

	userID := "user-uuid-1"
	keyID := "key-uuid-1"

	mock.ExpectExec(`DELETE FROM api_keys`).
		WithArgs(keyID, userID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	handler := DeleteAPIKey(mock)

	r := chi.NewRouter()
	r.Delete("/api/settings/api-keys/{id}", handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/api-keys/"+keyID, nil)
	req = req.WithContext(ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestDeleteAPIKey_NotOwner(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	defer mock.Close()

	userID := "user-uuid-1"
	keyID := "key-uuid-other"

	mock.ExpectExec(`DELETE FROM api_keys`).
		WithArgs(keyID, userID).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	handler := DeleteAPIKey(mock)

	r := chi.NewRouter()
	r.Delete("/api/settings/api-keys/{id}", handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/api-keys/"+keyID, nil)
	req = req.WithContext(ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestLookupAPIKey_Valid(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	defer mock.Close()

	key := "sr_" + strings.Repeat("ab", 32)
	expectedHash := HashAPIKey(key)
	userID := "user-uuid-1"

	mock.ExpectQuery(`SELECT user_id FROM api_keys`).
		WithArgs(expectedHash).
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow(userID))

	// Expect the async last_used_at update
	mock.ExpectExec(`UPDATE api_keys SET last_used_at`).
		WithArgs(expectedHash).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	result, err := LookupAPIKey(context.Background(), mock, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != userID {
		t.Errorf("expected user ID %q, got %q", userID, result)
	}

	// Give goroutine time to execute
	time.Sleep(50 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestLookupAPIKey_Invalid(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	defer mock.Close()

	key := "sr_" + strings.Repeat("cd", 32)
	expectedHash := HashAPIKey(key)

	mock.ExpectQuery(`SELECT user_id FROM api_keys`).
		WithArgs(expectedHash).
		WillReturnError(pgx.ErrNoRows)

	_, err = LookupAPIKey(context.Background(), mock, key)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestLookupAPIKey_NotAPIKeyFormat(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	defer mock.Close()

	_, err = LookupAPIKey(context.Background(), mock, "not-an-api-key")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
