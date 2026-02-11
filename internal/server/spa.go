package server

import (
	"io/fs"
	"net/http"
	stdpath "path"
	"strings"
)

type spaFileServer struct {
	fileServer http.Handler
	fileSystem fs.FS
}

func newSPAFileServer(fsys fs.FS) *spaFileServer {
	return &spaFileServer{
		fileServer: http.FileServer(http.FS(fsys)),
		fileSystem: fsys,
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

	s.fileServer.ServeHTTP(w, r)
}

func (s *spaFileServer) serveIndexWithStatus(w http.ResponseWriter, status int) {
	data, err := fs.ReadFile(s.fileSystem, "index.html")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
