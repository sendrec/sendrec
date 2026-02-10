package video

import (
	"context"
	"log"
	"time"

	"github.com/sendrec/sendrec/internal/database"
)

func EnqueueTranscription(ctx context.Context, db database.DBTX, videoID string) error {
	_, err := db.Exec(ctx,
		`UPDATE videos SET transcript_status = 'pending', updated_at = now()
		 WHERE id = $1 AND status != 'deleted'`,
		videoID,
	)
	return err
}

func processNextTranscription(ctx context.Context, db database.DBTX, storage ObjectStorage) {
	// Reset stuck jobs (processing for more than 10 minutes)
	if _, err := db.Exec(ctx,
		`UPDATE videos SET transcript_status = 'pending', transcript_started_at = NULL, updated_at = now()
		 WHERE transcript_status = 'processing' AND transcript_started_at < now() - INTERVAL '10 minutes'`,
	); err != nil {
		log.Printf("transcribe-worker: failed to reset stuck jobs: %v", err)
	}

	// Claim the next pending job
	var videoID, fileKey, userID, shareToken string
	err := db.QueryRow(ctx,
		`UPDATE videos SET transcript_status = 'processing', transcript_started_at = now(), updated_at = now()
		 WHERE id = (
		     SELECT id FROM videos
		     WHERE transcript_status = 'pending' AND status != 'deleted'
		     ORDER BY updated_at ASC LIMIT 1
		     FOR UPDATE SKIP LOCKED
		 )
		 RETURNING id, file_key, user_id, share_token`,
	).Scan(&videoID, &fileKey, &userID, &shareToken)
	if err != nil {
		return // no pending jobs or error
	}

	log.Printf("transcribe-worker: claimed video %s", videoID)
	processTranscription(ctx, db, storage, videoID, fileKey, userID, shareToken)
}

func StartTranscriptionWorker(ctx context.Context, db database.DBTX, storage ObjectStorage, interval time.Duration) {
	go func() {
		log.Println("transcribe-worker: started")
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("transcribe-worker: shutting down")
				return
			case <-ticker.C:
				processNextTranscription(ctx, db, storage)
			}
		}
	}()
}
