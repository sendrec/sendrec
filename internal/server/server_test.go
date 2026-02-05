package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sendrec/sendrec/internal/server"
)

func TestHealthEndpoint(t *testing.T) {
	srv := server.New()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	expected := `{"status":"ok"}`
	if rec.Body.String() != expected {
		t.Errorf("expected body %q, got %q", expected, rec.Body.String())
	}
}
