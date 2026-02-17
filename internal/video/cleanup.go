package video

import (
	"context"
	"log"
	"time"

	"github.com/sendrec/sendrec/internal/database"
)

func PurgeOrphanedFiles(ctx context.Context, db database.DBTX, storage ObjectStorage) {
	rows, err := db.Query(ctx,
		`SELECT file_key, thumbnail_key, transcript_key FROM videos
		 WHERE status = 'deleted' AND file_purged_at IS NULL
		 LIMIT 50`)
	if err != nil {
		log.Printf("cleanup: failed to query orphaned files: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var fileKey string
		var thumbnailKey *string
		var transcriptKey *string
		if err := rows.Scan(&fileKey, &thumbnailKey, &transcriptKey); err != nil {
			log.Printf("cleanup: failed to scan file key: %v", err)
			continue
		}
		if err := deleteWithRetry(ctx, storage, fileKey, 3); err != nil {
			log.Printf("cleanup: failed to delete %s: %v", fileKey, err)
			continue
		}
		if thumbnailKey != nil {
			if err := deleteWithRetry(ctx, storage, *thumbnailKey, 3); err != nil {
				log.Printf("cleanup: failed to delete thumbnail %s: %v", *thumbnailKey, err)
			}
		}
		if transcriptKey != nil {
			if err := deleteWithRetry(ctx, storage, *transcriptKey, 3); err != nil {
				log.Printf("cleanup: failed to delete transcript %s: %v", *transcriptKey, err)
			}
		}
		if _, err := db.Exec(ctx,
			`UPDATE videos SET file_purged_at = now() WHERE file_key = $1`,
			fileKey,
		); err != nil {
			log.Printf("cleanup: failed to mark purged for %s: %v", fileKey, err)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("cleanup: row iteration error: %v", err)
	}
}

func StartCleanupLoop(ctx context.Context, db database.DBTX, storage ObjectStorage, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("cleanup: shutting down")
				return
			case <-ticker.C:
				PurgeOrphanedFiles(ctx, db, storage)
			}
		}
	}()
}
