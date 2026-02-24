package video

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/sendrec/sendrec/internal/database"
)

func probeVideoInfo(path string) (frames int, info string, err error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-count_frames",
		"-show_entries", "stream=nb_read_frames,start_time,codec_name,width,height",
		"-of", "default=noprint_wrappers=1",
		path,
	)
	output, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		return 0, "", fmt.Errorf("ffprobe: %w: %s", cmdErr, string(output))
	}
	info = strings.TrimSpace(string(output))
	for _, line := range strings.Split(info, "\n") {
		if strings.HasPrefix(line, "nb_read_frames=") {
			_, _ = fmt.Sscanf(strings.TrimPrefix(line, "nb_read_frames="), "%d", &frames)
		}
	}
	return frames, info, nil
}

func compositeOverlay(screenPath, webcamPath, outputPath, contentType string) (string, error) {
	// PiP filter: scale webcam, add border, normalize timestamps.
	// setpts=PTS-STARTPTS normalizes webcam timestamps to start at 0.
	pipSetup := "[1:v]setpts=PTS-STARTPTS,scale=240:-1,pad=iw+8:ih+8:(ow-iw)/2:(oh-ih)/2:color=black@0.3[pip]"

	var args []string
	if contentType == "video/mp4" || contentType == "video/quicktime" {
		// Scale the screen DOWN first, then overlay PiP on top.
		// This ensures the PiP is sized relative to the output resolution,
		// not the original (which may be high-DPI, e.g. 3242x2626).
		filterComplex := "[0:v]scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2[screen];" +
			pipSetup + ";[screen][pip]overlay=W-w-20:H-h-20[vout]"
		args = []string{
			"-i", screenPath,
			"-i", webcamPath,
			"-filter_complex", filterComplex,
			"-map", "[vout]",
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
		// For WebM, also scale down high-DPI screens before overlay
		filterComplex := "[0:v]scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2[screen];" +
			pipSetup + ";[screen][pip]overlay=W-w-20:H-h-20[vout]"
		args = []string{
			"-i", screenPath,
			"-i", webcamPath,
			"-filter_complex", filterComplex,
			"-map", "[vout]",
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
		return string(output), fmt.Errorf("ffmpeg composite: %w: %s", err, string(output))
	}
	return string(output), nil
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

	// Log file sizes for debugging
	screenInfo, _ := os.Stat(tmpScreenPath)
	webcamInfo, _ := os.Stat(tmpWebcamPath)
	screenSize := int64(0)
	webcamSize := int64(0)
	if screenInfo != nil {
		screenSize = screenInfo.Size()
	}
	if webcamInfo != nil {
		webcamSize = webcamInfo.Size()
	}
	slog.Info("composite: files downloaded", "video_id", videoID, "screen_bytes", screenSize, "webcam_bytes", webcamSize)

	// Verify both inputs have video frames before compositing
	screenFrames, screenProbeInfo, screenProbeErr := probeVideoInfo(tmpScreenPath)
	if screenProbeErr != nil || screenFrames == 0 {
		slog.Warn("composite: screen has no video frames, skipping overlay", "video_id", videoID, "screen_bytes", screenSize, "probe_error", screenProbeErr)
		setReadyFallback()
		return
	}
	slog.Info("composite: screen validated", "video_id", videoID, "screen_frames", screenFrames, "screen_info", screenProbeInfo)

	webcamFrames, webcamProbeInfo, probeErr := probeVideoInfo(tmpWebcamPath)
	if probeErr != nil {
		slog.Error("composite: webcam probe failed", "video_id", videoID, "error", probeErr)
		setReadyFallback()
		return
	}
	if webcamFrames == 0 {
		slog.Warn("composite: webcam has no video frames, skipping overlay", "video_id", videoID, "webcam_bytes", webcamSize)
		setReadyFallback()
		return
	}
	slog.Info("composite: webcam validated", "video_id", videoID, "webcam_frames", webcamFrames, "webcam_info", webcamProbeInfo)

	tmpOutput, err := os.CreateTemp("", "sendrec-composite-output-*"+ext)
	if err != nil {
		slog.Error("composite: failed to create temp output file", "error", err)
		setReadyFallback()
		return
	}
	tmpOutputPath := tmpOutput.Name()
	_ = tmpOutput.Close()
	defer func() { _ = os.Remove(tmpOutputPath) }()

	ffmpegOutput, err := compositeOverlay(tmpScreenPath, tmpWebcamPath, tmpOutputPath, contentType)
	if err != nil {
		slog.Error("composite: ffmpeg failed", "video_id", videoID, "error", err, "ffmpeg_output", ffmpegOutput)
		setReadyFallback()
		return
	}
	slog.Info("composite: ffmpeg succeeded", "video_id", videoID, "ffmpeg_output", ffmpegOutput)

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
