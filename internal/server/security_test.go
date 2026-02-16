package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sendrec/sendrec/internal/httputil"
)

func TestSecurityHeaders_CSPContainsNonce(t *testing.T) {
	handler := securityHeaders(SecurityConfig{BaseURL: "https://app.test"})
	var capturedNonce string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedNonce = httputil.NonceFromContext(r.Context())
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(inner).ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "'nonce-"+capturedNonce+"'") {
		t.Errorf("CSP should contain nonce, got: %s", csp)
	}
	if capturedNonce == "" {
		t.Error("expected non-empty nonce in context")
	}
}

func TestSecurityHeaders_CSPOmitsUnsafeInline(t *testing.T) {
	handler := securityHeaders(SecurityConfig{BaseURL: "https://app.test"})
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(inner).ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if strings.Contains(csp, "'unsafe-inline'") {
		t.Errorf("CSP should not contain 'unsafe-inline', got: %s", csp)
	}
}

func TestSecurityHeaders_CSPIncludesStorageEndpoint(t *testing.T) {
	handler := securityHeaders(SecurityConfig{
		BaseURL:         "https://app.test",
		StorageEndpoint: "https://storage.example.com",
	})
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(inner).ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "connect-src 'self' https://storage.example.com") {
		t.Errorf("CSP connect-src should include storage endpoint, got: %s", csp)
	}
	if !strings.Contains(csp, "media-src 'self' data: https://storage.example.com") {
		t.Errorf("CSP media-src should include storage endpoint, got: %s", csp)
	}
	if !strings.Contains(csp, "img-src 'self' data: https://storage.example.com") {
		t.Errorf("CSP img-src should include storage endpoint, got: %s", csp)
	}
}

func TestSecurityHeaders_CSPOmitsStorageWhenEmpty(t *testing.T) {
	handler := securityHeaders(SecurityConfig{BaseURL: "https://app.test"})
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(inner).ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "connect-src 'self';") || strings.Contains(csp, "connect-src 'self' https://") {
		t.Errorf("CSP connect-src should be just 'self' when no storage endpoint, got: %s", csp)
	}
}

func TestSecurityHeaders_NonceInContext(t *testing.T) {
	handler := securityHeaders(SecurityConfig{BaseURL: "https://app.test"})
	var nonce string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce = httputil.NonceFromContext(r.Context())
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(inner).ServeHTTP(rec, req)

	if nonce == "" {
		t.Error("expected non-empty nonce in request context")
	}
}

func TestSecurityHeaders_UniqueNoncePerRequest(t *testing.T) {
	handler := securityHeaders(SecurityConfig{BaseURL: "https://app.test"})
	var nonces []string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonces = append(nonces, httputil.NonceFromContext(r.Context()))
	})

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		handler(inner).ServeHTTP(rec, req)
	}

	if nonces[0] == nonces[1] || nonces[1] == nonces[2] {
		t.Errorf("expected unique nonces per request, got %v", nonces)
	}
}

func TestSecurityHeaders_PermissionsPolicyAllowsMicrophone(t *testing.T) {
	handler := securityHeaders(SecurityConfig{BaseURL: "https://app.test"})
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(inner).ServeHTTP(rec, req)

	pp := rec.Header().Get("Permissions-Policy")
	if !strings.Contains(pp, "microphone=(self)") {
		t.Errorf("Permissions-Policy should allow microphone=(self), got: %s", pp)
	}
	if !strings.Contains(pp, "camera=(self)") {
		t.Errorf("Permissions-Policy should allow camera=(self), got: %s", pp)
	}
}

func TestSecurityHeaders_HSTSOnHTTPS(t *testing.T) {
	handler := securityHeaders(SecurityConfig{BaseURL: "https://app.test"})
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(inner).ServeHTTP(rec, req)

	hsts := rec.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("expected HSTS header for HTTPS base URL")
	}
}

func TestSecurityHeaders_NoHSTSOnHTTP(t *testing.T) {
	handler := securityHeaders(SecurityConfig{BaseURL: "http://localhost:8080"})
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(inner).ServeHTTP(rec, req)

	hsts := rec.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Errorf("expected no HSTS for HTTP base URL, got: %s", hsts)
	}
}

func TestSecurityHeaders_FrameAncestorsDefault(t *testing.T) {
	handler := securityHeaders(SecurityConfig{BaseURL: "https://app.test"})
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(inner).ServeHTTP(rec, req)
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors 'self'") {
		t.Errorf("CSP should contain frame-ancestors 'self', got: %s", csp)
	}
}

func TestSecurityHeaders_FrameAncestorsCustom(t *testing.T) {
	handler := securityHeaders(SecurityConfig{
		BaseURL:               "https://app.test",
		AllowedFrameAncestors: "https://nextcloud.example.com https://other.example.com",
	})
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(inner).ServeHTTP(rec, req)
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors 'self' https://nextcloud.example.com https://other.example.com") {
		t.Errorf("CSP should contain custom frame-ancestors, got: %s", csp)
	}
}
