package video

import (
	"testing"
)

func TestParseSilenceDetectOutput(t *testing.T) {
	stderr := `[silencedetect @ 0x7f8b1c000b80] silence_start: 3.504
[silencedetect @ 0x7f8b1c000b80] silence_end: 5.200 | silence_duration: 1.696
[silencedetect @ 0x7f8b1c000b80] silence_start: 12.800
[silencedetect @ 0x7f8b1c000b80] silence_end: 15.100 | silence_duration: 2.300
`
	segments := parseSilenceDetectOutput(stderr)

	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}
	if segments[0].Start != 3.504 || segments[0].End != 5.200 {
		t.Errorf("segment 0: expected {3.504, 5.200}, got {%f, %f}", segments[0].Start, segments[0].End)
	}
	if segments[1].Start != 12.800 || segments[1].End != 15.100 {
		t.Errorf("segment 1: expected {12.800, 15.100}, got {%f, %f}", segments[1].Start, segments[1].End)
	}
}

func TestParseSilenceDetectOutput_Empty(t *testing.T) {
	stderr := `size=N/A time=00:01:30.00 bitrate=N/A speed=120x
video:0kB audio:12345kB subtitle:0kB other streams:0kB global headers:0kB muxing overhead: unknown
`
	segments := parseSilenceDetectOutput(stderr)

	if len(segments) != 0 {
		t.Errorf("expected 0 segments, got %d", len(segments))
	}
}

func TestParseSilenceDetectOutput_UnpairedStart(t *testing.T) {
	stderr := `[silencedetect @ 0x7f8b1c000b80] silence_start: 3.504
[silencedetect @ 0x7f8b1c000b80] silence_end: 5.200 | silence_duration: 1.696
[silencedetect @ 0x7f8b1c000b80] silence_start: 88.700
`
	segments := parseSilenceDetectOutput(stderr)

	if len(segments) != 1 {
		t.Fatalf("expected 1 segment (unpaired start discarded), got %d", len(segments))
	}
	if segments[0].Start != 3.504 || segments[0].End != 5.200 {
		t.Errorf("segment 0: expected {3.504, 5.200}, got {%f, %f}", segments[0].Start, segments[0].End)
	}
}

func TestParseSilenceDetectOutput_OnlyUnpairedStart(t *testing.T) {
	stderr := "[silencedetect @ 0x7f8b1c000b80] silence_start: 55.000\n"
	segments := parseSilenceDetectOutput(stderr)
	if len(segments) != 0 {
		t.Fatalf("expected 0 segments (orphan start discarded), got %d", len(segments))
	}
}
