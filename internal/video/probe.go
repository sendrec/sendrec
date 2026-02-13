package video

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sendrec/sendrec/internal/database"
)

func probeDuration(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, fileKey string) {
	log.Printf("probe: starting duration probe for video %s", videoID)

	tmpFile, err := os.CreateTemp("", "sendrec-probe-*")
	if err != nil {
		log.Printf("probe: failed to create temp file: %v", err)
		return
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := storage.DownloadToFile(ctx, fileKey, tmpPath); err != nil {
		log.Printf("probe: failed to download video %s: %v", videoID, err)
		return
	}

	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		tmpPath,
	)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("probe: ffprobe failed for %s: %v", videoID, err)
		return
	}

	durationStr := strings.TrimSpace(string(output))
	durationFloat, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		log.Printf("probe: failed to parse duration %q for %s: %v", durationStr, videoID, err)
		return
	}

	duration := int(durationFloat)
	if duration <= 0 {
		log.Printf("probe: invalid duration %d for %s", duration, videoID)
		return
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET duration = $1, updated_at = now() WHERE id = $2`,
		duration, videoID,
	); err != nil {
		log.Printf("probe: failed to update duration for %s: %v", videoID, err)
		return
	}

	log.Printf("probe: video %s duration is %ds", videoID, duration)
}
