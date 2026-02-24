package video

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/sendrec/sendrec/internal/database"
)

func compositeOverlay(screenPath, webcamPath, outputPath, contentType string) error {
	pipFilter := "[1:v]scale=240:-1,pad=iw+8:ih+8:(ow-iw)/2:(oh-ih)/2:color=black@0.3[pip];[0:v][pip]overlay=W-w-20:H-h-20"

	var args []string
	if contentType == "video/mp4" || contentType == "video/quicktime" {
		// iOS-safe encoding: constrain resolution, set profile/level, transcode audio to AAC
		filterComplex := pipFilter + ",scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2"
		args = []string{
			"-i", screenPath,
			"-i", webcamPath,
			"-filter_complex", filterComplex,
			"-map", "0:a?",
			"-c:v", "libx264",
			"-profile:v", "high",
			"-level:v", "5.1",
			"-preset", "fast",
			"-crf", "23",
			"-r", "60",
			"-c:a", "aac",
			"-movflags", "+faststart",
			"-y",
			outputPath,
		}
	} else {
		args = []string{
			"-i", screenPath,
			"-i", webcamPath,
			"-filter_complex", pipFilter,
			"-map", "0:a?",
			"-c:a", "copy",
			"-c:v", "libvpx-vp9",
			"-y",
			outputPath,
		}
	}

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg composite: %w: %s", err, string(output))
	}
	return nil
}

func CompositeWithWebcam(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, screenKey, webcamKey, thumbnailKey, contentType string) {
	slog.Info("composite: starting webcam overlay", "video_id", videoID)

	setReadyFallback := func() {
		if _, err := db.Exec(ctx,
			`UPDATE videos SET status = 'ready', webcam_key = NULL, updated_at = now() WHERE id = $1`,
			videoID,
		); err != nil {
			slog.Error("composite: failed to set fallback ready status", "video_id", videoID, "error", err)
		}
	}

	ext := extensionForContentType(contentType)
	tmpScreen, err := os.CreateTemp("", "sendrec-composite-screen-*"+ext)
	if err != nil {
		slog.Error("composite: failed to create temp screen file", "error", err)
		setReadyFallback()
		return
	}
	tmpScreenPath := tmpScreen.Name()
	_ = tmpScreen.Close()
	defer func() { _ = os.Remove(tmpScreenPath) }()

	if err := storage.DownloadToFile(ctx, screenKey, tmpScreenPath); err != nil {
		slog.Error("composite: failed to download screen", "video_id", videoID, "error", err)
		setReadyFallback()
		return
	}

	tmpWebcam, err := os.CreateTemp("", "sendrec-composite-webcam-*"+ext)
	if err != nil {
		slog.Error("composite: failed to create temp webcam file", "error", err)
		setReadyFallback()
		return
	}
	tmpWebcamPath := tmpWebcam.Name()
	_ = tmpWebcam.Close()
	defer func() { _ = os.Remove(tmpWebcamPath) }()

	if err := storage.DownloadToFile(ctx, webcamKey, tmpWebcamPath); err != nil {
		slog.Error("composite: failed to download webcam", "video_id", videoID, "error", err)
		setReadyFallback()
		return
	}

	tmpOutput, err := os.CreateTemp("", "sendrec-composite-output-*"+ext)
	if err != nil {
		slog.Error("composite: failed to create temp output file", "error", err)
		setReadyFallback()
		return
	}
	tmpOutputPath := tmpOutput.Name()
	_ = tmpOutput.Close()
	defer func() { _ = os.Remove(tmpOutputPath) }()

	if err := compositeOverlay(tmpScreenPath, tmpWebcamPath, tmpOutputPath, contentType); err != nil {
		slog.Error("composite: ffmpeg failed", "video_id", videoID, "error", err)
		setReadyFallback()
		return
	}

	if err := storage.UploadFile(ctx, screenKey, tmpOutputPath, contentType); err != nil {
		slog.Error("composite: failed to upload composited video", "video_id", videoID, "error", err)
		setReadyFallback()
		return
	}

	if err := storage.DeleteObject(ctx, webcamKey); err != nil {
		slog.Error("composite: failed to delete webcam file", "key", webcamKey, "error", err)
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET status = 'ready', webcam_key = NULL, updated_at = now() WHERE id = $1`,
		videoID,
	); err != nil {
		slog.Error("composite: failed to update status", "video_id", videoID, "error", err)
		return
	}

	GenerateThumbnail(ctx, db, storage, videoID, screenKey, thumbnailKey)
	if err := EnqueueTranscription(ctx, db, videoID); err != nil {
		slog.Error("composite: failed to enqueue transcription", "video_id", videoID, "error", err)
	}
	slog.Info("composite: completed", "video_id", videoID)
}
