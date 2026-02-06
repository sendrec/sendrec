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

func extractFrame(inputPath, outputPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-ss", "2",
		"-frames:v", "1",
		"-vf", "scale=640:360:force_original_aspect_ratio=decrease,pad=640:360:(ow-iw)/2:(oh-ih)/2",
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

func GenerateThumbnail(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, fileKey, thumbnailKey string) {
	tmpVideo, err := os.CreateTemp("", "sendrec-thumb-*.webm")
	if err != nil {
		log.Printf("thumbnail: failed to create temp video file: %v", err)
		return
	}
	tmpVideoPath := tmpVideo.Name()
	tmpVideo.Close()
	defer os.Remove(tmpVideoPath)

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
	tmpThumb.Close()
	defer os.Remove(tmpThumbPath)

	if err := extractFrame(tmpVideoPath, tmpThumbPath); err != nil {
		log.Printf("thumbnail: ffmpeg failed for video %s: %v", videoID, err)
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
