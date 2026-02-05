package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/server"
	"github.com/sendrec/sendrec/internal/storage"
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
		Endpoint:  getEnv("S3_ENDPOINT", "http://localhost:9000"),
		Bucket:    getEnv("S3_BUCKET", "sendrec"),
		AccessKey: getEnv("S3_ACCESS_KEY", "minioadmin"),
		SecretKey: getEnv("S3_SECRET_KEY", "minioadmin"),
	})
	if err != nil {
		log.Fatalf("storage initialization failed: %v", err)
	}

	if err := store.EnsureBucket(ctx); err != nil {
		log.Fatalf("storage bucket check failed: %v", err)
	}
	log.Println("storage bucket ready")

	srv := server.New(db, store)

	log.Printf("sendrec listening on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), srv); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
