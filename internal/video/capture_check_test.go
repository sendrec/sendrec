package video

import (
	"strings"
	"testing"
)

func TestCaptureEndedEarly(t *testing.T) {
	tests := []struct {
		name string
		d    streamDurations
		want bool
	}{
		// The recordings that prompted this, measured with ffprobe.
		{"video stops at 15s of a 141s recording", streamDurations{Video: 15.53, Audio: 141.1}, true},
		{"video stops at 12s of a 133s recording", streamDurations{Video: 12.53, Audio: 132.76}, true},
		{"healthy recording", streamDurations{Video: 103.35, Audio: 103.42}, false},
		{"frozen picture, full length streams", streamDurations{Video: 236.0, Audio: 236.01}, false},

		// Trailing loss is normal: the last audio packet outlasts the last frame.
		{"video a shade short", streamDurations{Video: 99.2, Audio: 100}, false},
		{"video a tenth short", streamDurations{Video: 89, Audio: 100}, true},

		// Unknown durations are no verdict. WebM from MediaRecorder carries none.
		{"no durations at all", streamDurations{}, false},
		{"video duration unknown", streamDurations{Video: 0, Audio: 141.1}, false},
		{"audio duration unknown", streamDurations{Video: 15.53, Audio: 0}, false},

		{"too short to judge", streamDurations{Video: 0.2, Audio: 4}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := captureEndedEarly(tt.d); got != tt.want {
				t.Errorf("captureEndedEarly(%+v) = %v, want %v", tt.d, got, tt.want)
			}
		})
	}
}

func TestBuildStreamDurationArgs(t *testing.T) {
	args := strings.Join(buildStreamDurationArgs("/tmp/in.mp4"), " ")

	// Per stream, not per format: the format duration is the longest stream,
	// which on exactly the broken files is the audio.
	if !strings.Contains(args, "stream=codec_type,duration") {
		t.Errorf("want per-stream durations, got %q", args)
	}
	if !strings.HasSuffix(args, "/tmp/in.mp4") {
		t.Errorf("want the input last, got %q", args)
	}
}

func TestCaptureWarningFor(t *testing.T) {
	warning := captureWarningFor(streamDurations{Video: 15.53, Audio: 141.1})

	// The numbers are the evidence; without them this is just an accusation.
	if !strings.Contains(warning, "141s") || !strings.Contains(warning, "16s") {
		t.Errorf("want both durations in the warning, got %q", warning)
	}
}
