package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

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

	srv := server.New(db)

	log.Printf("sendrec listening on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), srv); err != nil {
		log.Fatal(err)
	}
}
