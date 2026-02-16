package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sendrec/sendrec/internal/auth"
)

func TestAPIKeyMiddleware_ValidKey(t *testing.T) {
	apiKey := "test-api-key-12345"
	mw := apiKeyOrJWTMiddleware(apiKey, auth.NewHandler(nil, "secret", false).Middleware)

	var capturedUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = auth.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if capturedUserID != "__api_key__" {
		t.Errorf("expected user ID %q, got %q", "__api_key__", capturedUserID)
	}
}

func TestAPIKeyMiddleware_FallsBackToJWT(t *testing.T) {
	apiKey := "test-api-key-12345"
	jwtSecret := "jwt-test-secret"
	mw := apiKeyOrJWTMiddleware(apiKey, auth.NewHandler(nil, jwtSecret, false).Middleware)

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

func TestAPIKeyMiddleware_EmptyKeyDisablesAPIKeyAuth(t *testing.T) {
	mw := apiKeyOrJWTMiddleware("", auth.NewHandler(nil, "secret", false).Middleware)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	req.Header.Set("Authorization", "Bearer some-random-key")
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 when API key is empty and JWT is invalid, got %d", rec.Code)
	}
}
