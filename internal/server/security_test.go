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

// BRANDING_DEFAULT_LOGO_URL can point at another host, and the watch, embed and
// playlist pages render it in an <img>. Without its origin in img-src the
// browser blocks it and every viewer page shows a broken logo.
func TestSecurityHeaders_AllowsBrandingLogoOrigin(t *testing.T) {
	csp := func(logoURL string) string {
		handler := securityHeaders(SecurityConfig{
			BaseURL:         "https://app.example.com",
			StorageEndpoint: "https://storage.example.com",
			BrandingLogoURL: logoURL,
		})
		rec := httptest.NewRecorder()
		handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).
			ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/watch/abc123", nil))
		return rec.Header().Get("Content-Security-Policy")
	}
	directive := func(policy, name string) string {
		for _, part := range strings.Split(policy, ";") {
			if strings.HasPrefix(strings.TrimSpace(part), name+" ") {
				return strings.TrimSpace(part)
			}
		}
		return ""
	}

	remote := csp("https://cdn.acme.example/brand/logo.png")
	if got := directive(remote, "img-src"); !strings.Contains(got, "https://cdn.acme.example") {
		t.Errorf("img-src should allow the logo's origin, got %q", got)
	}
	// The origin only: a whole URL in a source list would allow nothing else
	// from that host, and paths do not belong in CSP sources anyway.
	if strings.Contains(remote, "/brand/logo.png") {
		t.Errorf("CSP should carry the origin, not the full URL: %s", remote)
	}
	// Images only. The logo host has no business serving media or scripts.
	for _, name := range []string{"media-src", "script-src", "connect-src"} {
		if got := directive(remote, name); strings.Contains(got, "cdn.acme.example") {
			t.Errorf("%s should not allow the logo host, got %q", name, got)
		}
	}

	// A path on this host is already covered by 'self'. Compare img-src alone:
	// every response carries a fresh script nonce, so whole headers never match.
	if local, none := directive(csp("/images/acme.png"), "img-src"), directive(csp(""), "img-src"); local != none {
		t.Errorf("a same-host logo path should leave img-src unchanged:\n got %q\nwant %q", local, none)
	}
}
