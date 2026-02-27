package video

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sendrec/sendrec/internal/database"
)

func buildTranscodeArgs(inputPath, outputPath, audioFilter string) []string {
	args := []string{
		"-i", inputPath,
		"-c:v", "libx264",
		"-profile:v", "high",
		"-level:v", "5.1",
		"-preset", "fast",
		"-crf", "23",
		"-vf", "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2",
		"-r", "60",
	}
	if audioFilter != "" {
		args = append(args, "-af", audioFilter)
	}
	args = append(args, "-c:a", "aac", "-movflags", "+faststart", "-y", outputPath)
	return args
}

func transcodeToMP4(inputPath, outputPath, audioFilter string) error {
	args := buildTranscodeArgs(inputPath, outputPath, audioFilter)
	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg transcode: %w: %s", err, string(output))
	}
	return nil
}

func TranscodeWebMAsync(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, fileKey, audioFilter string) {
	// Check if video is still WebM (another transcode may have already completed)
	var contentType string
	if err := db.QueryRow(ctx, "SELECT content_type FROM videos WHERE id = $1", videoID).Scan(&contentType); err != nil {
		slog.Error("transcode: failed to check content type", "video_id", videoID, "error", err)
		return
	}
	if contentType != "video/webm" {
		slog.Info("transcode: skipped, already transcoded", "video_id", videoID, "content_type", contentType)
		return
	}

	slog.Info("transcode: starting", "video_id", videoID, "audio_filter", audioFilter)

	tmpInput, err := os.CreateTemp("", "sendrec-transcode-in-*.webm")
	if err != nil {
		slog.Error("transcode: failed to create temp input file", "error", err)
		return
	}
	tmpInputPath := tmpInput.Name()
	_ = tmpInput.Close()
	defer func() { _ = os.Remove(tmpInputPath) }()

	if err := storage.DownloadToFile(ctx, fileKey, tmpInputPath); err != nil {
		slog.Error("transcode: failed to download", "video_id", videoID, "error", err)
		return
	}

	tmpOutput, err := os.CreateTemp("", "sendrec-transcode-out-*.mp4")
	if err != nil {
		slog.Error("transcode: failed to create temp output file", "error", err)
		return
	}
	tmpOutputPath := tmpOutput.Name()
	_ = tmpOutput.Close()
	defer func() { _ = os.Remove(tmpOutputPath) }()

	if err := transcodeToMP4(tmpInputPath, tmpOutputPath, audioFilter); err != nil {
		slog.Error("transcode: ffmpeg failed", "video_id", videoID, "error", err)
		return
	}

	info, err := os.Stat(tmpOutputPath)
	if err != nil {
		slog.Error("transcode: failed to stat output", "video_id", videoID, "error", err)
		return
	}
	newFileSize := info.Size()

	newFileKey := strings.TrimSuffix(fileKey, ".webm") + ".mp4"

	if err := storage.UploadFile(ctx, newFileKey, tmpOutputPath, "video/mp4"); err != nil {
		slog.Error("transcode: failed to upload", "video_id", videoID, "error", err)
		return
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET file_key = $2, content_type = 'video/mp4', file_size = $3, cues_fixed = true, ios_normalized = true, updated_at = now() WHERE id = $1`,
		videoID, newFileKey, newFileSize,
	); err != nil {
		slog.Error("transcode: failed to update db", "video_id", videoID, "error", err)
		return
	}

	if err := storage.DeleteObject(ctx, fileKey); err != nil {
		slog.Warn("transcode: failed to delete old webm", "video_id", videoID, "key", fileKey, "error", err)
	}

	slog.Info("transcode: completed", "video_id", videoID, "new_key", newFileKey, "size", newFileSize)
}

func transcodeExistingWebM(ctx context.Context, db database.DBTX, storage ObjectStorage) {
	rows, err := db.Query(ctx,
		`SELECT id, file_key FROM videos
		 WHERE content_type = 'video/webm' AND status = 'ready'
		   AND created_at < now() - interval '5 minutes'
		 ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		slog.Error("transcode-worker: failed to query", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var videoID, fileKey string
		if err := rows.Scan(&videoID, &fileKey); err != nil {
			slog.Error("transcode-worker: failed to scan", "error", err)
			continue
		}
		TranscodeWebMAsync(ctx, db, storage, videoID, fileKey, "")
	}
}

func normalizeExistingVideos(ctx context.Context, db database.DBTX, storage ObjectStorage) {
	rows, err := db.Query(ctx,
		`SELECT id, file_key FROM videos
		 WHERE content_type IN ('video/mp4', 'video/quicktime')
		   AND status = 'ready' AND ios_normalized = false
		   AND created_at < now() - interval '5 minutes'
		 ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		slog.Error("normalize-worker: failed to query", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var videoID, fileKey string
		if err := rows.Scan(&videoID, &fileKey); err != nil {
			slog.Error("normalize-worker: failed to scan", "error", err)
			continue
		}
		NormalizeVideoAsync(ctx, db, storage, videoID, fileKey, "")
	}
}

func StartTranscodeWorker(ctx context.Context, db database.DBTX, storage ObjectStorage, interval time.Duration) {
	go func() {
		transcodeExistingWebM(ctx, db, storage)
		normalizeExistingVideos(ctx, db, storage)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("transcode-worker: shutting down")
				return
			case <-ticker.C:
				transcodeExistingWebM(ctx, db, storage)
				normalizeExistingVideos(ctx, db, storage)
			}
		}
	}()
}
