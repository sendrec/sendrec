package video

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

// Every ffmpeg call used exec.Command, so the 10 minute contexts the callers
// construct never reached the process: a pathological input could hold an
// encoder open indefinitely, and a job whose deadline passed kept its ~300 MB
// allocation alive while the caller had already moved on.
//
// A cancelled context has to stop the command before it starts. That is the
// cheapest observable proof the context is wired through at all.
func TestFFmpegRespectsContextCancellation(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed; this asserts wiring, not encoding")
	}

	cancelled := func() context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}

	dir := t.TempDir()

	for _, tc := range []struct {
		name string
		run  func(context.Context) error
	}{
		{"transcode", func(ctx context.Context) error { return transcodeToMP4(ctx, "in.webm", dir+"/o.mp4", "") }},
		{"normalize", func(ctx context.Context) error { return transcodeToIOSCompatible(ctx, "in.mp4", dir+"/o.mp4", "") }},
		{"trim", func(ctx context.Context) error { return trimVideo(ctx, "in.mp4", dir+"/o.mp4", "video/mp4", 0, 1) }},
		{"remove segments", func(ctx context.Context) error {
			return removeSegmentsFromVideo(ctx, "in.mp4", dir+"/o.mp4", "video/mp4", []segmentRange{{Start: 0, End: 1}}, false)
		}},
		{"composite", func(ctx context.Context) error {
			_, err := compositeOverlay(ctx, "s.mp4", "w.mp4", dir+"/o.mp4", "video/mp4")
			return err
		}},
		{"silence detect", func(ctx context.Context) error { _, err := detectSilence(ctx, "in.mp4", -30, 0.5); return err }},
		{"thumbnail", func(ctx context.Context) error { return extractFrameAt(ctx, "in.mp4", dir+"/o.jpg", 0) }},
		{"audio extract", func(ctx context.Context) error { return extractAudioAt(ctx, "in.mp4", dir+"/o.wav") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run(cancelled())
			if err == nil {
				t.Fatal("cancelled context produced no error; ffmpeg is not bound to it")
			}
			if !errors.Is(err, context.Canceled) {
				t.Errorf("error does not wrap context.Canceled, so cancellation did not reach ffmpeg: %v", err)
			}
		})
	}
}
