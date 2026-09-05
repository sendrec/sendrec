package video

import (
	"context"
	"log/slog"
	"time"
)

// DefaultEncoderConcurrency is how many ffmpeg encodes may run at once when
// MAX_CONCURRENT_ENCODES is unset.
//
// One, because the deployment is sized for one. A 1080p encode peaks around
// 300 MB and the chart requests 512Mi, so a second concurrent encode wants
// memory the pod never reserved. Raise it together with the memory request,
// not on its own.
const DefaultEncoderConcurrency = 1

// encoderSlots bounds how many ffmpeg encodes run concurrently in this process.
//
// Every encode runs in the process that serves HTTP, so concurrency multiplies
// peak memory directly: two 1080p edits at once want roughly twice what one
// does, and the pod is sized for one. Without this the memory bounds in
// x264MemoryParams and ffmpegPipelineThreads cap a single encode while nothing
// caps how many there are — a per-encode ceiling with no process-wide one.
//
// Queueing is the intended behaviour. An edit that waits is slower; an edit that
// runs alongside three others takes the pod down with it.
type encoderSlots struct {
	tokens chan struct{}
}

func newEncoderSlots(limit int) *encoderSlots {
	// A limit below one would block every encode until its deadline, which is
	// worse than any value an operator could have meant by it.
	if limit < 1 {
		limit = 1
	}
	return &encoderSlots{tokens: make(chan struct{}, limit)}
}

// acquire blocks until a slot is free or ctx is done. The returned function
// returns the slot and must be called exactly once.
func (s *encoderSlots) acquire(ctx context.Context) (release func(), err error) {
	waited := time.Now()
	select {
	case s.tokens <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Queueing is expected under load; queueing for a long time means the pod is
	// undersized for its edit volume, which is invisible without this line.
	if held := time.Since(waited); held > 5*time.Second {
		slog.Info("encoder: waited for a free slot", "waited", held.String(), "limit", cap(s.tokens))
	}

	var once bool
	return func() {
		if once {
			return
		}
		once = true
		<-s.tokens
	}, nil
}

// ffmpegEncoders gates the five encoding paths. Probing, thumbnail extraction
// and audio extraction stay outside it: they are cheap enough that queueing them
// behind an encode would cost more than it saves.
var ffmpegEncoders = newEncoderSlots(DefaultEncoderConcurrency)

// SetEncoderConcurrency replaces the process-wide limit. Called once at startup
// from MAX_CONCURRENT_ENCODES, before any request is served.
func SetEncoderConcurrency(limit int) {
	ffmpegEncoders = newEncoderSlots(limit)
	slog.Info("encoder concurrency", "limit", cap(ffmpegEncoders.tokens))
}
