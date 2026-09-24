package server

import (
	"fmt"
	"html"
	"io/fs"
	"math"
	"net/http"
	stdpath "path"
	"regexp"
	"strconv"
	"strings"

	"github.com/sendrec/sendrec/internal/httputil"
)

type spaFileServer struct {
	fileServer      http.Handler
	fileSystem      fs.FS
	analyticsScript string
	// BRANDING_DEFAULT_NAME and _LOGO_URL, validated at startup. #267.
	brandName    string
	brandLogoURL string
	// BRANDING_DEFAULT_COLOR_ACCENT, kept only if it is a plain hex colour: it
	// goes inside a <style>. #275.
	brandAccent string
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func newSPAFileServer(fsys fs.FS, analyticsScript, brandName, brandLogoURL, brandAccent string) *spaFileServer {
	if !hexColor.MatchString(brandAccent) {
		brandAccent = ""
	}
	return &spaFileServer{
		fileServer:      http.FileServer(http.FS(fsys)),
		fileSystem:      fsys,
		analyticsScript: analyticsScript,
		brandName:       brandName,
		brandLogoURL:    brandLogoURL,
		brandAccent:     brandAccent,
	}
}

func (s *spaFileServer) rewritesIndex() bool {
	return s.analyticsScript != "" || s.brandName != "" || s.brandLogoURL != "" || s.brandAccent != ""
}

func (s *spaFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	_, err := fs.Stat(s.fileSystem, path)
	if err != nil {
		if stdpath.Ext(r.URL.Path) != "" {
			s.serveIndexWithStatus(w, r, http.StatusNotFound)
			return
		}
		r.URL.Path = "/"
	}

	if r.URL.Path == "/" && s.rewritesIndex() {
		s.serveIndexWithStatus(w, r, http.StatusOK)
		return
	}

	s.fileServer.ServeHTTP(w, r)
}

func (s *spaFileServer) serveIndexWithStatus(w http.ResponseWriter, r *http.Request, status int) {
	data, err := fs.ReadFile(s.fileSystem, "index.html")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	data = []byte(s.brandIndex(string(data), httputil.NonceFromContext(r.Context())))
	if s.analyticsScript != "" {
		data = []byte(strings.Replace(string(data), "</head>", s.analyticsScript+"\n</head>", 1))
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

var (
	titleTag = regexp.MustCompile(`<title>[^<]*</title>`)
	iconTags = regexp.MustCompile(`(?m)^[ \t]*<link rel="(?:icon|apple-touch-icon)"[^>]*>\r?\n?`)
)

// brandIndex puts the instance's name in the tab title and its logo in place of
// SendRec's icons. Both load before any script runs, so they cannot wait for
// the frontend. The frontend reads the meta tags to brand the navigation and
// the sign-in pages.
func (s *spaFileServer) brandIndex(page, nonce string) string {
	var meta strings.Builder
	if s.brandName != "" {
		name := html.EscapeString(s.brandName)
		page = titleTag.ReplaceAllLiteralString(page, "<title>"+name+"</title>")
		meta.WriteString(`<meta name="sendrec:brand-name" content="` + name + `">` + "\n")
	}
	if s.brandLogoURL != "" {
		logo := html.EscapeString(s.brandLogoURL)
		page = iconTags.ReplaceAllLiteralString(page, "")
		meta.WriteString(`<link rel="icon" href="` + logo + `">` + "\n")
		meta.WriteString(`<link rel="apple-touch-icon" href="` + logo + `">` + "\n")
		meta.WriteString(`<meta name="sendrec:brand-logo" content="` + logo + `">` + "\n")
	}
	if s.brandAccent != "" {
		// Both themes take the one colour; the shades are mixed from it, and
		// button text goes whichever way reads better on it.
		a := s.brandAccent
		fmt.Fprintf(&meta, `<style nonce="%s">:root,[data-theme="light"]{--color-accent:%s;--color-accent-hover:color-mix(in srgb,%s 85%%,#000);--color-accent-subtle:color-mix(in srgb,%s 12%%,transparent);--color-drag-highlight:color-mix(in srgb,%s 5%%,transparent);--color-on-accent:%s;}</style>`+"\n",
			html.EscapeString(nonce), a, a, a, a, onAccentColor(a))
	}
	if meta.Len() == 0 {
		return page
	}
	return strings.Replace(page, "</head>", meta.String()+"</head>", 1)
}

// onAccentColor picks white or near-black text for a hex background, whichever
// contrasts more (WCAG relative luminance).
func onAccentColor(hex string) string {
	channel := func(i int) float64 {
		v, _ := strconv.ParseUint(hex[i:i+2], 16, 8)
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	l := 0.2126*channel(1) + 0.7152*channel(3) + 0.0722*channel(5)
	const dark = 0.0056 // #111111
	if (1.05)/(l+0.05) >= (l+0.05)/(dark+0.05) {
		return "#ffffff"
	}
	return "#111111"
}
