package video

import (
	"context"
	"log"
	"time"

	"github.com/sendrec/sendrec/internal/database"
)

func PurgeOrphanedFiles(ctx context.Context, db database.DBTX, storage ObjectStorage) {
	rows, err := db.Query(ctx,
		`SELECT file_key FROM videos
		 WHERE status = 'deleted' AND file_purged_at IS NULL
		 LIMIT 50`)
	if err != nil {
		log.Printf("cleanup: failed to query orphaned files: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var fileKey string
		if err := rows.Scan(&fileKey); err != nil {
			log.Printf("cleanup: failed to scan file key: %v", err)
			continue
		}
		if err := deleteWithRetry(ctx, storage, fileKey, 3); err != nil {
			log.Printf("cleanup: failed to delete %s: %v", fileKey, err)
			continue
		}
		if _, err := db.Exec(ctx,
			`UPDATE videos SET file_purged_at = now() WHERE file_key = $1`,
			fileKey,
		); err != nil {
			log.Printf("cleanup: failed to mark purged for %s: %v", fileKey, err)
		}
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
