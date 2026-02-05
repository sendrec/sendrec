package server

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
)

type Server struct {
	router      chi.Router
	db          *database.DB
	authHandler *auth.Handler
}

func New(db *database.DB) *Server {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	s := &Server{router: r, db: db}

	if db != nil {
		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = "dev-secret-change-in-production"
		}
		s.authHandler = auth.NewHandler(db.Pool, jwtSecret)
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
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
