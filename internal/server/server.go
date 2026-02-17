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
	"github.com/sendrec/sendrec/internal/docs"
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
	EnableDocs              bool
	BrandingEnabled         bool
	AllowedFrameAncestors   string
	AnalyticsScript         string
	EmailSender             auth.EmailSender
	CommentNotifier         video.CommentNotifier
	ViewNotifier            video.ViewNotifier
}

type Server struct {
	router       chi.Router
	pinger       Pinger
	authHandler  *auth.Handler
	videoHandler *video.Handler
	db           database.DBTX
	webFS        fs.FS
	enableDocs   bool
}

func New(cfg Config) *Server {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders(SecurityConfig{
		BaseURL:               cfg.BaseURL,
		StorageEndpoint:       cfg.S3PublicEndpoint,
		AllowedFrameAncestors: cfg.AllowedFrameAncestors,
	}))

	s := &Server{router: r, pinger: cfg.Pinger, db: cfg.DB, webFS: cfg.WebFS, enableDocs: cfg.EnableDocs}

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
		if cfg.ViewNotifier != nil {
			s.videoHandler.SetViewNotifier(cfg.ViewNotifier)
		}
		if cfg.BrandingEnabled {
			s.videoHandler.SetBrandingEnabled(true)
		}
		if cfg.AnalyticsScript != "" {
			s.videoHandler.SetAnalyticsScript(cfg.AnalyticsScript)
		}
	}

	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func apiKeyOrJWTMiddleware(db database.DBTX, jwtMiddleware func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			token, found := strings.CutPrefix(authHeader, "Bearer ")
			if found {
				userID, err := auth.LookupAPIKey(r.Context(), db, token)
				if err == nil {
					ctx := auth.ContextWithUserID(r.Context(), userID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			jwtMiddleware(next).ServeHTTP(w, r)
		})
	}
}

func maxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) routes() {
	s.router.Get("/api/health", s.handleHealth)
	if s.enableDocs {
		s.router.Get("/api/docs", docs.HandleDocs)
		s.router.Get("/api/docs/openapi.yaml", docs.HandleSpec)
	}

	if s.authHandler != nil {
		authLimiter := ratelimit.NewLimiter(0.5, 5)
		s.router.Route("/api/auth", func(r chi.Router) {
			r.Use(authLimiter.Middleware)
			r.Use(maxBodySize(64 * 1024))
			r.Post("/register", s.authHandler.Register)
			r.Post("/login", s.authHandler.Login)
			r.Post("/refresh", s.authHandler.Refresh)
			r.Post("/logout", s.authHandler.Logout)
			r.Post("/forgot-password", s.authHandler.ForgotPassword)
			r.Post("/reset-password", s.authHandler.ResetPassword)
			r.Post("/confirm-email", s.authHandler.ConfirmEmail)
			r.Post("/resend-confirmation", s.authHandler.ResendConfirmation)
		})
	}

	if s.authHandler != nil {
		s.router.Route("/api/user", func(r chi.Router) {
			r.Use(s.authHandler.Middleware)
			r.Use(maxBodySize(64 * 1024))
			r.Get("/", s.authHandler.GetUser)
			r.Patch("/", s.authHandler.UpdateUser)
		})
	}

	if s.videoHandler != nil {
		s.router.Route("/api/settings", func(r chi.Router) {
			r.Use(s.authHandler.Middleware)
			r.Use(maxBodySize(64 * 1024))
			r.Get("/notifications", s.videoHandler.GetNotificationPreferences)
			r.Put("/notifications", s.videoHandler.PutNotificationPreferences)
			r.Get("/branding", s.videoHandler.GetBrandingSettings)
			r.Put("/branding", s.videoHandler.PutBrandingSettings)
			r.Post("/branding/logo", s.videoHandler.UploadBrandingLogo)
			r.Delete("/branding/logo", s.videoHandler.DeleteBrandingLogo)
			r.Post("/api-keys", auth.GenerateAPIKey(s.db))
			r.Get("/api-keys", auth.ListAPIKeys(s.db))
			r.Delete("/api-keys/{id}", auth.DeleteAPIKey(s.db))
		})

		videoLimiter := ratelimit.NewLimiter(2, 10)
		s.router.Route("/api/videos", func(r chi.Router) {
			r.Use(videoLimiter.Middleware)
			r.Use(maxBodySize(64 * 1024))
			// List accepts API key OR JWT
			r.With(apiKeyOrJWTMiddleware(s.db, s.authHandler.Middleware)).Get("/", s.videoHandler.List)
			// All other endpoints require JWT
			r.Group(func(r chi.Router) {
				r.Use(s.authHandler.Middleware)
				r.Post("/", s.videoHandler.Create)
				r.Post("/upload", s.videoHandler.Upload)
				r.Get("/limits", s.videoHandler.Limits)
				r.Patch("/{id}", s.videoHandler.Update)
				r.Delete("/{id}", s.videoHandler.Delete)
				r.Post("/{id}/extend", s.videoHandler.Extend)
				r.Get("/{id}/download", s.videoHandler.Download)
				r.Post("/{id}/trim", s.videoHandler.Trim)
				r.Post("/{id}/retranscribe", s.videoHandler.Retranscribe)
				r.Put("/{id}/password", s.videoHandler.SetPassword)
				r.Put("/{id}/comment-mode", s.videoHandler.SetCommentMode)
				r.Get("/{id}/comments", s.videoHandler.ListOwnerComments)
				r.Delete("/{id}/comments/{commentId}", s.videoHandler.DeleteComment)
				r.Get("/{id}/analytics", s.videoHandler.Analytics)
				r.Put("/{id}/notifications", s.videoHandler.SetVideoNotification)
				r.Put("/{id}/download-enabled", s.videoHandler.SetDownloadEnabled)
				r.Put("/{id}/link-expiry", s.videoHandler.SetLinkExpiry)
				r.Get("/{id}/branding", s.videoHandler.GetVideoBranding)
				r.Put("/{id}/branding", s.videoHandler.SetVideoBranding)
				r.Post("/{id}/thumbnail", s.videoHandler.UploadThumbnail)
				r.Delete("/{id}/thumbnail", s.videoHandler.ResetThumbnail)
			})
		})
		watchAuthLimiter := ratelimit.NewLimiter(0.5, 5)
		commentLimiter := ratelimit.NewLimiter(0.2, 3)
		s.router.Get("/api/watch/{shareToken}", s.videoHandler.Watch)
		s.router.Get("/api/watch/{shareToken}/download", s.videoHandler.WatchDownload)
		s.router.With(watchAuthLimiter.Middleware, maxBodySize(64*1024)).Post("/api/watch/{shareToken}/verify", s.videoHandler.VerifyWatchPassword)
		s.router.Get("/api/watch/{shareToken}/comments", s.videoHandler.ListWatchComments)
		s.router.With(commentLimiter.Middleware, maxBodySize(64*1024)).Post("/api/watch/{shareToken}/comments", s.videoHandler.PostWatchComment)
		s.router.Get("/api/videos/{shareToken}/oembed", s.videoHandler.OEmbed)
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
