package video

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/sendrec/sendrec/internal/database"
)

func trimVideo(inputPath, outputPath string, startSeconds, endSeconds float64) error {
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-ss", fmt.Sprintf("%.3f", startSeconds),
		"-to", fmt.Sprintf("%.3f", endSeconds),
		"-c:v", "libvpx-vp9",
		"-c:a", "copy",
		"-y",
		outputPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg trim: %w: %s", err, string(output))
	}
	return nil
}

func TrimVideoAsync(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, fileKey, thumbnailKey string, startSeconds, endSeconds float64) {
	log.Printf("trim: starting for video %s (%.1f-%.1f)", videoID, startSeconds, endSeconds)

	setReadyFallback := func() {
		if _, err := db.Exec(ctx,
			`UPDATE videos SET status = 'ready', updated_at = now() WHERE id = $1`,
			videoID,
		); err != nil {
			log.Printf("trim: failed to set fallback ready status for %s: %v", videoID, err)
		}
	}

	tmpInput, err := os.CreateTemp("", "sendrec-trim-input-*.webm")
	if err != nil {
		log.Printf("trim: failed to create temp input file: %v", err)
		setReadyFallback()
		return
	}
	tmpInputPath := tmpInput.Name()
	_ = tmpInput.Close()
	defer func() { _ = os.Remove(tmpInputPath) }()

	if err := storage.DownloadToFile(ctx, fileKey, tmpInputPath); err != nil {
		log.Printf("trim: failed to download video %s: %v", videoID, err)
		setReadyFallback()
		return
	}

	tmpOutput, err := os.CreateTemp("", "sendrec-trim-output-*.webm")
	if err != nil {
		log.Printf("trim: failed to create temp output file: %v", err)
		setReadyFallback()
		return
	}
	tmpOutputPath := tmpOutput.Name()
	_ = tmpOutput.Close()
	defer func() { _ = os.Remove(tmpOutputPath) }()

	if err := trimVideo(tmpInputPath, tmpOutputPath, startSeconds, endSeconds); err != nil {
		log.Printf("trim: ffmpeg failed for %s: %v", videoID, err)
		setReadyFallback()
		return
	}

	if err := storage.UploadFile(ctx, fileKey, tmpOutputPath, "video/webm"); err != nil {
		log.Printf("trim: failed to upload trimmed video %s: %v", videoID, err)
		setReadyFallback()
		return
	}

	newDuration := int(endSeconds - startSeconds)
	if _, err := db.Exec(ctx,
		`UPDATE videos SET status = 'ready', duration = $1, updated_at = now() WHERE id = $2`,
		newDuration, videoID,
	); err != nil {
		log.Printf("trim: failed to update status for %s: %v", videoID, err)
		return
	}

	GenerateThumbnail(ctx, db, storage, videoID, fileKey, thumbnailKey)
	if err := EnqueueTranscription(ctx, db, videoID); err != nil {
		log.Printf("trim: failed to enqueue transcription for %s: %v", videoID, err)
	}
	log.Printf("trim: completed for video %s", videoID)
}
