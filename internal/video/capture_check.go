package video

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
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
func captureEndedEarly(d streamDurations) bool {
	if d.Audio < minCaptureCheckSeconds || d.Video <= 0 {
		return false
	}
	return d.Video < d.Audio*minVideoCoverage
}

// captureWarningFor is what the owner is told, on the video page. It names the
// numbers because they are the evidence, and it does not assume a browser: the
// recorders catch the stalls their browser admits to, so what reaches here is
// whatever failed silently.
func captureWarningFor(d streamDurations) string {
	return fmt.Sprintf(
		"This recording has %.0fs of sound but its picture stops after %.0fs — the browser stopped capturing partway through. Please record it again. Safari does this whenever its window is not in front; Chrome and Edge keep capturing.",
		d.Audio, d.Video,
	)
}
