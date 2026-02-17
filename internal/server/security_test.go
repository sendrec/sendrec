package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders_EmbedPath_PermissiveFrameAncestors(t *testing.T) {
	handler := securityHeaders(SecurityConfig{
		BaseURL:               "https://app.sendrec.eu",
		StorageEndpoint:       "https://storage.sendrec.eu",
		AllowedFrameAncestors: "example.com",
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/embed/abc123", nil)
	rec := httptest.NewRecorder()
	handler(inner).ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors *") {
		t.Errorf("expected 'frame-ancestors *' for embed path, got CSP: %s", csp)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/watch/abc123", nil)
	rec2 := httptest.NewRecorder()
	handler(inner).ServeHTTP(rec2, req2)

	csp2 := rec2.Header().Get("Content-Security-Policy")
	if strings.Contains(csp2, "frame-ancestors *") {
		t.Errorf("non-embed path should NOT have 'frame-ancestors *', got CSP: %s", csp2)
	}
	if !strings.Contains(csp2, "frame-ancestors 'self' example.com") {
		t.Errorf("expected configured frame-ancestors for watch path, got CSP: %s", csp2)
	}
}
