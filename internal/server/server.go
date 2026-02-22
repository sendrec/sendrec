package server

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/billing"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/docs"
	"github.com/sendrec/sendrec/internal/ratelimit"
	"github.com/sendrec/sendrec/internal/video"
	"github.com/sendrec/sendrec/internal/webhook"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Config struct {
	Version                 string
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
	AiEnabled               bool
	AllowedFrameAncestors   string
	AnalyticsScript         string
	EmailSender             auth.EmailSender
	CommentNotifier         video.CommentNotifier
	ViewNotifier            video.ViewNotifier
	SlackNotifier           video.SlackNotifier
	WebhookClient           *webhook.Client
	CreemAPIKey             string
	CreemWebhookSecret      string
	CreemProProductID       string
}

type Server struct {
	router          chi.Router
	version         string
	pinger          Pinger
	authHandler     *auth.Handler
	videoHandler    *video.Handler
	db              database.DBTX
	billingHandlers *billing.Handlers
	webFS           fs.FS
	enableDocs      bool
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

	s := &Server{router: r, version: cfg.Version, pinger: cfg.Pinger, db: cfg.DB, webFS: cfg.WebFS, enableDocs: cfg.EnableDocs}

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
		if cfg.AiEnabled {
			s.videoHandler.SetAIEnabled(true)
		}
		if cfg.SlackNotifier != nil {
			s.videoHandler.SetSlackNotifier(cfg.SlackNotifier)
		}
		if cfg.WebhookClient != nil {
			s.videoHandler.SetWebhookClient(cfg.WebhookClient)
		}

		if cfg.CreemAPIKey != "" {
			creemClient := billing.New(cfg.CreemAPIKey, "")
			s.billingHandlers = billing.NewHandlers(cfg.DB, creemClient, baseURL, cfg.CreemProProductID, cfg.CreemWebhookSecret)
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
			r.Post("/notifications/test-slack", s.videoHandler.TestSlackWebhook)
			r.Post("/notifications/test-webhook", s.videoHandler.TestWebhook)
			r.Post("/notifications/regenerate-webhook-secret", s.videoHandler.RegenerateWebhookSecret)
			r.Get("/notifications/webhook-deliveries", s.videoHandler.ListWebhookDeliveries)
			r.Get("/branding", s.videoHandler.GetBrandingSettings)
			r.Put("/branding", s.videoHandler.PutBrandingSettings)
			r.Post("/branding/logo", s.videoHandler.UploadBrandingLogo)
			r.Delete("/branding/logo", s.videoHandler.DeleteBrandingLogo)
			r.Post("/api-keys", auth.GenerateAPIKey(s.db))
			r.Get("/api-keys", auth.ListAPIKeys(s.db))
			r.Delete("/api-keys/{id}", auth.DeleteAPIKey(s.db))
			if s.billingHandlers != nil {
				r.Get("/billing", s.billingHandlers.GetBilling)
				r.Post("/billing/checkout", s.billingHandlers.CreateCheckout)
				r.Post("/billing/cancel", s.billingHandlers.CancelSubscription)
			}
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
				r.Post("/batch/delete", s.videoHandler.BatchDelete)
				r.Post("/batch/folder", s.videoHandler.BatchSetFolder)
				r.Post("/batch/tags", s.videoHandler.BatchSetTags)
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
				r.Put("/{id}/cta", s.videoHandler.SetCTA)
				r.Put("/{id}/email-gate", s.videoHandler.SetEmailGate)
				r.Post("/{id}/summarize", s.videoHandler.Summarize)
				r.Put("/{id}/folder", s.videoHandler.SetVideoFolder)
				r.Put("/{id}/tags", s.videoHandler.SetVideoTags)
			r.Post("/{id}/remove-segments", s.videoHandler.RemoveSegments)
			r.Put("/{id}/dismiss-title", s.videoHandler.DismissTitle)
			})
		})

		s.router.Route("/api/folders", func(r chi.Router) {
			r.Use(s.authHandler.Middleware)
			r.Use(maxBodySize(64 * 1024))
			r.Get("/", s.videoHandler.ListFolders)
			r.Post("/", s.videoHandler.CreateFolder)
			r.Put("/{id}", s.videoHandler.UpdateFolder)
			r.Delete("/{id}", s.videoHandler.DeleteFolder)
		})

		s.router.Route("/api/tags", func(r chi.Router) {
			r.Use(s.authHandler.Middleware)
			r.Use(maxBodySize(64 * 1024))
			r.Get("/", s.videoHandler.ListTags)
			r.Post("/", s.videoHandler.CreateTag)
			r.Put("/{id}", s.videoHandler.UpdateTag)
			r.Delete("/{id}", s.videoHandler.DeleteTag)
		})

		watchAuthLimiter := ratelimit.NewLimiter(0.5, 5)
		commentLimiter := ratelimit.NewLimiter(0.2, 3)
		s.router.Get("/api/watch/{shareToken}", s.videoHandler.Watch)
		s.router.Get("/api/watch/{shareToken}/download", s.videoHandler.WatchDownload)
		s.router.With(watchAuthLimiter.Middleware, maxBodySize(64*1024)).Post("/api/watch/{shareToken}/verify", s.videoHandler.VerifyWatchPassword)
		s.router.Get("/api/watch/{shareToken}/comments", s.videoHandler.ListWatchComments)
		s.router.With(commentLimiter.Middleware, maxBodySize(64*1024)).Post("/api/watch/{shareToken}/comments", s.videoHandler.PostWatchComment)
		s.router.With(watchAuthLimiter.Middleware, maxBodySize(64*1024)).Post("/api/watch/{shareToken}/identify", s.videoHandler.IdentifyViewer)
		s.router.Post("/api/watch/{shareToken}/cta-click", s.videoHandler.RecordCTAClick)
		s.router.Post("/api/watch/{shareToken}/milestone", s.videoHandler.RecordMilestone)
		s.router.Get("/api/videos/{shareToken}/oembed", s.videoHandler.OEmbed)
		s.router.Get("/watch/{shareToken}", s.videoHandler.WatchPage)
		s.router.Get("/embed/{shareToken}", s.videoHandler.EmbedPage)

		if s.billingHandlers != nil {
			s.router.Post("/api/webhooks/creem", s.billingHandlers.Webhook)
		}
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
			_, _ = fmt.Fprintf(w, `{"status":"unhealthy","version":%q,"error":"database unreachable"}`, s.version)
			return
		}
	}
	_, _ = fmt.Fprintf(w, `{"status":"ok","version":%q}`, s.version)
}
