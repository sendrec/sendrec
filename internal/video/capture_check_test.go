package video

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pashagolub/pgxmock/v5"
)

func TestCaptureEndedEarly(t *testing.T) {
	tests := []struct {
		name      string
		video     float64
		recording float64
		want      bool
	}{
		// The recordings that prompted this, measured with ffprobe.
		{"video stops at 15s of a 141s recording", 15.53, 141.1, true},
		{"video stops at 12s of a 133s recording", 12.53, 132.76, true},
		{"healthy recording", 103.35, 103.42, false},
		{"frozen picture, full length streams", 236.0, 236.01, false},

		// Trailing loss is normal: the last audio packet outlasts the last frame.
		{"video a shade short", 99.2, 100, false},
		{"video a tenth short", 89, 100, true},

		// Unknown durations are no verdict. WebM from MediaRecorder carries none.
		{"no durations at all", 0, 0, false},
		{"video duration unknown", 0, 141.1, false},
		{"recording length unknown", 15.53, 0, false},

		{"too short to judge", 0.2, 4, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := captureEndedEarly(tt.video, tt.recording); got != tt.want {
				t.Errorf("captureEndedEarly(%v, %v) = %v, want %v", tt.video, tt.recording, got, tt.want)
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

func TestRecordingLength(t *testing.T) {
	// A screen recording only opens the microphone when system audio is on, so a
	// silent one has no audio track to measure against — the client's own
	// measurement is what is left.
	if got := recordingLength(streamDurations{Video: 15.53}, 141); got != 141 {
		t.Errorf("want the client duration for a silent recording, got %v", got)
	}
	if got := recordingLength(streamDurations{Video: 15.53, Audio: 141.1}, 9); got != 141.1 {
		t.Errorf("want the audio duration where there is one, got %v", got)
	}
}

func TestCaptureWarningFor(t *testing.T) {
	warning := captureWarningFor(15.53, 141.1)

	// The numbers are the evidence; without them this is just an accusation.
	if !strings.Contains(warning, "141s") || !strings.Contains(warning, "16s") {
		t.Errorf("want both durations in the warning, got %q", warning)
	}
}

func TestCheckCapture(t *testing.T) {
	original := probeStreamDurations
	t.Cleanup(func() { probeStreamDurations = original })

	t.Run("stores the warning when the video died early", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		probeStreamDurations = func(context.Context, string) (streamDurations, error) {
			return streamDurations{Video: 15.53, Audio: 141.1}, nil
		}
		mock.ExpectExec(`UPDATE videos SET capture_warning`).
			WithArgs("video-1", pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		CheckCapture(context.Background(), mock, "video-1", "/tmp/in.mp4", 141)

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	})

	// Written on every pass, not only on failures: this is what clears a warning
	// once a trim or a re-encode has fixed the recording.
	t.Run("clears the warning on a healthy recording", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		probeStreamDurations = func(context.Context, string) (streamDurations, error) {
			return streamDurations{Video: 103.35, Audio: 103.42}, nil
		}
		mock.ExpectExec(`UPDATE videos SET capture_warning`).
			WithArgs("video-1", (*string)(nil)).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		CheckCapture(context.Background(), mock, "video-1", "/tmp/in.mp4", 103)

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	})

	t.Run("leaves the row alone when the file cannot be probed", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		probeStreamDurations = func(context.Context, string) (streamDurations, error) {
			return streamDurations{}, errors.New("ffprobe exploded")
		}

		CheckCapture(context.Background(), mock, "video-1", "/tmp/in.mp4", 141)

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	})
}
