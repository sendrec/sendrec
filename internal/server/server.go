package server

import (
	"io/fs"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/storage"
	"github.com/sendrec/sendrec/internal/video"
)

type Server struct {
	router       chi.Router
	db           *database.DB
	authHandler  *auth.Handler
	videoHandler *video.Handler
	webFS        fs.FS
}

func New(db *database.DB, store *storage.Storage, webFS fs.FS) *Server {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	s := &Server{router: r, db: db, webFS: webFS}

	if db != nil {
		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = "dev-secret-change-in-production"
		}
		s.authHandler = auth.NewHandler(db.Pool, jwtSecret)

		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}
		s.videoHandler = video.NewHandler(db.Pool, store, baseURL)
	}

	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.router.Get("/api/health", s.handleHealth)

	if s.authHandler != nil {
		s.router.Post("/api/auth/register", s.authHandler.Register)
		s.router.Post("/api/auth/login", s.authHandler.Login)
		s.router.Post("/api/auth/refresh", s.authHandler.Refresh)
	}

	if s.videoHandler != nil {
		s.router.Route("/api/videos", func(r chi.Router) {
			r.Use(s.authHandler.Middleware)
			r.Post("/", s.videoHandler.Create)
			r.Get("/", s.videoHandler.List)
			r.Patch("/{id}", s.videoHandler.Update)
			r.Delete("/{id}", s.videoHandler.Delete)
		})
		s.router.Get("/api/watch/{shareToken}", s.videoHandler.Watch)
		s.router.Get("/watch/{shareToken}", s.videoHandler.WatchPage)
	}

	if s.webFS != nil {
		spa := newSPAFileServer(s.webFS)
		s.router.NotFound(spa.ServeHTTP)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
