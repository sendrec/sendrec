package video

import (
	"slices"
	"strings"
	"testing"
)

// flagValue reads a flag's value by key rather than position, so the tests stay
// honest when the argument list grows.
func flagValue(t *testing.T, args []string, flag string) string {
	t.Helper()
	i := slices.Index(args, flag)
	if i == -1 || i+1 >= len(args) {
		t.Fatalf("no %s in %v", flag, args)
	}
	return args[i+1]
}

// Every libx264 encode in the app runs in the pod that serves HTTP, so peak
// resident memory is a availability concern, not a tuning detail. Measured on a
// 1080p30 source: ~620 MB with the defaults, ~350 MB with these.
func TestX264MemoryParams_BoundsTheKnownCosts(t *testing.T) {
	got := strings.Split(flagValue(t, x264MemoryParams(), "-x264-params"), ":")

	for _, want := range []string{"rc-lookahead=10", "ref=1", "bframes=0"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
}

// Left alone, x264 scales threads with the visible CPU count and each thread
// holds frame buffers, so an uncapped encoder allocates more on a bigger node.
// Without this the memory bound holds on a 4-core box and fails on a 32-core one.
func TestX264MemoryParams_CapsThreads(t *testing.T) {
	i := slices.Index(x264MemoryParams(), "-threads")
	if i == -1 {
		t.Fatal("thread count must be capped; x264 otherwise scales it with the node's CPU count")
	}
	if got := x264MemoryParams()[i+1]; got != "4" {
		t.Errorf("thread cap = %q, want 4", got)
	}
}

// ffmpeg applies options to the output that follows them, so anything after the
// output URL is a trailing option it parses and discards with a warning nobody
// reads. A bound that lands there is worse than no bound: the tests pass, the
// flag is present in the command line, and the encoder runs unconstrained.
func TestBuildersPlaceX264ParamsBeforeTheOutput(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		output string
	}{
		{"transcode", buildTranscodeArgs("in.webm", "out.mp4", ""), "out.mp4"},
		{"normalize", buildNormalizeArgs("in.mp4", "out.mp4", ""), "out.mp4"},
		{"trim mp4", buildTrimArgs("in.mp4", "out.mp4", "video/mp4", 1, 5), "out.mp4"},
		{"trim quicktime", buildTrimArgs("in.mov", "out.mov", "video/quicktime", 1, 5), "out.mov"},
		{"composite mp4", buildCompositeArgs("s.mp4", "w.mp4", "out.mp4", "video/mp4"), "out.mp4"},
		{"composite quicktime", buildCompositeArgs("s.mov", "w.mov", "out.mov", "video/quicktime"), "out.mov"},
		{"remove-segments mp4", buildRemoveSegmentsArgs("in.mp4", "out.mp4", "video/mp4", []segmentRange{{Start: 1, End: 2}}, true), "out.mp4"},
		{"remove-segments silent", buildRemoveSegmentsArgs("in.mp4", "out.mp4", "video/mp4", []segmentRange{{Start: 1, End: 2}}, false), "out.mp4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paramsAt := slices.Index(tc.args, "-x264-params")
			outputAt := slices.Index(tc.args, tc.output)
			if paramsAt == -1 {
				t.Fatalf("no -x264-params in %v", tc.args)
			}
			if outputAt == -1 {
				t.Fatalf("output %q missing from %v", tc.output, tc.args)
			}
			if paramsAt > outputAt {
				t.Errorf("-x264-params at %d comes after the output at %d; ffmpeg discards trailing options", paramsAt, outputAt)
			}
		})
	}
}

func TestBuildersBoundX264Memory(t *testing.T) {
	want := flagValue(t, x264MemoryParams(), "-x264-params")

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
			if got := flagValue(t, tc.args, "-x264-params"); got != want {
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
