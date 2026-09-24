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
		fmt.Fprintf(&meta, `<style nonce="%s">`, html.EscapeString(nonce))
		for _, theme := range themes {
			sh := accentShades(s.brandAccent, theme)
			fmt.Fprintf(&meta, `%s{--color-accent:%s;--color-accent-hover:%s;--color-accent-subtle:%s;--color-drag-highlight:%s;--color-on-accent:%s;}`,
				theme.selector, sh.accent, sh.hover, rgba(sh.accent, 0.12), rgba(sh.accent, 0.05), sh.onAccent)
		}
		meta.WriteString("</style>\n")
	}
	if meta.Len() == 0 {
		return page
	}
	return strings.Replace(page, "</head>", meta.String()+"</head>", 1)
}

// The dashboard's two themes, as styles.css defines them: the accent has to
// read on each one's page background and card surface.
type theme struct {
	name, selector, background, surface string
}

var themes = []theme{
	{"dark", ":root", "#0a1628", "#111d32"},
	{"light", `[data-theme="light"]`, "#f8fafc", "#ffffff"},
}

type shades struct {
	accent, hover, onAccent string
}

// minContrast is WCAG AA for normal text. The accent is link text as well as
// button fill, so it is held to the text standard.
const minContrast = 4.5

// accentShades fits the operator's accent to a theme. The accent is moved
// toward white or black only as far as it takes to read on the theme's
// backgrounds, so a colour that already reads is used exactly as given. No
// colour reads on both a near-white and a navy page, so most accents are
// kept in one theme and shifted in the other. Button
// text is white or black, whichever contrasts more, and hover moves away from
// the text, so hovering can only make the label easier to read.
func accentShades(hex string, t theme) shades {
	c := parseHex(hex)
	towards := [3]float64{0, 0, 0}
	if luminance(parseHex(t.background)) < 0.5 {
		towards = [3]float64{255, 255, 255}
	}
	for range 40 {
		if contrastOf(c, parseHex(t.background)) >= minContrast && contrastOf(c, parseHex(t.surface)) >= minContrast {
			break
		}
		// Rounded every step: the hex that is emitted is what gets checked.
		c = parseHex(toHex(mix(c, towards, 0.05)))
	}

	onAccent, away := "#ffffff", [3]float64{0, 0, 0}
	if contrastOf(c, [3]float64{0, 0, 0}) > contrastOf(c, [3]float64{255, 255, 255}) {
		onAccent, away = "#000000", [3]float64{255, 255, 255}
	}
	return shades{accent: toHex(c), hover: toHex(mix(c, away, 0.15)), onAccent: onAccent}
}

func parseHex(hex string) [3]float64 {
	var c [3]float64
	for i := range c {
		v, _ := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
		c[i] = float64(v)
	}
	return c
}

func toHex(c [3]float64) string {
	return fmt.Sprintf("#%02x%02x%02x", int(math.Round(c[0])), int(math.Round(c[1])), int(math.Round(c[2])))
}

func rgba(hex string, alpha float64) string {
	c := parseHex(hex)
	return fmt.Sprintf("rgba(%d,%d,%d,%g)", int(c[0]), int(c[1]), int(c[2]), alpha)
}

func mix(c, towards [3]float64, amount float64) [3]float64 {
	for i := range c {
		c[i] += (towards[i] - c[i]) * amount
	}
	return c
}

// luminance is WCAG relative luminance.
func luminance(c [3]float64) float64 {
	var l [3]float64
	for i, v := range c {
		v /= 255
		if v <= 0.03928 {
			l[i] = v / 12.92
		} else {
			l[i] = math.Pow((v+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*l[0] + 0.7152*l[1] + 0.0722*l[2]
}

func contrastOf(a, b [3]float64) float64 {
	la, lb := luminance(a), luminance(b)
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05)
}

func contrast(a, b string) float64 {
	return contrastOf(parseHex(a), parseHex(b))
}
