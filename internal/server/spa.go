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
			http.NotFound(w, r)
			return
		}
		r.URL.Path = "/"
	}

	s.fileServer.ServeHTTP(w, r)
}
