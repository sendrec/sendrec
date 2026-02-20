package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/database"
)

const maxTranscriptChars = 30000

func formatTranscriptForLLM(segments []TranscriptSegment) string {
	var result string
	for _, seg := range segments {
		totalSeconds := int(seg.Start)
		minutes := totalSeconds / 60
		seconds := totalSeconds % 60
		line := fmt.Sprintf("[%02d:%02d] %s\n", minutes, seconds, seg.Text)
		if len(result)+len(line) > maxTranscriptChars {
			break
		}
		result += line
	}
	return result
}

func processNextSummary(ctx context.Context, db database.DBTX, ai *AIClient) {
	if _, err := db.Exec(ctx,
		`UPDATE videos SET summary_status = 'pending', summary_started_at = NULL, updated_at = now()
		 WHERE summary_status = 'processing'
		   AND (summary_started_at < now() - INTERVAL '10 minutes' OR summary_started_at IS NULL)`,
	); err != nil {
		log.Printf("summary-worker: failed to reset stuck jobs: %v", err)
	}

	var videoID string
	var transcriptJSON []byte
	err := db.QueryRow(ctx,
		`UPDATE videos SET summary_status = 'processing', summary_started_at = now(), updated_at = now()
		 WHERE id = (
		     SELECT id FROM videos
		     WHERE summary_status = 'pending' AND status != 'deleted'
		     ORDER BY updated_at ASC LIMIT 1
		     FOR UPDATE SKIP LOCKED
		 )
		 RETURNING id, transcript_json`,
	).Scan(&videoID, &transcriptJSON)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("summary-worker: failed to claim job: %v", err)
		}
		return
	}

	var segments []TranscriptSegment
	if err := json.Unmarshal(transcriptJSON, &segments); err != nil {
		log.Printf("summary-worker: failed to parse transcript for video %s: %v", videoID, err)
		markSummaryFailed(ctx, db, videoID)
		return
	}

	if len(segments) < 2 {
		log.Printf("summary-worker: skipping video %s (only %d segments)", videoID, len(segments))
		markSummaryFailed(ctx, db, videoID)
		return
	}

	transcript := formatTranscriptForLLM(segments)
	result, err := ai.GenerateSummary(ctx, transcript)
	if err != nil {
		log.Printf("summary-worker: AI generation failed for video %s: %v", videoID, err)
		markSummaryFailed(ctx, db, videoID)
		return
	}

	chaptersJSON, err := json.Marshal(result.Chapters)
	if err != nil {
		log.Printf("summary-worker: failed to marshal chapters for video %s: %v", videoID, err)
		markSummaryFailed(ctx, db, videoID)
		return
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET summary = $1, chapters = $2, summary_status = 'ready', summary_started_at = NULL, updated_at = now()
		 WHERE id = $3`,
		result.Summary, chaptersJSON, videoID,
	); err != nil {
		log.Printf("summary-worker: failed to save summary for video %s: %v", videoID, err)
	}
}

func markSummaryFailed(ctx context.Context, db database.DBTX, videoID string) {
	if _, err := db.Exec(ctx,
		`UPDATE videos SET summary_status = 'failed', summary_started_at = NULL, updated_at = now()
		 WHERE id = $1`,
		videoID,
	); err != nil {
		log.Printf("summary-worker: failed to mark video %s as failed: %v", videoID, err)
	}
}

func StartSummaryWorker(ctx context.Context, db database.DBTX, ai *AIClient, interval time.Duration) {
	if ai == nil {
		return
	}
	go func() {
		log.Println("summary-worker: started")
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("summary-worker: shutting down")
				return
			case <-ticker.C:
				processNextSummary(ctx, db, ai)
			}
		}
	}()
}
