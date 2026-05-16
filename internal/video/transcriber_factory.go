package video

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// NewTranscriberFromEnv builds a Transcriber from the TRANSCRIPTION_PROVIDER
// environment variable. Supported providers:
//   - "" or "local": runs whisper-cli locally (default).
//   - "openai":      OpenAI-compatible /v1/audio/transcriptions endpoint.
//                    Works with OpenAI, Groq, Scaleway, self-hosted Faster-Whisper, etc.
//                    Reads TRANSCRIPTION_API_URL, TRANSCRIPTION_API_KEY, TRANSCRIPTION_MODEL.
//   - "deepgram":    Deepgram /v1/listen.
//                    Reads TRANSCRIPTION_API_KEY, TRANSCRIPTION_MODEL.
func NewTranscriberFromEnv() (Transcriber, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("TRANSCRIPTION_PROVIDER")))
	switch provider {
	case "", "local":
		return newLocalWhisper(), nil
	case "openai":
		key := os.Getenv("TRANSCRIPTION_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("TRANSCRIPTION_PROVIDER=openai requires TRANSCRIPTION_API_KEY")
		}
		return newOpenAIWhisper(
			os.Getenv("TRANSCRIPTION_API_URL"),
			key,
			os.Getenv("TRANSCRIPTION_MODEL"),
			parseTimeoutEnv("TRANSCRIPTION_TIMEOUT_SECONDS"),
		), nil
	case "deepgram":
		key := os.Getenv("TRANSCRIPTION_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("TRANSCRIPTION_PROVIDER=deepgram requires TRANSCRIPTION_API_KEY")
		}
		return newDeepgram(
			key,
			os.Getenv("TRANSCRIPTION_MODEL"),
			parseTimeoutEnv("TRANSCRIPTION_TIMEOUT_SECONDS"),
		), nil
	default:
		return nil, fmt.Errorf("unknown TRANSCRIPTION_PROVIDER %q (expected one of: local, openai, deepgram)", provider)
	}
}

func parseTimeoutEnv(name string) time.Duration {
	v := os.Getenv(name)
	if v == "" {
		return 0
	}
	d, err := time.ParseDuration(v + "s")
	if err != nil {
		return 0
	}
	return d
}
