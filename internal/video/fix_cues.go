package video

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/sendrec/sendrec/internal/database"
)

func fixWebMCues(inputPath, outputPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-c", "copy",
		"-reserve_index_space", "200k",
		"-y",
		outputPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg fix cues: %w: %s", err, string(output))
	}
	return nil
}

func FixWebMCuesAsync(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, fileKey string) {
	slog.Info("fix-cues: starting", "video_id", videoID)

	tmpInput, err := os.CreateTemp("", "sendrec-fixcues-in-*.webm")
	if err != nil {
		slog.Error("fix-cues: failed to create temp input file", "error", err)
		return
	}
	tmpInputPath := tmpInput.Name()
	_ = tmpInput.Close()
	defer func() { _ = os.Remove(tmpInputPath) }()

	if err := storage.DownloadToFile(ctx, fileKey, tmpInputPath); err != nil {
		slog.Error("fix-cues: failed to download", "video_id", videoID, "error", err)
		return
	}

	tmpOutput, err := os.CreateTemp("", "sendrec-fixcues-out-*.webm")
	if err != nil {
		slog.Error("fix-cues: failed to create temp output file", "error", err)
		return
	}
	tmpOutputPath := tmpOutput.Name()
	_ = tmpOutput.Close()
	defer func() { _ = os.Remove(tmpOutputPath) }()

	if err := fixWebMCues(tmpInputPath, tmpOutputPath); err != nil {
		slog.Error("fix-cues: ffmpeg failed", "video_id", videoID, "error", err)
		return
	}

	if err := storage.UploadFile(ctx, fileKey, tmpOutputPath, "video/webm"); err != nil {
		slog.Error("fix-cues: failed to upload", "video_id", videoID, "error", err)
		return
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET cues_fixed = true, updated_at = now() WHERE id = $1`,
		videoID,
	); err != nil {
		slog.Error("fix-cues: failed to update status", "video_id", videoID, "error", err)
		return
	}

	slog.Info("fix-cues: completed", "video_id", videoID)
}

func fixExistingWebMCues(ctx context.Context, db database.DBTX, storage ObjectStorage) {
	rows, err := db.Query(ctx,
		`SELECT id, file_key FROM videos
		 WHERE content_type = 'video/webm' AND status = 'ready' AND NOT cues_fixed
		 ORDER BY created_at DESC LIMIT 5`)
	if err != nil {
		slog.Error("fix-cues-worker: failed to query", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var videoID, fileKey string
		if err := rows.Scan(&videoID, &fileKey); err != nil {
			slog.Error("fix-cues-worker: failed to scan", "error", err)
			continue
		}
		FixWebMCuesAsync(ctx, db, storage, videoID, fileKey)
	}
}

func StartCuesFixWorker(ctx context.Context, db database.DBTX, storage ObjectStorage, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("fix-cues-worker: shutting down")
				return
			case <-ticker.C:
				fixExistingWebMCues(ctx, db, storage)
			}
		}
	}()
}
