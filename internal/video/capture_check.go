package video

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
)

// A capture can stall without the browser noticing: the frames keep arriving and
// the picture inside them never changes. macOS does exactly this when a screen
// capture freezes while Presenter Overlay keeps compositing the camera on top,
// and the recording that reaches us is a full length video of one still image.
// The recorder cannot see that case — no track is muted — so it is caught here.
//
// Three samples across the recording are enough to tell, and cost three seeks
// rather than a full decode.
var frameSamplePoints = []float64{0.1, 0.5, 0.9}

const (
	// Samples are compared as 64x64 grayscale: small enough that re-encoding
	// noise averages out, large enough that a moving cursor still shows.
	frameSampleSize = 64

	// Mean absolute difference per pixel below which two samples are the same
	// picture. Measured on real recordings: 0.1 for a frozen capture, 6.4 for a
	// live one, so anything near 1 separates them with room on both sides.
	frozenFrameTolerance = 1.0

	// Below this there is too little recording to judge, and a short clip of
	// something genuinely motionless is not worth accusing.
	minFrozenCheckSeconds = 5
)

// buildFrameSampleArgs takes one frame at a point in the file, downscaled to
// grayscale, on stdout. -ss goes before -i so ffmpeg seeks instead of decoding
// from the start for every sample.
func buildFrameSampleArgs(inputPath string, at float64) []string {
	return []string{
		"-v", "error",
		"-ss", strconv.FormatFloat(at, 'f', 3, 64),
		"-i", inputPath,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:%d,format=gray", frameSampleSize, frameSampleSize),
		"-f", "rawvideo",
		"-",
	}
}

// Package-level var so tests can reach the comparison without an ffmpeg binary.
//
// An empty sample with no error is ffmpeg reporting that there is no frame at
// that point: the video stream ends before the recording does.
var sampleGrayFrame = func(ctx context.Context, inputPath string, at float64) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", buildFrameSampleArgs(inputPath, at)...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg sample at %.3fs: %w", at, err)
	}
	return output, nil
}

// framesLookFrozen reports whether every sample shows the same picture. It needs
// at least two comparable samples; anything else is no verdict rather than a
// frozen one.
func framesLookFrozen(frames [][]byte) bool {
	if len(frames) < 2 {
		return false
	}
	for _, frame := range frames {
		if len(frame) == 0 || len(frame) != len(frames[0]) {
			return false
		}
	}

	for i := 1; i < len(frames); i++ {
		if meanAbsDiff(frames[0], frames[i]) >= frozenFrameTolerance {
			return false
		}
	}
	return true
}

func meanAbsDiff(a, b []byte) float64 {
	var total int
	for i := range a {
		diff := int(a[i]) - int(b[i])
		if diff < 0 {
			diff = -diff
		}
		total += diff
	}
	return float64(total) / float64(len(a))
}

// captureLooksStalled samples the recording and reports whether the video stopped
// carrying content while the audio ran on — either because the picture never
// changes, or because the video stream ends before the recording does.
//
// Sampling failures return false: accusing a good recording is worse than
// missing a bad one.
func captureLooksStalled(ctx context.Context, inputPath string, durationSeconds float64) bool {
	if durationSeconds < minFrozenCheckSeconds {
		return false
	}

	frames := make([][]byte, 0, len(frameSamplePoints))
	for _, point := range frameSamplePoints {
		frame, err := sampleGrayFrame(ctx, inputPath, durationSeconds*point)
		if err != nil {
			return false
		}
		// No frame this far into the recording: the video ran out early, which is
		// the truncated-capture case and needs no comparison.
		if len(frame) == 0 {
			return true
		}
		frames = append(frames, frame)
	}
	return framesLookFrozen(frames)
}
