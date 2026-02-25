package video

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/languages"
)

func processNextDocument(ctx context.Context, db database.DBTX, ai *AIClient) {
	if _, err := db.Exec(ctx,
		`UPDATE videos SET document_status = 'pending', document_started_at = NULL, updated_at = now()
		 WHERE document_status = 'processing'
		   AND (document_started_at < now() - INTERVAL '10 minutes' OR document_started_at IS NULL)`,
	); err != nil {
		slog.Error("document-worker: failed to reset stuck jobs", "error", err)
	}

	var videoID string
	var transcriptJSON []byte
	var language *string
	err := db.QueryRow(ctx,
		`UPDATE videos SET document_status = 'processing', document_started_at = now(), updated_at = now()
		 WHERE id = (
		     SELECT id FROM videos
		     WHERE document_status = 'pending' AND status != 'deleted'
		     ORDER BY updated_at ASC LIMIT 1
		     FOR UPDATE SKIP LOCKED
		 )
		 RETURNING id, transcript_json, transcription_language`,
	).Scan(&videoID, &transcriptJSON, &language)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("document-worker: failed to claim job", "error", err)
		}
		return
	}

	var segments []TranscriptSegment
	if err := json.Unmarshal(transcriptJSON, &segments); err != nil {
		slog.Error("document-worker: failed to parse transcript", "video_id", videoID, "error", err)
		markDocumentStatus(ctx, db, videoID, "failed")
		return
	}

	if len(segments) < 2 {
		slog.Warn("document-worker: skipping video, insufficient segments", "video_id", videoID, "segments", len(segments))
		markDocumentStatus(ctx, db, videoID, "too_short")
		return
	}

	transcript := formatTranscriptForLLM(segments)

	var langName string
	if language != nil && *language != "" && *language != "auto" {
		langName = languages.LanguageName(*language)
	}

	document, err := ai.GenerateDocument(ctx, transcript, langName)
	if err != nil {
		slog.Error("document-worker: AI generation failed", "video_id", videoID, "error", err)
		markDocumentStatus(ctx, db, videoID, "failed")
		return
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET document = $1, document_status = 'ready', document_started_at = NULL, updated_at = now()
		 WHERE id = $2`,
		document, videoID,
	); err != nil {
		slog.Error("document-worker: failed to save document", "video_id", videoID, "error", err)
		return
	}

	slog.Info("document-worker: generated document", "video_id", videoID)
}

func markDocumentStatus(ctx context.Context, db database.DBTX, videoID, status string) {
	if _, err := db.Exec(ctx,
		`UPDATE videos SET document_status = $1, document_started_at = NULL, updated_at = now()
		 WHERE id = $2`,
		status, videoID,
	); err != nil {
		slog.Error("document-worker: failed to update status", "video_id", videoID, "status", status, "error", err)
	}
}

func StartDocumentWorker(ctx context.Context, db database.DBTX, ai *AIClient, interval time.Duration) {
	if ai == nil {
		return
	}
	go func() {
		slog.Info("document-worker: started")
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("document-worker: shutting down")
				return
			case <-ticker.C:
				processNextDocument(ctx, db, ai)
			}
		}
	}()
}
