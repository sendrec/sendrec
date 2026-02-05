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
	"syscall"
	"time"

	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/server"
	"github.com/sendrec/sendrec/internal/storage"
	"github.com/sendrec/sendrec/web"
)

func main() {
	port := getEnv("PORT", "8080")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx := context.Background()

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
		Endpoint:       getEnv("S3_ENDPOINT", "http://localhost:9000"),
		PublicEndpoint: os.Getenv("S3_PUBLIC_ENDPOINT"),
		Bucket:         getEnv("S3_BUCKET", "sendrec"),
		AccessKey:      getEnv("S3_ACCESS_KEY", "minioadmin"),
		SecretKey:      getEnv("S3_SECRET_KEY", "minioadmin"),
	})
	if err != nil {
		log.Fatalf("storage initialization failed: %v", err)
	}

	if err := store.EnsureBucket(ctx); err != nil {
		log.Fatalf("storage bucket check failed: %v", err)
	}

	baseURL := getEnv("BASE_URL", "http://localhost:8080")
	if err := store.SetCORS(ctx, []string{baseURL}); err != nil {
		log.Printf("warning: failed to set storage CORS: %v", err)
	}
	log.Println("storage bucket ready")

	var webFS fs.FS
	if sub, err := fs.Sub(web.DistFS, "dist"); err == nil {
		webFS = sub
		log.Println("embedded frontend loaded")
	} else {
		log.Println("no embedded frontend found, SPA serving disabled")
	}

	jwtSecret := os.Getenv("JWT_SECRET")

	srv := server.New(server.Config{
		DB:        db.Pool,
		Pinger:    db,
		Storage:   store,
		WebFS:     webFS,
		JWTSecret: jwtSecret,
		BaseURL:   baseURL,
	})

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: srv,
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
