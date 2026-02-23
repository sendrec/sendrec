package video

import (
	"context"
	"log/slog"
	"time"

	"github.com/sendrec/sendrec/internal/database"
)

func PurgeOrphanedFiles(ctx context.Context, db database.DBTX, storage ObjectStorage) {
	rows, err := db.Query(ctx,
		`SELECT file_key, thumbnail_key, transcript_key FROM videos
		 WHERE status = 'deleted' AND file_purged_at IS NULL
		 LIMIT 50`)
	if err != nil {
		slog.Error("cleanup: failed to query orphaned files", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var fileKey string
		var thumbnailKey *string
		var transcriptKey *string
		if err := rows.Scan(&fileKey, &thumbnailKey, &transcriptKey); err != nil {
			slog.Error("cleanup: failed to scan file key", "error", err)
			continue
		}
		if err := deleteWithRetry(ctx, storage, fileKey, 3); err != nil {
			slog.Error("cleanup: failed to delete file", "key", fileKey, "error", err)
			continue
		}
		if thumbnailKey != nil {
			if err := deleteWithRetry(ctx, storage, *thumbnailKey, 3); err != nil {
				slog.Error("cleanup: failed to delete thumbnail", "key", *thumbnailKey, "error", err)
			}
		}
		if transcriptKey != nil {
			if err := deleteWithRetry(ctx, storage, *transcriptKey, 3); err != nil {
				slog.Error("cleanup: failed to delete transcript", "key", *transcriptKey, "error", err)
			}
		}
		if _, err := db.Exec(ctx,
			`UPDATE videos SET file_purged_at = now() WHERE file_key = $1`,
			fileKey,
		); err != nil {
			slog.Error("cleanup: failed to mark purged", "key", fileKey, "error", err)
		}
	}
	if err := rows.Err(); err != nil {
		slog.Error("cleanup: row iteration error", "error", err)
	}
}

func StartCleanupLoop(ctx context.Context, db database.DBTX, storage ObjectStorage, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("cleanup: shutting down")
				return
			case <-ticker.C:
				PurgeOrphanedFiles(ctx, db, storage)
			}
		}
	}()
}
