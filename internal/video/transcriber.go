package video

import (
	"context"
	"errors"
)

// ErrNoAudio is returned by a Transcriber when the audio file has no audible content.
var ErrNoAudio = errors.New("audio has no usable speech")

// Transcriber turns a 16kHz mono WAV at audioPath into a list of segments.
// language is either an ISO code (e.g. "en", "ro") or "auto".
type Transcriber interface {
	// Name returns a short identifier used in logs and the /api/health response.
	Name() string
	// Available reports whether the transcriber can actually run (binaries present, API key set, etc.).
	Available() bool
	// Transcribe blocks until the audio at audioPath has been transcribed.
	// Returns ErrNoAudio when the provider reports an empty / silent recording.
	Transcribe(ctx context.Context, audioPath, language string) ([]TranscriptSegment, error)
}
