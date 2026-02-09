package video

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/sendrec/sendrec/internal/database"
)

func compositeOverlay(screenPath, webcamPath, outputPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", screenPath,
		"-i", webcamPath,
		"-filter_complex", "[1:v]scale=240:-1,pad=iw+8:ih+8:(ow-iw)/2:(oh-ih)/2:color=black@0.3[pip];[0:v][pip]overlay=W-w-20:H-h-20",
		"-map", "0:a?",
		"-c:a", "copy",
		"-c:v", "libvpx-vp9",
		"-y",
		outputPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg composite: %w: %s", err, string(output))
	}
	return nil
}

func CompositeWithWebcam(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, screenKey, webcamKey, thumbnailKey string) {
	log.Printf("composite: starting webcam overlay for video %s", videoID)

	setReadyFallback := func() {
		if _, err := db.Exec(ctx,
			`UPDATE videos SET status = 'ready', webcam_key = NULL, updated_at = now() WHERE id = $1`,
			videoID,
		); err != nil {
			log.Printf("composite: failed to set fallback ready status for %s: %v", videoID, err)
		}
	}

	tmpScreen, err := os.CreateTemp("", "sendrec-composite-screen-*.webm")
	if err != nil {
		log.Printf("composite: failed to create temp screen file: %v", err)
		setReadyFallback()
		return
	}
	tmpScreenPath := tmpScreen.Name()
	_ = tmpScreen.Close()
	defer func() { _ = os.Remove(tmpScreenPath) }()

	if err := storage.DownloadToFile(ctx, screenKey, tmpScreenPath); err != nil {
		log.Printf("composite: failed to download screen %s: %v", videoID, err)
		setReadyFallback()
		return
	}

	tmpWebcam, err := os.CreateTemp("", "sendrec-composite-webcam-*.webm")
	if err != nil {
		log.Printf("composite: failed to create temp webcam file: %v", err)
		setReadyFallback()
		return
	}
	tmpWebcamPath := tmpWebcam.Name()
	_ = tmpWebcam.Close()
	defer func() { _ = os.Remove(tmpWebcamPath) }()

	if err := storage.DownloadToFile(ctx, webcamKey, tmpWebcamPath); err != nil {
		log.Printf("composite: failed to download webcam %s: %v", videoID, err)
		setReadyFallback()
		return
	}

	tmpOutput, err := os.CreateTemp("", "sendrec-composite-output-*.webm")
	if err != nil {
		log.Printf("composite: failed to create temp output file: %v", err)
		setReadyFallback()
		return
	}
	tmpOutputPath := tmpOutput.Name()
	_ = tmpOutput.Close()
	defer func() { _ = os.Remove(tmpOutputPath) }()

	if err := compositeOverlay(tmpScreenPath, tmpWebcamPath, tmpOutputPath); err != nil {
		log.Printf("composite: ffmpeg failed for %s: %v", videoID, err)
		setReadyFallback()
		return
	}

	if err := storage.UploadFile(ctx, screenKey, tmpOutputPath, "video/webm"); err != nil {
		log.Printf("composite: failed to upload composited video %s: %v", videoID, err)
		setReadyFallback()
		return
	}

	if err := storage.DeleteObject(ctx, webcamKey); err != nil {
		log.Printf("composite: failed to delete webcam file %s: %v", webcamKey, err)
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET status = 'ready', webcam_key = NULL, updated_at = now() WHERE id = $1`,
		videoID,
	); err != nil {
		log.Printf("composite: failed to update status for %s: %v", videoID, err)
		return
	}

	GenerateThumbnail(ctx, db, storage, videoID, screenKey, thumbnailKey)
	log.Printf("composite: completed for video %s", videoID)
}
