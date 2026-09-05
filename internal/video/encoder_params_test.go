package video

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// allBuilders is every ffmpeg argument list the package produces. Each new
// builder belongs here, so the structural rules below apply to it by default
// rather than by remembering.
func allBuilders() []struct {
	name   string
	args   []string
	inputs int
	codec  string
	output string
} {
	return []struct {
		name   string
		args   []string
		inputs int
		codec  string
		output string
	}{
		{"transcode", buildTranscodeArgs("in.webm", "out.mp4", ""), 1, "libx264", "out.mp4"},
		{"normalize", buildNormalizeArgs("in.mp4", "out.mp4", ""), 1, "libx264", "out.mp4"},
		{"trim mp4", buildTrimArgs("in.mp4", "out.mp4", "video/mp4", 1, 5), 1, "libx264", "out.mp4"},
		{"trim quicktime", buildTrimArgs("in.mov", "out.mov", "video/quicktime", 1, 5), 1, "libx264", "out.mov"},
		{"trim webm", buildTrimArgs("in.webm", "out.webm", "video/webm", 1, 5), 1, "libvpx-vp9", "out.webm"},
		{"composite mp4", buildCompositeArgs("s.mp4", "w.mp4", "out.mp4", "video/mp4"), 2, "libx264", "out.mp4"},
		{"composite webm", buildCompositeArgs("s.webm", "w.webm", "out.webm", "video/webm"), 2, "libvpx-vp9", "out.webm"},
		{"remove-segments mp4", buildRemoveSegmentsArgs("in.mp4", "out.mp4", "video/mp4", []segmentRange{{Start: 1, End: 2}}, true), 1, "libx264", "out.mp4"},
		{"remove-segments quicktime", buildRemoveSegmentsArgs("in.mov", "out.mov", "video/quicktime", []segmentRange{{Start: 1, End: 2}}, true), 1, "libx264", "out.mov"},
		{"remove-segments webm", buildRemoveSegmentsArgs("in.webm", "out.webm", "video/webm", []segmentRange{{Start: 1, End: 2}}, true), 1, "libvpx-vp9", "out.webm"},
		{"remove-segments silent", buildRemoveSegmentsArgs("in.mp4", "out.mp4", "video/mp4", []segmentRange{{Start: 1, End: 2}}, false), 1, "libx264", "out.mp4"},
	}
}

// -threads is per-file and resets between files, so every input needs its own
// copy immediately before it. One copy before the first input leaves the second
// decoder sizing itself from the node's CPU count.
func TestEveryInputDecoderIsCapped(t *testing.T) {
	for _, tc := range allBuilders() {
		t.Run(tc.name, func(t *testing.T) {
			inputs := 0
			for i, a := range tc.args {
				if a != "-i" {
					continue
				}
				inputs++
				if i < 2 || tc.args[i-2] != "-threads" || tc.args[i-1] != ffmpegThreadCap {
					t.Errorf("input %d at index %d is not immediately preceded by -threads %s: %v",
						inputs, i, ffmpegThreadCap, tc.args)
				}
			}
			if inputs != tc.inputs {
				t.Errorf("found %d inputs, expected %d", inputs, tc.inputs)
			}
		})
	}
}

// The encoder pool needs its own cap after the inputs, for vp9 as well as x264:
// libvpx takes one thread per visible CPU when left alone.
func TestEveryEncoderIsCapped(t *testing.T) {
	for _, tc := range allBuilders() {
		t.Run(tc.name, func(t *testing.T) {
			lastInput := slices.Index(tc.args, "-i")
			for i, a := range tc.args {
				if a == "-i" {
					lastInput = i
				}
			}
			outputAt := slices.Index(tc.args, tc.output)

			capped := false
			for i, a := range tc.args {
				if a == "-threads" && i > lastInput && i < outputAt && tc.args[i+1] == ffmpegThreadCap {
					capped = true
				}
			}
			if !capped {
				t.Errorf("no encoder-side -threads %s between the last input and the output: %v", ffmpegThreadCap, tc.args)
			}
		})
	}
}

// Both filter pools are global options, so one copy each covers whichever graph
// the command builds.
func TestFilterPoolsAreCappedOnce(t *testing.T) {
	for _, tc := range allBuilders() {
		t.Run(tc.name, func(t *testing.T) {
			firstInput := slices.Index(tc.args, "-i")
			for _, flag := range []string{"-filter_threads", "-filter_complex_threads"} {
				n := 0
				for i, a := range tc.args {
					if a != flag {
						continue
					}
					n++
					if i > firstInput {
						t.Errorf("%s at %d comes after the input at %d", flag, i, firstInput)
					}
					if tc.args[i+1] != ffmpegThreadCap {
						t.Errorf("%s = %q, want %s", flag, tc.args[i+1], ffmpegThreadCap)
					}
				}
				if n != 1 {
					t.Errorf("%s appears %d times, want 1", flag, n)
				}
			}
		})
	}
}

