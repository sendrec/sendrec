package scim

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestBearerAuth_ValidToken(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	token := "scim_test_token_123"
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	mock.ExpectQuery(`SELECT st\.token_hash, o\.subscription_plan FROM organization_scim_tokens st JOIN organizations o ON o\.id = st\.organization_id WHERE st\.organization_id = \$1`).
		WithArgs("org-1").
		WillReturnRows(pgxmock.NewRows([]string{"token_hash", "subscription_plan"}).AddRow(tokenHash, "business"))

	handler := BearerAuth(mock)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
}

func TestBearerAuth_MissingHeader(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := BearerAuth(mock)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want 401", rec.Code)
	}
}

func TestBearerAuth_InvalidToken(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`SELECT st\.token_hash, o\.subscription_plan FROM organization_scim_tokens st JOIN organizations o ON o\.id = st\.organization_id WHERE st\.organization_id = \$1`).
		WithArgs("org-1").
		WillReturnRows(pgxmock.NewRows([]string{"token_hash", "subscription_plan"}).AddRow("different_hash", "business"))

	handler := BearerAuth(mock)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong_token")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want 401", rec.Code)
	}
}

func TestBearerAuth_NoTokenConfigured(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`SELECT st\.token_hash, o\.subscription_plan FROM organization_scim_tokens st JOIN organizations o ON o\.id = st\.organization_id WHERE st\.organization_id = \$1`).
		WithArgs("org-1").
		WillReturnError(pgx.ErrNoRows)

	handler := BearerAuth(mock)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer some_token")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want 401", rec.Code)
	}
}

func TestBearerAuth_RequiresBusinessPlan(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	token := "scim_test_token_123"
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	mock.ExpectQuery(`SELECT st\.token_hash, o\.subscription_plan FROM organization_scim_tokens st JOIN organizations o ON o\.id = st\.organization_id WHERE st\.organization_id = \$1`).
		WithArgs("org-1").
		WillReturnRows(pgxmock.NewRows([]string{"token_hash", "subscription_plan"}).AddRow(tokenHash, "pro"))

	handler := BearerAuth(mock)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("got status %d, want 403", rec.Code)
	}
}
