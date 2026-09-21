package video

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func flatFrame(value byte) []byte {
	return bytes.Repeat([]byte{value}, frameSampleSize*frameSampleSize)
}

// A frozen capture still re-encodes, so its samples differ by encoder noise
// rather than by content. The real files measured ~0.1 per pixel; a live
// recording measured ~6.4.
func noisyFrame(value byte, noisePixels int) []byte {
	frame := flatFrame(value)
	for i := 0; i < noisePixels; i++ {
		frame[i] = value + 1
	}
	return frame
}

func TestFramesLookFrozen(t *testing.T) {
	tests := []struct {
		name   string
		frames [][]byte
		want   bool
	}{
		{"identical samples", [][]byte{flatFrame(120), flatFrame(120), flatFrame(120)}, true},
		{"encoder noise only", [][]byte{flatFrame(120), noisyFrame(120, 200), noisyFrame(120, 300)}, true},
		{"live recording", [][]byte{flatFrame(40), flatFrame(120), flatFrame(200)}, false},
		{"one sample moves late", [][]byte{flatFrame(120), flatFrame(120), flatFrame(200)}, false},
		{"too few samples to judge", [][]byte{flatFrame(120)}, false},
		{"no samples", nil, false},
		{"mismatched sizes", [][]byte{flatFrame(120), {1, 2, 3}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := framesLookFrozen(tt.frames); got != tt.want {
				t.Errorf("framesLookFrozen() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildFrameSampleArgs(t *testing.T) {
	args := buildFrameSampleArgs("/tmp/in.mp4", 12.5)
	joined := strings.Join(args, " ")

	// -ss before -i is the fast seek; after it, ffmpeg decodes from the start of
	// the file every time, which on a four minute recording is the whole point
	// of sampling rather than decoding.
	ssIdx, inputIdx := indexOf(args, "-ss"), indexOf(args, "-i")
	if ssIdx == -1 || inputIdx == -1 || ssIdx > inputIdx {
		t.Errorf("-ss must precede -i, got %q", joined)
	}
	if !strings.Contains(joined, "12.500") {
		t.Errorf("sample point missing from args: %q", joined)
	}
	if !strings.Contains(joined, "scale=64:64,format=gray") {
		t.Errorf("samples must be downscaled to grayscale: %q", joined)
	}
	if args[len(args)-1] != "-" {
		t.Errorf("samples must be written to stdout, got %q", joined)
	}
}

func indexOf(args []string, want string) int {
	for i, arg := range args {
		if arg == want {
			return i
		}
	}
	return -1
}

func TestCaptureLooksFrozen(t *testing.T) {
	original := sampleGrayFrame
	t.Cleanup(func() { sampleGrayFrame = original })

	t.Run("frozen capture", func(t *testing.T) {
		sampleGrayFrame = func(context.Context, string, float64) ([]byte, error) {
			return flatFrame(120), nil
		}
		if !captureLooksStalled(context.Background(), "/tmp/in.mp4", 236) {
			t.Error("want frozen")
		}
	})

	t.Run("no video stream at all", func(t *testing.T) {
		sampleGrayFrame = func(context.Context, string, float64) ([]byte, error) {
			return nil, nil
		}
		if !captureLooksStalled(context.Background(), "/tmp/in.mp4", 132) {
			t.Error("want stalled")
		}
	})

	t.Run("live capture", func(t *testing.T) {
		var call int
		sampleGrayFrame = func(context.Context, string, float64) ([]byte, error) {
			call++
			return flatFrame(byte(40 * call)), nil
		}
		if captureLooksStalled(context.Background(), "/tmp/in.mp4", 236) {
			t.Error("want live")
		}
	})

	t.Run("recording too short to judge", func(t *testing.T) {
		sampleGrayFrame = func(context.Context, string, float64) ([]byte, error) {
			t.Error("must not sample a recording this short")
			return nil, nil
		}
		if captureLooksStalled(context.Background(), "/tmp/in.mp4", 3) {
			t.Error("want no verdict")
		}
	})

	// ffmpeg exits cleanly with no frame when the point asked for is past the end
	// of the video stream — which is the other shape this failure takes: a video
	// track that stops seconds in while the audio runs for minutes.
	t.Run("video stream ends before the recording does", func(t *testing.T) {
		sampleGrayFrame = func(_ context.Context, _ string, at float64) ([]byte, error) {
			if at > 20 {
				return nil, nil
			}
			return flatFrame(120), nil
		}
		if !captureLooksStalled(context.Background(), "/tmp/in.mp4", 132) {
			t.Error("want stalled")
		}
	})

	// A sampling failure is not evidence of a stalled capture, and accusing a
	// good recording is worse than missing a bad one.
	t.Run("sampling fails", func(t *testing.T) {
		sampleGrayFrame = func(context.Context, string, float64) ([]byte, error) {
			return nil, errors.New("ffmpeg exploded")
		}
		if captureLooksStalled(context.Background(), "/tmp/in.mp4", 236) {
			t.Error("want no verdict")
		}
	})
}
