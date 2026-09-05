package video

import (
	"slices"
	"strings"
	"testing"
)

func x264ParamsOf(t *testing.T, args []string) string {
	t.Helper()
	i := slices.Index(args, "-x264-params")
	if i == -1 || i+1 >= len(args) {
		t.Fatalf("no -x264-params in %v", args)
	}
	return args[i+1]
}

// Every libx264 encode in the app runs in the pod that serves HTTP, so peak
// resident memory is a availability concern, not a tuning detail. Measured on a
// 1080p30 source: ~620 MB with the defaults, ~350 MB with these.
func TestX264MemoryParams_BoundsTheKnownCosts(t *testing.T) {
	got := strings.Split(x264MemoryParams()[1], ":")

	for _, want := range []string{"rc-lookahead=10", "ref=1", "bframes=0"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
}

// -threads 1 was measured too: another 106 MB off, but double the wall time,
// and every one of these jobs runs under a 10 minute context.
func TestX264MemoryParams_LeavesThreadingAlone(t *testing.T) {
	if slices.Contains(x264MemoryParams(), "-threads") {
		t.Error("thread count must stay at the ffmpeg default; capping it risks the 10 minute job timeout")
	}
}

func TestBuildersBoundX264Memory(t *testing.T) {
	want := x264MemoryParams()[1]

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"transcode", buildTranscodeArgs("in.webm", "out.mp4", "")},
		{"normalize", buildNormalizeArgs("in.mp4", "out.mp4", "")},
		{"trim mp4", buildTrimArgs("in.mp4", "out.mp4", "video/mp4", 1, 5)},
		{"composite mp4", buildCompositeArgs("s.mp4", "w.mp4", "out.mp4", "video/mp4")},
		{"remove-segments mp4", buildRemoveSegmentsArgs("in.mp4", "out.mp4", "video/mp4", []segmentRange{{Start: 1, End: 2}}, true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := x264ParamsOf(t, tc.args); got != want {
				t.Errorf("x264 params = %q, want %q", got, want)
			}
		})
	}
}

// The params are libx264-only; libvpx-vp9 rejects the flag outright.
func TestVP9BuildersOmitX264Params(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"trim webm", buildTrimArgs("in.webm", "out.webm", "video/webm", 1, 5)},
		{"composite webm", buildCompositeArgs("s.webm", "w.webm", "out.webm", "video/webm")},
		{"remove-segments webm", buildRemoveSegmentsArgs("in.webm", "out.webm", "video/webm", []segmentRange{{Start: 1, End: 2}}, true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if slices.Contains(tc.args, "-x264-params") {
				t.Errorf("-x264-params passed to a libvpx-vp9 encode: %v", tc.args)
			}
		})
	}
}
