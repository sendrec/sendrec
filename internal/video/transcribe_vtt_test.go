package video

import "testing"

func TestFormatVTTTimestamp(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "00:00:00.000"},
		{1.5, "00:00:01.500"},
		{61.25, "00:01:01.250"},
		{3725.125, "01:02:05.125"},
		{-5, "00:00:00.000"},
		{59.9999, "00:01:00.000"},
		{3599.9999, "01:00:00.000"},
		{59.9994, "00:00:59.999"},
	}
	for _, tc := range tests {
		got := formatVTTTimestamp(tc.in)
		if got != tc.want {
			t.Errorf("formatVTTTimestamp(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSegmentsToVTT(t *testing.T) {
	segments := []TranscriptSegment{
		{Start: 0, End: 1.5, Text: "Hello world"},
		{Start: 1.5, End: 3.2, Text: "How are you"},
	}
	got := segmentsToVTT(segments)
	want := "WEBVTT\n\n00:00:00.000 --> 00:00:01.500\nHello world\n\n00:00:01.500 --> 00:00:03.200\nHow are you\n\n"
	if got != want {
		t.Errorf("segmentsToVTT mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}
