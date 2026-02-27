package database

import (
	"context"
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
