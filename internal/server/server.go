package server

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/ratelimit"
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
			log.Println("WARNING: using default JWT secret, set JWT_SECRET in production")
		}

		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}

		secureCookies := strings.HasPrefix(baseURL, "https://")
		s.authHandler = auth.NewHandler(db.Pool, jwtSecret, secureCookies)
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
		authLimiter := ratelimit.NewLimiter(0.5, 5)
		s.router.Route("/api/auth", func(r chi.Router) {
			r.Use(authLimiter.Middleware)
			r.Post("/register", s.authHandler.Register)
			r.Post("/login", s.authHandler.Login)
			r.Post("/refresh", s.authHandler.Refresh)
			r.Post("/logout", s.authHandler.Logout)
		})
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
	if s.db != nil {
		if err := s.db.Pool.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"unhealthy","error":"database unreachable"}`))
			return
		}
	}
	w.Write([]byte(`{"status":"ok"}`))
}
