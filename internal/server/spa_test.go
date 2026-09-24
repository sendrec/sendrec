package server

import (
	"fmt"
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
// the dashboard too, each theme with its own readable shade of it. #275.
func TestSPA_AppliesInstanceAccent(t *testing.T) {
	fsys := fstest.MapFS{"index.html": {Data: []byte(brandedTestIndex)}}
	body := serveSPAWithNonce(t, newSPAFileServer(fsys, "", "", "", "#7c3aed"), "n0nce")

	// The CSP only lets a <style> through with the request's nonce.
	if !strings.Contains(body, `<style nonce="n0nce">`) {
		t.Fatalf("want a nonced style element, got:\n%s", body)
	}
	for _, want := range []string{
		// Purple reads on the light page as given; the dark theme gets its own
		// lighter shade, checked below.
		`:root{--color-accent:#`,
		`[data-theme="light"]{--color-accent:#7c3aed;`,
		`--color-accent-subtle:rgba(124,58,237,0.12);`,
		`--color-drag-highlight:rgba(124,58,237,0.05);`,
		`--color-on-accent:#ffffff;`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("want %q in the page, got:\n%s", want, body)
		}
	}
}

// The accent is link text, focus rings and button fill, on two backgrounds.
// Whatever colour an operator picks, every one of those has to stay readable
// (WCAG AA, 4.5:1), hover included. Swept across the colour cube.
func TestAccentShades_StayReadable(t *testing.T) {
	for r := 0; r < 256; r += 17 {
		for g := 0; g < 256; g += 17 {
			for b := 0; b < 256; b += 17 {
				accent := fmt.Sprintf("#%02x%02x%02x", r, g, b)
				for _, theme := range themes {
					sh := accentShades(accent, theme)
					for _, pair := range []struct{ what, fg, bg string }{
						{"accent on background", sh.accent, theme.background},
						{"accent on surface", sh.accent, theme.surface},
						{"button text", sh.onAccent, sh.accent},
						{"button text on hover", sh.onAccent, sh.hover},
					} {
						if c := contrast(pair.fg, pair.bg); c < 4.5 {
							t.Fatalf("%s, %s theme: %s %s on %s is %.2f:1", accent, theme.name, pair.what, pair.fg, pair.bg, c)
						}
					}
				}
			}
		}
	}
}

// Only as much change as readability needs: a colour that already reads is
// used exactly as given.
func TestAccentShades_KeepAReadableColour(t *testing.T) {
	if got := accentShades("#7c3aed", themes[1]).accent; got != "#7c3aed" {
		t.Errorf("light theme changed a readable purple to %s", got)
	}
	if got := accentShades("#ffee00", themes[0]).accent; got != "#ffee00" {
		t.Errorf("yellow reads on the dark theme, got %s", got)
	}
	if got := accentShades("#ffee00", themes[1]).accent; got == "#ffee00" {
		t.Error("yellow cannot be link text on the light theme")
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
