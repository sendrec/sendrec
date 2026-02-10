package docs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleSpec(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/docs/openapi.yaml", nil)
	rec := httptest.NewRecorder()

	HandleSpec(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/yaml" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/yaml")
	}
	if !strings.HasPrefix(rec.Body.String(), "openapi:") {
		t.Error("body should start with 'openapi:'")
	}
}

func TestHandleDocs(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	rec := httptest.NewRecorder()

	HandleDocs(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "api-reference") {
		t.Error("body should contain 'api-reference'")
	}
	if !strings.Contains(body, "scalar") {
		t.Error("body should contain 'scalar'")
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "cdn.jsdelivr.net") {
		t.Errorf("CSP should allow cdn.jsdelivr.net, got %q", csp)
	}
}

func TestSpecContainsAllEndpoints(t *testing.T) {
	spec := string(specYAML)

	endpoints := []string{
		"/api/health",
		"/api/auth/register",
		"/api/auth/login",
		"/api/auth/refresh",
		"/api/auth/logout",
		"/api/auth/forgot-password",
		"/api/auth/reset-password",
		"/api/user",
		"/api/videos",
		"/api/videos/limits",
		"/api/videos/{id}",
		"/api/videos/{id}/extend",
		"/api/videos/{id}/download",
		"/api/videos/{id}/trim",
		"/api/videos/{id}/retranscribe",
		"/api/videos/{id}/password",
		"/api/videos/{id}/comment-mode",
		"/api/videos/{id}/comments",
		"/api/videos/{id}/comments/{commentId}",
		"/api/watch/{shareToken}",
		"/api/watch/{shareToken}/download",
		"/api/watch/{shareToken}/verify",
		"/api/watch/{shareToken}/comments",
	}

	for _, ep := range endpoints {
		if !strings.Contains(spec, ep) {
			t.Errorf("spec missing endpoint: %s", ep)
		}
	}
}
