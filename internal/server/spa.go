package server

import (
	"html"
	"io/fs"
	"net/http"
	stdpath "path"
	"regexp"
	"strings"
)

type spaFileServer struct {
	fileServer      http.Handler
	fileSystem      fs.FS
	analyticsScript string
	// BRANDING_DEFAULT_NAME and _LOGO_URL, validated at startup. #267.
	brandName    string
	brandLogoURL string
}

func newSPAFileServer(fsys fs.FS, analyticsScript, brandName, brandLogoURL string) *spaFileServer {
	return &spaFileServer{
		fileServer:      http.FileServer(http.FS(fsys)),
		fileSystem:      fsys,
		analyticsScript: analyticsScript,
		brandName:       brandName,
		brandLogoURL:    brandLogoURL,
	}
}

func (s *spaFileServer) rewritesIndex() bool {
	return s.analyticsScript != "" || s.brandName != "" || s.brandLogoURL != ""
}

func (s *spaFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	_, err := fs.Stat(s.fileSystem, path)
	if err != nil {
		if stdpath.Ext(r.URL.Path) != "" {
			s.serveIndexWithStatus(w, http.StatusNotFound)
			return
		}
		r.URL.Path = "/"
	}

	if r.URL.Path == "/" && s.rewritesIndex() {
		s.serveIndexWithStatus(w, http.StatusOK)
		return
	}

	s.fileServer.ServeHTTP(w, r)
}

func (s *spaFileServer) serveIndexWithStatus(w http.ResponseWriter, status int) {
	data, err := fs.ReadFile(s.fileSystem, "index.html")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	data = []byte(s.brandIndex(string(data)))
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
func (s *spaFileServer) brandIndex(page string) string {
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
	if meta.Len() == 0 {
		return page
	}
	return strings.Replace(page, "</head>", meta.String()+"</head>", 1)
}
