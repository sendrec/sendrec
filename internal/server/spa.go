package server

import (
	"io/fs"
	"net/http"
	stdpath "path"
	"strings"
)

type spaFileServer struct {
	fileServer      http.Handler
	fileSystem      fs.FS
	analyticsScript string
}

func newSPAFileServer(fsys fs.FS, analyticsScript string) *spaFileServer {
	return &spaFileServer{
		fileServer:      http.FileServer(http.FS(fsys)),
		fileSystem:      fsys,
		analyticsScript: analyticsScript,
	}
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

	if r.URL.Path == "/" && s.analyticsScript != "" {
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
	if s.analyticsScript != "" {
		data = []byte(strings.Replace(string(data), "</head>", s.analyticsScript+"\n</head>", 1))
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
