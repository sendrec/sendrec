package video

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"

	"github.com/sendrec/sendrec/internal/database"
)

// A capture can die while the recording carries on: the browser stops delivering
// frames and the microphone keeps going, so the file that arrives has a video
// stream ending seconds in and an audio stream running for minutes. Two of the
// recordings that prompted this carried 15.5s and 12.5s of video against 141.1s
// and 132.8s of audio, and were published as ready.
//
// Comparing the picture itself was tried and dropped. A recording of a static
// document with a moving cursor is indistinguishable from a frozen capture at
// any sampling resolution — measured on real files, a cursor-only recording
// scored 0.0 mean difference per pixel while a genuinely frozen capture scored
// 0.42, because re-encoding noise outweighs a cursor. Guessing there means
// telling people their good recordings are broken, so only the stream lengths,
// which are a fact about the file, are checked.
const (
	// Below this there is too little recording for the ratio to mean anything.
	minCaptureCheckSeconds = 5

	// The share of the recording the video has to cover. Trailing loss is normal
	// — the last audio packet outlasts the last frame — so this leaves room for
	// it without leaving room for a capture that died.
	minVideoCoverage = 0.9
)

type streamDurations struct {
	Video float64
	Audio float64
}

func buildStreamDurationArgs(inputPath string) []string {
	return []string{
		"-v", "error",
		"-show_entries", "stream=codec_type,duration",
		"-of", "json",
		inputPath,
	}
}

// Package-level var so tests can reach the comparison without an ffprobe binary.
var probeStreamDurations = func(ctx context.Context, inputPath string) (streamDurations, error) {
	cmd := exec.CommandContext(ctx, "ffprobe", buildStreamDurationArgs(inputPath)...)
	output, err := cmd.Output()
	if err != nil {
		return streamDurations{}, fmt.Errorf("ffprobe streams: %w", err)
	}

	var parsed struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Duration  string `json:"duration"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &parsed); err != nil {
		return streamDurations{}, fmt.Errorf("ffprobe streams parse: %w", err)
	}

	// A container can leave a stream's duration out — MediaRecorder's WebM does,
	// having no Segment Duration at all — and an absent duration parses to zero,
	// which reads as no verdict rather than as a stream of length zero.
	var durations streamDurations
	for _, stream := range parsed.Streams {
		seconds, _ := strconv.ParseFloat(stream.Duration, 64)
		switch stream.CodecType {
		case "video":
			if seconds > durations.Video {
				durations.Video = seconds
			}
		case "audio":
			if seconds > durations.Audio {
				durations.Audio = seconds
			}
		}
	}
	return durations, nil
}

// captureEndedEarly reports whether the video stopped well before the recording
// did. Unknown durations are no verdict: accusing a good recording is worse than
// missing a bad one.
func captureEndedEarly(videoSeconds, recordingSeconds float64) bool {
	if recordingSeconds < minCaptureCheckSeconds || videoSeconds <= 0 {
		return false
	}
	return videoSeconds < recordingSeconds*minVideoCoverage
}

// recordingLength is what the video is measured against: the audio, or where a
// recording has no audio track at all — the screen recorder only opens the
// microphone when system audio is on — the length the client measured.
func recordingLength(d streamDurations, clientSeconds int) float64 {
	if d.Audio > 0 {
		return d.Audio
	}
	return float64(clientSeconds)
}

// CheckCapture stores or clears a video's capture warning from a local copy of
// the file. Every job that writes the file calls it with the copy it already
// has: probing from here costs nothing extra, and re-checking after a trim or a
// transcode is what clears a warning the edit fixed.
func CheckCapture(ctx context.Context, db database.DBTX, videoID, path string, clientSeconds int) {
	durations, err := probeStreamDurations(ctx, path)
	if err != nil {
		slog.Warn("capture-check: stream durations unavailable", "video_id", videoID, "error", err)
		return
	}

	length := recordingLength(durations, clientSeconds)
	var warning *string
	if captureEndedEarly(durations.Video, length) {
		message := captureWarningFor(durations.Video, length)
		warning = &message
		slog.Warn("capture-check: capture ended early", "video_id", videoID,
			"video_seconds", durations.Video, "recording_seconds", length)
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET capture_warning = $2, updated_at = now() WHERE id = $1`,
		videoID, warning,
	); err != nil {
		slog.Error("capture-check: failed to store verdict", "video_id", videoID, "error", err)
	}
}

// captureWarningFor is what the owner is told, on the video page. It names the
// numbers because they are the evidence, and it blames a browser only
// conditionally: the same handler takes uploaded files, which no browser here
// recorded.
func captureWarningFor(videoSeconds, recordingSeconds float64) string {
	return fmt.Sprintf(
		"This video runs %.0fs but its picture stops after %.0fs. If you recorded it here, the browser stopped capturing partway through — Safari does that whenever its window is not in front, while Chrome and Edge keep going.",
		recordingSeconds, videoSeconds,
	)
}
