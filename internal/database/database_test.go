package database

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestConnect_InvalidURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := Connect(ctx, "postgres://invalid:invalid@localhost:1/nonexistent?sslmode=disable&connect_timeout=1")
	if err == nil {
		t.Fatal("expected error for invalid database URL")
	}
}

func TestMigrate_InvalidURL(t *testing.T) {
	db := &DB{}
	err := db.Migrate("postgres://invalid:invalid@localhost:1/nonexistent")
	if err == nil {
		t.Fatal("expected error for invalid migration URL")
	}
}

func TestMigrate_AcceptsPoolParams(t *testing.T) {
	baseURL := os.Getenv("TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	databaseURL := baseURL + "&pool_max_conns=5"

	db, err := Connect(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(databaseURL); err != nil {
		t.Fatalf("migrate with pool params: %v", err)
	}
}