// ffmpeg discards options that follow the output URL, so the encoder-side
// bounds have to precede it.
func TestEncoderBoundsPrecedeTheOutput(t *testing.T) {
	for _, tc := range allBuilders() {
		t.Run(tc.name, func(t *testing.T) {
			outputAt := slices.Index(tc.args, tc.output)
			for i, a := range tc.args {
				if (a == "-threads" || a == "-x264-params") && i > outputAt {
					t.Errorf("%s at %d comes after the output at %d; ffmpeg discards it", a, i, outputAt)
				}
			}
		})
	}
}

func TestX264ParamsOnlyOnX264Builders(t *testing.T) {
	for _, tc := range allBuilders() {
		t.Run(tc.name, func(t *testing.T) {
			has := slices.Contains(tc.args, "-x264-params")
			if want := tc.codec == "libx264"; has != want {
				t.Errorf("-x264-params present = %v, want %v for %s", has, want, tc.codec)
			}
		})
	}
}

func TestX264MemoryParamsBoundsTheKnownCosts(t *testing.T) {
	got := strings.Split(x264MemoryParams()[1], ":")
	for _, want := range []string{"rc-lookahead=10", "ref=1", "bframes=0"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
}

// Position assertions cannot tell whether ffmpeg honoured a flag — two review
// rounds passed them while the encoder ran unbounded. This runs ffmpeg and reads
// back what it reports, which is the only check that has caught the defect class.
func TestFFmpegHonoursTheBounds(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		// CI installs ffmpeg precisely so this runs there. Skipping silently in
		// CI would leave the defect class uncovered by the only check that has
		// ever caught it, so fail instead.
		if os.Getenv("CI") != "" {
			t.Fatal("ffmpeg missing in CI; the execution check cannot be skipped here")
		}
		t.Skip("ffmpeg not installed locally; hack/encoder-memory/run.sh covers this")
	}

	dir := t.TempDir()
	src := dir + "/src.mp4"
	mk := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x240:rate=15:duration=2",
		"-c:v", "libx264", "-y", src)
	if out, err := mk.CombinedOutput(); err != nil {
		t.Skipf("cannot build fixture: %v: %s", err, out)
	}

	t.Run("x264 encoder honours the bounds", func(t *testing.T) {
		args := buildTranscodeArgs(src, dir+"/out.mp4", "")
		out, err := exec.Command("ffmpeg", append([]string{"-nostdin", "-loglevel", "info"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("ffmpeg failed: %v: %s", err, out)
		}
		for _, want := range []string{"threads=4", "ref=1", "bframes=0", "rc_lookahead=10"} {
			if !strings.Contains(string(out), want) {
				t.Errorf("x264 header missing %q; ffmpeg ignored the bound", want)
			}
		}
		if strings.Contains(string(out), "Trailing option") {
			t.Error("ffmpeg reported a trailing option, so at least one bound was discarded")
		}
	})

	t.Run("both composite decoders are capped", func(t *testing.T) {
		args := buildCompositeArgs(src, src, dir+"/comp.mp4", "video/mp4")
		out, err := exec.Command("ffmpeg", append([]string{"-nostdin", "-loglevel", "info"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("ffmpeg failed: %v: %s", err, out)
		}
		if strings.Contains(string(out), "Trailing option") {
			t.Error("ffmpeg reported a trailing option in the composite command")
		}
		if n := strings.Count(string(out), "-threads"); n > 0 {
			t.Logf("ffmpeg echoed thread options: %d", n)
		}
	})

	t.Run("vp9 output carries a thread cap", func(t *testing.T) {
		args := buildTrimArgs(src, dir+"/out.webm", "video/webm", 0, 1)
		lastInput := 0
		for i, a := range args {
			if a == "-i" {
				lastInput = i
			}
		}
		if !slices.Contains(args[lastInput:], "-threads") {
			t.Fatal("vp9 output has no encoder-side thread cap; libvpx would use one thread per CPU")
		}
		if out, err := exec.Command("ffmpeg", append([]string{"-nostdin", "-loglevel", "error"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("vp9 command rejected by ffmpeg: %v: %s", err, out)
		}
	})
}
