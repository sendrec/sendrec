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
	slog.Info("probe: starting probe", "video_id", videoID)

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

	// Before the duration: a container can leave the format duration out — WebM
	// from MediaRecorder does — and that must not cost the recording its check.
	var warning *string
	if durations, err := probeStreamDurations(ctx, tmpPath); err != nil {
		slog.Warn("probe: stream durations unavailable", "video_id", videoID, "error", err)
	} else if captureEndedEarly(durations) {
		message := captureWarningFor(durations)
		warning = &message
		slog.Warn("probe: capture ended early", "video_id", videoID,
			"video_seconds", durations.Video, "audio_seconds", durations.Audio)
	}

	duration := probeFormatDuration(ctx, videoID, tmpPath)
	if duration <= 0 && warning == nil {
		// Nothing learned and nothing to report.
		return
	}

	// The client measures its own recording and is right about it; overwriting
	// that here would only round it down to whole seconds. Fill it in when it is
	// missing, and leave it alone otherwise — including when this probe could not
	// read one, which arrives here as zero.
	if _, err := db.Exec(ctx,
		`UPDATE videos
		 SET duration = CASE WHEN duration = 0 AND $1 > 0 THEN $1 ELSE duration END,
		     capture_warning = $3,
		     updated_at = now()
		 WHERE id = $2`,
		duration, videoID, warning,
	); err != nil {
		slog.Error("probe: failed to update video", "video_id", videoID, "error", err)
		return
	}

	slog.Info("probe: video probed", "video_id", videoID, "duration", duration, "capture_warning", warning != nil)
}

// probeFormatDuration returns the recording's length in whole seconds, or zero
// where the container does not carry one.
func probeFormatDuration(ctx context.Context, videoID, path string) int {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		slog.Error("probe: ffprobe failed", "video_id", videoID, "error", err)
		return 0
	}

	raw := strings.TrimSpace(string(output))
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		slog.Warn("probe: no usable duration", "video_id", videoID, "raw_duration", raw)
		return 0
	}
	if seconds <= 0 {
		slog.Warn("probe: invalid duration", "video_id", videoID, "duration", seconds)
		return 0
	}
	return int(seconds)
}
