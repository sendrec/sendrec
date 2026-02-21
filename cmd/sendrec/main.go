package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/email"
	"github.com/sendrec/sendrec/internal/plans"
	"github.com/sendrec/sendrec/internal/server"
	slackpkg "github.com/sendrec/sendrec/internal/slack"
	webhookpkg "github.com/sendrec/sendrec/internal/webhook"
	"github.com/sendrec/sendrec/internal/storage"
	"github.com/sendrec/sendrec/internal/video"
	"github.com/sendrec/sendrec/web"
)

func main() {
	port := getEnv("PORT", "8080")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(databaseURL); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}
	log.Println("database migrations applied")

	store, err := storage.New(ctx, storage.Config{
		Endpoint:       getEnv("S3_ENDPOINT", "http://localhost:3900"),
		PublicEndpoint: os.Getenv("S3_PUBLIC_ENDPOINT"),
		Bucket:         getEnv("S3_BUCKET", "sendrec"),
		AccessKey:      os.Getenv("S3_ACCESS_KEY"),
		SecretKey:      os.Getenv("S3_SECRET_KEY"),
		Region:         getEnv("S3_REGION", "eu-central-1"),
		MaxUploadBytes: getEnvInt64("MAX_UPLOAD_BYTES", 500*1024*1024),
	})
	if err != nil {
		log.Fatalf("storage initialization failed: %v", err)
	}

	if err := store.EnsureBucket(ctx); err != nil {
		log.Fatalf("storage bucket check failed: %v", err)
	}

	baseURL := getEnv("BASE_URL", "http://localhost:8080")

	log.Println("storage bucket ready")

	var webFS fs.FS
	if sub, err := fs.Sub(web.DistFS, "dist"); err == nil {
		webFS = sub
		log.Println("embedded frontend loaded")
	} else {
		log.Println("no embedded frontend found, SPA serving disabled")
	}

	emailClient := email.New(email.Config{
		BaseURL:           os.Getenv("LISTMONK_URL"),
		Username:          getEnv("LISTMONK_USER", "admin"),
		Password:          os.Getenv("LISTMONK_PASSWORD"),
		TemplateID:        int(getEnvInt64("LISTMONK_TEMPLATE_ID", 0)),
		CommentTemplateID: int(getEnvInt64("LISTMONK_COMMENT_TEMPLATE_ID", 0)),
		ViewTemplateID:    int(getEnvInt64("LISTMONK_VIEW_TEMPLATE_ID", 0)),
		ConfirmTemplateID: int(getEnvInt64("LISTMONK_CONFIRM_TEMPLATE_ID", 0)),
		Allowlist:         email.ParseAllowlist(os.Getenv("EMAIL_ALLOWLIST")),
	})

	aiEnabled := getEnv("AI_ENABLED", "false") == "true"

	slackClient := slackpkg.New(db.Pool)
	webhookClient := webhookpkg.New(db.Pool)

	creemAPIKey := os.Getenv("CREEM_API_KEY")
	creemWebhookSecret := os.Getenv("CREEM_WEBHOOK_SECRET")
	creemProProductID := os.Getenv("CREEM_PRO_PRODUCT_ID")

	srv := server.New(server.Config{
		DB:                      db.Pool,
		Pinger:                  db,
		Storage:                 store,
		WebFS:                   webFS,
		JWTSecret:               jwtSecret,
		BaseURL:                 baseURL,
		MaxUploadBytes:          getEnvInt64("MAX_UPLOAD_BYTES", 500*1024*1024),
		MaxVideosPerMonth:       int(getEnvInt64("MAX_VIDEOS_PER_MONTH", int64(plans.Free.MaxVideosPerMonth))),
		MaxVideoDurationSeconds: int(getEnvInt64("MAX_VIDEO_DURATION_SECONDS", int64(plans.Free.MaxVideoDurationSeconds))),
		S3PublicEndpoint:        os.Getenv("S3_PUBLIC_ENDPOINT"),
		EnableDocs:              getEnv("API_DOCS_ENABLED", "false") == "true",
		BrandingEnabled:         getEnv("BRANDING_ENABLED", "false") == "true",
		AiEnabled:               aiEnabled,
		AllowedFrameAncestors:   os.Getenv("ALLOWED_FRAME_ANCESTORS"),
		AnalyticsScript:         os.Getenv("ANALYTICS_SCRIPT"),
		EmailSender:             emailClient,
		CommentNotifier:         emailClient,
		ViewNotifier:            emailClient,
		SlackNotifier:           slackClient,
		WebhookClient:           webhookClient,
		CreemAPIKey:             creemAPIKey,
		CreemWebhookSecret:      creemWebhookSecret,
		CreemProProductID:       creemProProductID,
	})

	if creemAPIKey != "" {
		log.Println("Creem billing enabled")
	}

	var aiClient *video.AIClient
	if aiEnabled {
		aiClient = video.NewAIClient(
			os.Getenv("AI_BASE_URL"),
			os.Getenv("AI_API_KEY"),
			getEnv("AI_MODEL", "mistral-small-latest"),
		)
		log.Printf("AI summaries enabled (model: %s)", getEnv("AI_MODEL", "mistral-small-latest"))
	}

	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	defer cleanupCancel()
	video.StartCleanupLoop(cleanupCtx, db.Pool, store, 10*time.Minute)
	video.StartTranscriptionWorker(cleanupCtx, db.Pool, store, 5*time.Second, aiEnabled)
	video.StartSummaryWorker(cleanupCtx, db.Pool, aiClient, 10*time.Second)
	video.StartDigestWorker(cleanupCtx, db.Pool, emailClient, baseURL)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("sendrec listening on :%s", port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-shutdownCh
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown failed: %v", err)
	}
	log.Println("shutdown complete")
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			return parsed
		}
	}
	return fallback
}
