package video

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Three review rounds on #205 found bounds that were present and inert. The
// semaphore is only worth anything if every encoding path actually acquires
// it, so prove that per path: hold the single slot, call the path with a short
// deadline, and require it to die waiting rather than reach ffmpeg.
func TestEveryEncodePathAcquiresASlot(t *testing.T) {
	original := ffmpegEncoders.Load()
	ffmpegEncoders.Store(newEncoderSlots(1))
	t.Cleanup(func() { ffmpegEncoders.Store(original) })

	release, err := encoders().acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()

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
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			err := tc.run(ctx)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("did not wait for a slot; got %v — the path is not gated", err)
			}
		})
	}
}
