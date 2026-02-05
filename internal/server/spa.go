package server

import (
	"io/fs"
	"net/http"
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
		r.URL.Path = "/"
	}

	s.fileServer.ServeHTTP(w, r)
}
