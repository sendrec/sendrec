package video

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/sendrec/sendrec/internal/database"
)

func thumbnailFileKey(userID, shareToken string) string {
	return fmt.Sprintf("recordings/%s/%s.jpg", userID, shareToken)
}

func extractFrameAt(inputPath, outputPath string, seekSeconds int) error {
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-ss", fmt.Sprintf("%d", seekSeconds),
		"-frames:v", "1",
		"-vf", "scale=640:-1",
		"-q:v", "5",
		"-y",
		outputPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, string(output))
	}
	return nil
}

func extractFrame(inputPath, outputPath string) error {
	return extractFrameAt(inputPath, outputPath, 2)
}

func GenerateThumbnail(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, fileKey, thumbnailKey string) {
	tmpVideo, err := os.CreateTemp("", "sendrec-thumb-*.webm")
	if err != nil {
		log.Printf("thumbnail: failed to create temp video file: %v", err)
		return
	}
	tmpVideoPath := tmpVideo.Name()
	_ = tmpVideo.Close()
	defer func() { _ = os.Remove(tmpVideoPath) }()

	if err := storage.DownloadToFile(ctx, fileKey, tmpVideoPath); err != nil {
		log.Printf("thumbnail: failed to download video %s: %v", videoID, err)
		return
	}

	tmpThumb, err := os.CreateTemp("", "sendrec-thumb-*.jpg")
	if err != nil {
		log.Printf("thumbnail: failed to create temp thumbnail file: %v", err)
		return
	}
	tmpThumbPath := tmpThumb.Name()
	_ = tmpThumb.Close()
	defer func() { _ = os.Remove(tmpThumbPath) }()

	if err := extractFrame(tmpVideoPath, tmpThumbPath); err != nil {
		log.Printf("thumbnail: ffmpeg failed for video %s: %v", videoID, err)
		return
	}

	// If -ss 2 produced a 0-byte file (video shorter than 2s), retry at the start
	if info, err := os.Stat(tmpThumbPath); err == nil && info.Size() == 0 {
		log.Printf("thumbnail: video %s too short for seek=2, retrying at seek=0", videoID)
		if err := extractFrameAt(tmpVideoPath, tmpThumbPath, 0); err != nil {
			log.Printf("thumbnail: ffmpeg retry failed for video %s: %v", videoID, err)
			return
		}
	}

	// Skip upload if thumbnail is still empty
	if info, err := os.Stat(tmpThumbPath); err != nil || info.Size() == 0 {
		log.Printf("thumbnail: no frame extracted for video %s, skipping", videoID)
		return
	}

	if err := storage.UploadFile(ctx, thumbnailKey, tmpThumbPath, "image/jpeg"); err != nil {
		log.Printf("thumbnail: failed to upload thumbnail for video %s: %v", videoID, err)
		return
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET thumbnail_key = $1, updated_at = now() WHERE id = $2`,
		thumbnailKey, videoID,
	); err != nil {
		log.Printf("thumbnail: failed to update thumbnail_key for video %s: %v", videoID, err)
	}
}
