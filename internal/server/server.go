package server

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/ratelimit"
	"github.com/sendrec/sendrec/internal/video"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Config struct {
	DB                      database.DBTX
	Pinger                  Pinger
	Storage                 video.ObjectStorage
	WebFS                   fs.FS
	JWTSecret               string
	BaseURL                 string
	MaxUploadBytes          int64
	MaxVideosPerMonth       int
	MaxVideoDurationSeconds int
	S3PublicEndpoint        string
	EmailSender             auth.EmailSender
	CommentNotifier         video.CommentNotifier
}

type Server struct {
	router       chi.Router
	pinger       Pinger
	authHandler  *auth.Handler
	videoHandler *video.Handler
	webFS        fs.FS
}

func New(cfg Config) *Server {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders(SecurityConfig{
		BaseURL:         cfg.BaseURL,
		StorageEndpoint: cfg.S3PublicEndpoint,
	}))

	s := &Server{router: r, pinger: cfg.Pinger, webFS: cfg.WebFS}

	if cfg.DB != nil {
		jwtSecret := cfg.JWTSecret
		if jwtSecret == "" {
			log.Fatal("JWT_SECRET is required; set the environment variable")
		}

		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}

		secureCookies := strings.HasPrefix(baseURL, "https://")
		s.authHandler = auth.NewHandler(cfg.DB, jwtSecret, secureCookies)
		if cfg.EmailSender != nil {
			s.authHandler.SetEmailSender(cfg.EmailSender, baseURL)
		}
		s.videoHandler = video.NewHandler(cfg.DB, cfg.Storage, baseURL, cfg.MaxUploadBytes, cfg.MaxVideosPerMonth, cfg.MaxVideoDurationSeconds, jwtSecret, secureCookies)
		if cfg.CommentNotifier != nil {
			s.videoHandler.SetCommentNotifier(cfg.CommentNotifier)
		}
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
			r.Post("/forgot-password", s.authHandler.ForgotPassword)
			r.Post("/reset-password", s.authHandler.ResetPassword)
		})
	}

	if s.authHandler != nil {
		s.router.Route("/api/user", func(r chi.Router) {
			r.Use(s.authHandler.Middleware)
			r.Get("/", s.authHandler.GetUser)
			r.Patch("/", s.authHandler.UpdateUser)
		})
	}

	if s.videoHandler != nil {
		videoLimiter := ratelimit.NewLimiter(2, 10)
		s.router.Route("/api/videos", func(r chi.Router) {
			r.Use(videoLimiter.Middleware)
			r.Use(s.authHandler.Middleware)
			r.Post("/", s.videoHandler.Create)
			r.Get("/limits", s.videoHandler.Limits)
			r.Get("/", s.videoHandler.List)
			r.Patch("/{id}", s.videoHandler.Update)
			r.Delete("/{id}", s.videoHandler.Delete)
			r.Post("/{id}/extend", s.videoHandler.Extend)
			r.Get("/{id}/download", s.videoHandler.Download)
			r.Post("/{id}/trim", s.videoHandler.Trim)
			r.Put("/{id}/password", s.videoHandler.SetPassword)
			r.Put("/{id}/comment-mode", s.videoHandler.SetCommentMode)
			r.Get("/{id}/comments", s.videoHandler.ListOwnerComments)
			r.Delete("/{id}/comments/{commentId}", s.videoHandler.DeleteComment)
		})
		watchAuthLimiter := ratelimit.NewLimiter(0.5, 5)
		commentLimiter := ratelimit.NewLimiter(0.2, 3)
		s.router.Get("/api/watch/{shareToken}", s.videoHandler.Watch)
		s.router.Get("/api/watch/{shareToken}/download", s.videoHandler.WatchDownload)
		s.router.With(watchAuthLimiter.Middleware).Post("/api/watch/{shareToken}/verify", s.videoHandler.VerifyWatchPassword)
		s.router.Get("/api/watch/{shareToken}/comments", s.videoHandler.ListWatchComments)
		s.router.With(commentLimiter.Middleware).Post("/api/watch/{shareToken}/comments", s.videoHandler.PostWatchComment)
		s.router.Get("/watch/{shareToken}", s.videoHandler.WatchPage)
	}

	if s.webFS != nil {
		spa := newSPAFileServer(s.webFS)
		s.router.NotFound(spa.ServeHTTP)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.pinger != nil {
		if err := s.pinger.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unhealthy","error":"database unreachable"}`))
			return
		}
	}
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
