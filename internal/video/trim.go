package video

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/sendrec/sendrec/internal/database"
)

func videoCodecForContentType(ct string) string {
	switch ct {
	case "video/mp4", "video/quicktime":
		return "libx264"
	default:
		return "libvpx-vp9"
	}
}

func trimVideo(inputPath, outputPath, contentType string, startSeconds, endSeconds float64) error {
	var args []string
	if contentType == "video/mp4" || contentType == "video/quicktime" {
		// iOS-safe encoding: constrain resolution, set profile/level, transcode audio to AAC
		args = []string{
			"-i", inputPath,
			"-ss", fmt.Sprintf("%.3f", startSeconds),
			"-to", fmt.Sprintf("%.3f", endSeconds),
			"-c:v", "libx264",
			"-profile:v", "high",
			"-level:v", "5.1",
			"-preset", "fast",
			"-crf", "23",
			"-vf", "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2",
			"-r", "60",
			"-c:a", "aac",
			"-movflags", "+faststart",
			"-y",
			outputPath,
		}
	} else {
		args = []string{
			"-i", inputPath,
			"-ss", fmt.Sprintf("%.3f", startSeconds),
			"-to", fmt.Sprintf("%.3f", endSeconds),
			"-c:v", "libvpx-vp9",
			"-c:a", "copy",
			"-y",
			outputPath,
		}
	}

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg trim: %w: %s", err, string(output))
	}
	return nil
}

func TrimVideoAsync(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, fileKey, thumbnailKey, contentType string, startSeconds, endSeconds float64) {
	slog.Info("trim: starting", "video_id", videoID, "start_seconds", startSeconds, "end_seconds", endSeconds)

	setReadyFallback := func() {
		if _, err := db.Exec(ctx,
			`UPDATE videos SET status = 'ready', updated_at = now() WHERE id = $1`,
			videoID,
		); err != nil {
			slog.Error("trim: failed to set fallback ready status", "video_id", videoID, "error", err)
		}
	}

	ext := extensionForContentType(contentType)
	tmpInput, err := os.CreateTemp("", "sendrec-trim-input-*"+ext)
	if err != nil {
		slog.Error("trim: failed to create temp input file", "error", err)
		setReadyFallback()
		return
	}
	tmpInputPath := tmpInput.Name()
	_ = tmpInput.Close()
	defer func() { _ = os.Remove(tmpInputPath) }()

	if err := storage.DownloadToFile(ctx, fileKey, tmpInputPath); err != nil {
		slog.Error("trim: failed to download video", "video_id", videoID, "error", err)
		setReadyFallback()
		return
	}

	tmpOutput, err := os.CreateTemp("", "sendrec-trim-output-*"+ext)
	if err != nil {
		slog.Error("trim: failed to create temp output file", "error", err)
		setReadyFallback()
		return
	}
	tmpOutputPath := tmpOutput.Name()
	_ = tmpOutput.Close()
	defer func() { _ = os.Remove(tmpOutputPath) }()

	if err := trimVideo(tmpInputPath, tmpOutputPath, contentType, startSeconds, endSeconds); err != nil {
		slog.Error("trim: ffmpeg failed", "video_id", videoID, "error", err)
		setReadyFallback()
		return
	}

	if err := storage.UploadFile(ctx, fileKey, tmpOutputPath, contentType); err != nil {
		slog.Error("trim: failed to upload trimmed video", "video_id", videoID, "error", err)
		setReadyFallback()
		return
	}

	newDuration := int(endSeconds - startSeconds)
	if _, err := db.Exec(ctx,
		`UPDATE videos SET status = 'ready', duration = $1, updated_at = now() WHERE id = $2`,
		newDuration, videoID,
	); err != nil {
		slog.Error("trim: failed to update status", "video_id", videoID, "error", err)
		return
	}

	GenerateThumbnail(ctx, db, storage, videoID, fileKey, thumbnailKey)
	if err := EnqueueTranscription(ctx, db, videoID); err != nil {
		slog.Error("trim: failed to enqueue transcription", "video_id", videoID, "error", err)
	}
	slog.Info("trim: completed", "video_id", videoID)
}
