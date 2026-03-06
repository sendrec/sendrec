package scim

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func newTestHandler(t *testing.T) (*Handler, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(mock, "http://localhost:8080"), mock
}

func withOrgID(r *http.Request, orgID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgId", orgID)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestServiceProviderConfig(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = withOrgID(req, "org-1")
	rec := httptest.NewRecorder()

	handler.ServiceProviderConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/scim+json" {
		t.Errorf("Content-Type = %q, want application/scim+json", ct)
	}

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)
	schemas := body["schemas"].([]interface{})
	if schemas[0] != SPConfigSchema {
		t.Errorf("schema = %q, want %q", schemas[0], SPConfigSchema)
	}
}

func TestSchemas(t *testing.T) {
	handler, mock := newTestHandler(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = withOrgID(req, "org-1")
	rec := httptest.NewRecorder()

	handler.Schemas(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
}
