package video

import (
	"slices"
	"testing"
)

func codecPair(args []string) (video, audio string) {
	if i := slices.Index(args, "-c:v"); i != -1 && i+1 < len(args) {
		video = args[i+1]
	}
	if i := slices.Index(args, "-c:a"); i != -1 && i+1 < len(args) {
		audio = args[i+1]
	}
	return
}

// AAC is not a legal codec in a WebM container: ffmpeg refuses at header write
// with "Only VP8 or VP9 or AV1 video and Vorbis or Opus audio ... are supported
// for WebM", so remove-segments could never succeed on a WebM with audio.
func TestBuildRemoveSegmentsArgs_WebMUsesOpus(t *testing.T) {
	args := buildRemoveSegmentsArgs("in.webm", "out.webm", "video/webm", []segmentRange{{Start: 1, End: 2}}, true)

	v, a := codecPair(args)
	if v != "libvpx-vp9" {
		t.Errorf("video codec = %q, want libvpx-vp9", v)
	}
	if a != "libopus" {
		t.Errorf("audio codec = %q, want libopus (aac is invalid in WebM)", a)
	}
}

func TestBuildRemoveSegmentsArgs_MP4UsesAAC(t *testing.T) {
	args := buildRemoveSegmentsArgs("in.mp4", "out.mp4", "video/mp4", []segmentRange{{Start: 1, End: 2}}, true)

	v, a := codecPair(args)
	if v != "libx264" {
		t.Errorf("video codec = %q, want libx264", v)
	}
	if a != "aac" {
		t.Errorf("audio codec = %q, want aac", a)
	}
}

// The container follows the source, so the codec pair has to follow it too —
// pairing a vp9 stream with an mp4 muxer or the reverse is the same class of bug.
func TestBuildRemoveSegmentsArgs_CodecPairMatchesContainer(t *testing.T) {
	for _, tc := range []struct{ contentType, wantVideo, wantAudio string }{
		{"video/mp4", "libx264", "aac"},
		{"video/quicktime", "libx264", "aac"},
		{"video/webm", "libvpx-vp9", "libopus"},
		{"", "libvpx-vp9", "libopus"},
	} {
		args := buildRemoveSegmentsArgs("in", "out", tc.contentType, []segmentRange{{Start: 1, End: 2}}, true)
		v, a := codecPair(args)
		if v != tc.wantVideo || a != tc.wantAudio {
			t.Errorf("contentType %q: got %s/%s, want %s/%s", tc.contentType, v, a, tc.wantVideo, tc.wantAudio)
		}
	}
}

// Silent sources take the -an branch, which has no audio codec to get wrong.
func TestBuildRemoveSegmentsArgs_NoAudioStream(t *testing.T) {
	args := buildRemoveSegmentsArgs("in.webm", "out.webm", "video/webm", []segmentRange{{Start: 1, End: 2}}, false)

	if !slices.Contains(args, "-an") {
		t.Error("expected -an when the source has no audio stream")
	}
	if slices.Contains(args, "-c:a") {
		t.Error("expected no audio codec when the source has no audio stream")
	}
}
