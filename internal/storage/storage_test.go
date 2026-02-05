package storage_test

import (
	"context"
	"testing"

	"github.com/sendrec/sendrec/internal/storage"
)

func TestNewStorageRequiresConfig(t *testing.T) {
	ctx := context.Background()

	// Should not panic with valid config (will fail to connect, but that's OK)
	_, err := storage.New(ctx, storage.Config{
		Endpoint:  "http://localhost:9000",
		Bucket:    "test",
		AccessKey: "test",
		SecretKey: "test",
	})
	if err != nil {
		t.Fatalf("expected no error creating storage client, got: %v", err)
	}
}
