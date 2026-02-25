package video

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
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

func processNextTranscription(ctx context.Context, db database.DBTX, storage ObjectStorage, aiEnabled bool) {
	// Reset stuck jobs (processing for more than 10 minutes)
	if _, err := db.Exec(ctx,
		`UPDATE videos SET transcript_status = 'pending', transcript_started_at = NULL, updated_at = now()
		 WHERE transcript_status = 'processing'
		   AND (transcript_started_at < now() - INTERVAL '10 minutes' OR transcript_started_at IS NULL)`,
	); err != nil {
		slog.Error("transcribe-worker: failed to reset stuck jobs", "error", err)
	}

	// Claim the next pending job
	var videoID, fileKey, userID, shareToken, language string
	err := db.QueryRow(ctx,
		`UPDATE videos SET transcript_status = 'processing', transcript_started_at = now(), updated_at = now()
		 WHERE id = (
		     SELECT v.id FROM videos v
		     WHERE v.transcript_status = 'pending' AND v.status != 'deleted'
		     ORDER BY v.updated_at ASC LIMIT 1
		     FOR UPDATE SKIP LOCKED
		 )
		 RETURNING id, file_key, user_id, share_token,
		     COALESCE(transcription_language, (SELECT transcription_language FROM users WHERE id = videos.user_id))`,
	).Scan(&videoID, &fileKey, &userID, &shareToken, &language)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("transcribe-worker: failed to claim job", "error", err)
		}
		return
	}

	slog.Info("transcribe-worker: claimed video", "video_id", videoID)
	processTranscription(ctx, db, storage, videoID, fileKey, userID, shareToken, language, aiEnabled)
}

func StartTranscriptionWorker(ctx context.Context, db database.DBTX, storage ObjectStorage, interval time.Duration, aiEnabled bool) {
	go func() {
		slog.Info("transcribe-worker: started")
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("transcribe-worker: shutting down")
				return
			case <-ticker.C:
				processNextTranscription(ctx, db, storage, aiEnabled)
			}
		}
	}()
}
