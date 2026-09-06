package video

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sendrec/sendrec/internal/database"
)

// pgxmock accepts cancelled contexts, unlike the database driver.
type recoveryDB struct {
	database.DBTX
	updated  bool
	deadline time.Time
}

func (db *recoveryDB) Exec(ctx context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	if err := ctx.Err(); err != nil {
		return pgconn.CommandTag{}, err
	}
	db.updated = true
	db.deadline, _ = ctx.Deadline()
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func TestFailedEditsRecoverAfterCancellation(t *testing.T) {
	for _, edit := range []struct {
		name string
		run  func(context.Context, database.DBTX, ObjectStorage)
	}{
		{"trim", func(ctx context.Context, db database.DBTX, storage ObjectStorage) {
			TrimVideoAsync(ctx, db, storage, "video", "input", "thumbnail", "video/mp4", 0, 2)
		}},
		{"composite", func(ctx context.Context, db database.DBTX, storage ObjectStorage) {
			CompositeWithWebcam(ctx, db, storage, "video", "input", "webcam", "thumbnail", "video/mp4")
		}},
		{"remove-segments", func(ctx context.Context, db database.DBTX, storage ObjectStorage) {
			RemoveSegmentsAsync(ctx, db, storage, "video", "input", "thumbnail", "video/mp4", []segmentRange{{Start: 1, End: 2}}, 3)
		}},
	} {
		t.Run(edit.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			db := &recoveryDB{}
			edit.run(ctx, db, &mockStorage{downloadToFileErr: context.Canceled})
			if !db.updated {
				t.Fatal("cancelled edit left its processing status stuck")
			}
			if remaining := time.Until(db.deadline); remaining <= 0 || remaining > 10*time.Second {
				t.Errorf("recovery must have a fresh bounded deadline, got %v", db.deadline)
			}
		})
	}
}
