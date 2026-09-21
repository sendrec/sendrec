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
	// The DPA and the sub-processors page say no CDN serves this. The policy is
	// what makes that true rather than aspirational: anything this page tries to
	// fetch from elsewhere is blocked before it leaves the browser.
	csp := rec.Header().Get("Content-Security-Policy")
	for _, host := range []string{"cdn.jsdelivr.net", "fonts.scalar.com", "proxy.scalar.com", "http"} {
		if strings.Contains(csp, host) {
			t.Errorf("CSP names an external host %q: %s", host, csp)
		}
	}
	if strings.Contains(body, "//") && strings.Contains(body, "https:") {
		t.Errorf("docs page references an external URL: %s", body)
	}
	if !strings.Contains(body, `src="/api/docs/scalar.js"`) {
		t.Error("docs page should load the bundle we serve ourselves")
	}
	// Scalar fetches its fonts and a CORS proxy unless both are turned off.
	if !strings.Contains(body, `"withDefaultFonts":false`) {
		t.Error("default fonts would be fetched from fonts.scalar.com")
	}
	if !strings.Contains(body, `"proxyUrl":""`) {
		t.Error("try-it requests would be routed through proxy.scalar.com")
	}
}

func TestHandleScalarBundle(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/docs/scalar.js", nil)
	rec := httptest.NewRecorder()

	HandleScalarBundle(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript", ct)
	}
	if rec.Body.Len() < 1_000_000 {
		t.Errorf("bundle is %d bytes, which is too small to be Scalar", rec.Body.Len())
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
		"/api/videos/{id}/transcript",
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
