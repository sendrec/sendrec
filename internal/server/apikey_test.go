package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/auth"
)

func TestAPIKeyMiddleware_ValidKey(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	defer mock.Close()

	key := "sr_abababababababababababababababababababababababababababababababababab"
	expectedHash := auth.HashAPIKey(key)
	userID := "550e8400-e29b-41d4-a716-446655440000"

	mock.ExpectQuery(`SELECT user_id FROM api_keys`).
		WithArgs(expectedHash).
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow(userID))

	mock.ExpectExec(`UPDATE api_keys SET last_used_at`).
		WithArgs(expectedHash).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mw := apiKeyOrJWTMiddleware(mock, auth.NewHandler(nil, "secret", false).Middleware)

	var capturedUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = auth.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if capturedUserID != userID {
		t.Errorf("expected user ID %q, got %q", userID, capturedUserID)
	}
}

func TestAPIKeyMiddleware_InvalidKeyFallsBackToJWT(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	defer mock.Close()

	key := "sr_cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd"
	expectedHash := auth.HashAPIKey(key)

	mock.ExpectQuery(`SELECT user_id FROM api_keys`).
		WithArgs(expectedHash).
		WillReturnError(pgx.ErrNoRows)

	jwtSecret := "jwt-test-secret"
	mw := apiKeyOrJWTMiddleware(mock, auth.NewHandler(nil, jwtSecret, false).Middleware)

	var capturedUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = auth.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	userID := "550e8400-e29b-41d4-a716-446655440000"
	token, err := auth.GenerateAccessToken(jwtSecret, userID)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if capturedUserID != userID {
		t.Errorf("expected user ID %q, got %q", userID, capturedUserID)
	}
}

func TestAPIKeyMiddleware_NoAuthFallsToJWT(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mw := apiKeyOrJWTMiddleware(mock, auth.NewHandler(nil, "secret", false).Middleware)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 when no auth header, got %d", rec.Code)
	}
}
