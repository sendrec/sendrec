package video

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sendrec/sendrec/internal/database"
)

const (
	iOSMaxWidth  = 1920
	iOSMaxHeight = 1080
	iOSMaxLevel  = 51
	iOSMaxFPS    = 60
)

type videoProperties struct {
	Width     int
	Height    int
	Level     int
	FrameRate float64
	CodecName string
}

func (p videoProperties) needsNormalization() bool {
	if p.CodecName != "h264" {
		return true
	}
	if p.Width > iOSMaxWidth || p.Height > iOSMaxHeight {
		return true
	}
	if p.Level > iOSMaxLevel {
		return true
	}
	if p.FrameRate > float64(iOSMaxFPS) {
		return true
	}
	return false
}

func parseFrameRate(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 {
		f, _ := strconv.ParseFloat(raw, 64)
		return f
	}
	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	den, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || den == 0 {
		return 0
	}
	return num / den
}

type ffprobeResult struct {
	Streams []struct {
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		Level      int    `json:"level"`
		RFrameRate string `json:"r_frame_rate"`
		CodecName  string `json:"codec_name"`
	} `json:"streams"`
}

func probeVideoProperties(inputPath string) (videoProperties, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,level,r_frame_rate,codec_name",
		"-of", "json",
		inputPath,
	)
	output, err := cmd.Output()
	if err != nil {
		return videoProperties{}, fmt.Errorf("ffprobe: %w", err)
	}

	var result ffprobeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return videoProperties{}, fmt.Errorf("ffprobe parse: %w", err)
	}

	if len(result.Streams) == 0 {
		return videoProperties{}, fmt.Errorf("ffprobe: no video streams found")
	}

	s := result.Streams[0]
	return videoProperties{
		Width:     s.Width,
		Height:    s.Height,
		Level:     s.Level,
		FrameRate: parseFrameRate(s.RFrameRate),
		CodecName: s.CodecName,
	}, nil
}

func transcodeToIOSCompatible(inputPath, outputPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
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
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg normalize: %w: %s", err, string(output))
	}
	return nil
}

func NormalizeVideoAsync(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, fileKey string) {
	slog.Info("normalize: starting", "video_id", videoID)

	tmpInput, err := os.CreateTemp("", "sendrec-normalize-in-*.mp4")
	if err != nil {
		slog.Error("normalize: failed to create temp input file", "error", err)
		return
	}
	tmpInputPath := tmpInput.Name()
	_ = tmpInput.Close()
	defer func() { _ = os.Remove(tmpInputPath) }()

	if err := storage.DownloadToFile(ctx, fileKey, tmpInputPath); err != nil {
		slog.Error("normalize: failed to download", "video_id", videoID, "error", err)
		return
	}

	props, err := probeVideoProperties(tmpInputPath)
	if err != nil {
		slog.Warn("normalize: probe failed, marking normalized", "video_id", videoID, "error", err)
		markIOSNormalized(ctx, db, videoID)
		return
	}

	if !props.needsNormalization() {
		slog.Info("normalize: already compatible", "video_id", videoID,
			"width", props.Width, "height", props.Height, "level", props.Level, "fps", props.FrameRate)
		markIOSNormalized(ctx, db, videoID)
		return
	}

	slog.Info("normalize: re-encoding", "video_id", videoID,
		"width", props.Width, "height", props.Height, "level", props.Level,
		"fps", props.FrameRate, "codec", props.CodecName)

	tmpOutput, err := os.CreateTemp("", "sendrec-normalize-out-*.mp4")
	if err != nil {
		slog.Error("normalize: failed to create temp output file", "error", err)
		return
	}
	tmpOutputPath := tmpOutput.Name()
	_ = tmpOutput.Close()
	defer func() { _ = os.Remove(tmpOutputPath) }()

	if err := transcodeToIOSCompatible(tmpInputPath, tmpOutputPath); err != nil {
		slog.Error("normalize: ffmpeg failed", "video_id", videoID, "error", err)
		return
	}

	info, err := os.Stat(tmpOutputPath)
	if err != nil {
		slog.Error("normalize: failed to stat output", "video_id", videoID, "error", err)
		return
	}
	newFileSize := info.Size()

	if err := storage.UploadFile(ctx, fileKey, tmpOutputPath, "video/mp4"); err != nil {
		slog.Error("normalize: failed to upload", "video_id", videoID, "error", err)
		return
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET file_size = $2, ios_normalized = true, updated_at = now() WHERE id = $1`,
		videoID, newFileSize,
	); err != nil {
		slog.Error("normalize: failed to update db", "video_id", videoID, "error", err)
		return
	}

	slog.Info("normalize: completed", "video_id", videoID, "new_size", newFileSize)
}

func markIOSNormalized(ctx context.Context, db database.DBTX, videoID string) {
	if _, err := db.Exec(ctx,
		`UPDATE videos SET ios_normalized = true, updated_at = now() WHERE id = $1`,
		videoID,
	); err != nil {
		slog.Error("normalize: failed to mark normalized", "video_id", videoID, "error", err)
	}
}
