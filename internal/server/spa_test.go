package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sendrec/sendrec/internal/httputil"
)

// The head the built index.html ships with: a title, two favicons and a touch
// icon, all SendRec's.
const brandedTestIndex = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <link rel="icon" type="image/png" sizes="32x32" href="/favicon-32x32.png">
    <link rel="icon" type="image/png" sizes="16x16" href="/favicon-16x16.png">
    <link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon.png">
    <title>SendRec</title>
</head>
<body><div id="root"></div></body>
</html>`

func serveSPA(t *testing.T, s *spaFileServer, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d", path, rec.Code)
	}
	return rec.Body.String()
}

// A self-hosted install that sets BRANDING_DEFAULT_NAME and _LOGO_URL gets its
// own name in the tab title and its own logo as the favicon, on every page of
// the app, including the deep links the SPA serves index.html for. #267.
func TestSPA_AppliesInstanceBranding(t *testing.T) {
	fsys := fstest.MapFS{"index.html": {Data: []byte(brandedTestIndex)}}
	s := newSPAFileServer(fsys, "", "Acme Video", "https://cdn.acme.example/logo.png", "")

	for _, path := range []string{"/", "/library", "/videos/abc"} {
		body := serveSPA(t, s, path)
		if !strings.Contains(body, "<title>Acme Video</title>") {
			t.Errorf("%s: want the instance name as the title, got:\n%s", path, body)
		}
		if strings.Contains(body, "favicon-32x32.png") || strings.Contains(body, "apple-touch-icon.png") {
			t.Errorf("%s: SendRec's own icons should be replaced, got:\n%s", path, body)
		}
		if !strings.Contains(body, `<link rel="icon" href="https://cdn.acme.example/logo.png">`) {
			t.Errorf("%s: want the instance logo as the favicon, got:\n%s", path, body)
		}
		// The frontend reads these to brand the navigation and sign-in pages.
		if !strings.Contains(body, `<meta name="sendrec:brand-name" content="Acme Video">`) ||
			!strings.Contains(body, `<meta name="sendrec:brand-logo" content="https://cdn.acme.example/logo.png">`) {
			t.Errorf("%s: want brand meta tags for the frontend, got:\n%s", path, body)
		}
	}
}

// The name is operator input going into HTML. Escape it.
func TestSPA_EscapesInstanceBranding(t *testing.T) {
	fsys := fstest.MapFS{"index.html": {Data: []byte(brandedTestIndex)}}
	s := newSPAFileServer(fsys, "", `Acme "Video" <script>`, "", "")

	body := serveSPA(t, s, "/")
	if strings.Contains(body, "<script>") {
		t.Errorf("the name must be escaped, got:\n%s", body)
	}
	if !strings.Contains(body, "<title>Acme &#34;Video&#34; &lt;script&gt;</title>") {
		t.Errorf("want the escaped name as the title, got:\n%s", body)
	}
}

// Name only: the title changes and SendRec's icons stay.
func TestSPA_NameWithoutLogoKeepsIcons(t *testing.T) {
	fsys := fstest.MapFS{"index.html": {Data: []byte(brandedTestIndex)}}
	body := serveSPA(t, newSPAFileServer(fsys, "", "Acme Video", "", ""), "/")

	if !strings.Contains(body, "<title>Acme Video</title>") || !strings.Contains(body, "favicon-32x32.png") {
		t.Errorf("want the new title and SendRec's icons, got:\n%s", body)
	}
	if strings.Contains(body, "sendrec:brand-logo") {
		t.Errorf("no logo configured, so no logo meta tag, got:\n%s", body)
	}
}

// No branding configured: index.html goes out exactly as built.
func TestSPA_WithoutBrandingServesIndexUnchanged(t *testing.T) {
	fsys := fstest.MapFS{"index.html": {Data: []byte(brandedTestIndex)}}
	if body := serveSPA(t, newSPAFileServer(fsys, "", "", "", ""), "/"); body != brandedTestIndex {
		t.Errorf("want index.html unchanged, got:\n%s", body)
	}
}

func serveSPAWithNonce(t *testing.T, s *spaFileServer, nonce string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/library", nil)
	req = req.WithContext(httputil.ContextWithNonce(req.Context(), nonce))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec.Body.String()
}

// BRANDING_DEFAULT_COLOR_ACCENT already colours the viewer pages; it colours
// the dashboard too, in both themes. #275.
func TestSPA_AppliesInstanceAccent(t *testing.T) {
	fsys := fstest.MapFS{"index.html": {Data: []byte(brandedTestIndex)}}
	body := serveSPAWithNonce(t, newSPAFileServer(fsys, "", "", "", "#1a237e"), "n0nce")

	// The CSP only lets a <style> through with the request's nonce.
	if !strings.Contains(body, `<style nonce="n0nce">`) {
		t.Fatalf("want a nonced style element, got:\n%s", body)
	}
	for _, want := range []string{
		`:root,[data-theme="light"]{`,
		`--color-accent:#1a237e;`,
		`--color-accent-hover:color-mix(in srgb,#1a237e 85%,#000);`,
		`--color-accent-subtle:color-mix(in srgb,#1a237e 12%,transparent);`,
		`--color-drag-highlight:color-mix(in srgb,#1a237e 5%,transparent);`,
		`--color-on-accent:#ffffff;`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("want %q in the page, got:\n%s", want, body)
		}
	}
}

// Text on an accent button has to stay readable whatever colour is chosen.
func TestSPA_AccentTextContrasts(t *testing.T) {
	for accent, want := range map[string]string{
		"#1a237e": "#ffffff", // navy
		"#ffee00": "#111111", // yellow
		"#00b67a": "#111111", // SendRec's own green is light enough for dark text
		"#d32f2f": "#ffffff", // red
	} {
		if got := onAccentColor(accent); got != want {
			t.Errorf("onAccentColor(%s) = %s, want %s", accent, got, want)
		}
	}
}

// The value lands inside a <style>. Anything but a plain hex colour stays out.
func TestSPA_IgnoresInvalidAccent(t *testing.T) {
	fsys := fstest.MapFS{"index.html": {Data: []byte(brandedTestIndex)}}
	for _, accent := range []string{"red", "#fff", "#1a237e;}body{display:none", "</style><script>"} {
		if body := serveSPAWithNonce(t, newSPAFileServer(fsys, "", "", "", accent), "n"); body != brandedTestIndex {
			t.Errorf("accent %q: want index.html unchanged, got:\n%s", accent, body)
		}
	}
}
