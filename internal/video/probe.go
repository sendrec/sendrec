package video

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sendrec/sendrec/internal/database"
)

func probeDuration(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, fileKey string) {
	slog.Info("probe: starting duration probe", "video_id", videoID)

	tmpFile, err := os.CreateTemp("", "sendrec-probe-*")
	if err != nil {
		slog.Error("probe: failed to create temp file", "error", err)
		return
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := storage.DownloadToFile(ctx, fileKey, tmpPath); err != nil {
		slog.Error("probe: failed to download video", "video_id", videoID, "error", err)
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
		slog.Error("probe: ffprobe failed", "video_id", videoID, "error", err)
		return
	}

	durationStr := strings.TrimSpace(string(output))
	durationFloat, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		slog.Error("probe: failed to parse duration", "video_id", videoID, "raw_duration", durationStr, "error", err)
		return
	}

	duration := int(durationFloat)
	if duration <= 0 {
		slog.Warn("probe: invalid duration", "video_id", videoID, "duration", duration)
		return
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET duration = $1, updated_at = now() WHERE id = $2`,
		duration, videoID,
	); err != nil {
		slog.Error("probe: failed to update duration", "video_id", videoID, "error", err)
		return
	}

	slog.Info("probe: video duration detected", "video_id", videoID, "duration", duration)
}
