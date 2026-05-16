package video

import (
	"strings"
	"testing"
)

func TestNewTranscriberFromEnv_DefaultsToLocal(t *testing.T) {
	t.Setenv("TRANSCRIPTION_PROVIDER", "")
	tr, err := NewTranscriberFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.Name() != "local-whisper" {
		t.Errorf("default provider should be local-whisper, got %q", tr.Name())
	}
}

func TestNewTranscriberFromEnv_OpenAIRequiresKey(t *testing.T) {
	t.Setenv("TRANSCRIPTION_PROVIDER", "openai")
	t.Setenv("TRANSCRIPTION_API_KEY", "")
	_, err := NewTranscriberFromEnv()
	if err == nil {
		t.Fatal("expected error when API key missing")
	}
	if !strings.Contains(err.Error(), "TRANSCRIPTION_API_KEY") {
		t.Errorf("error should mention missing key: %v", err)
	}
}

func TestNewTranscriberFromEnv_OpenAIWithKey(t *testing.T) {
	t.Setenv("TRANSCRIPTION_PROVIDER", "openai")
	t.Setenv("TRANSCRIPTION_API_KEY", "k")
	t.Setenv("TRANSCRIPTION_MODEL", "whisper-large-v3")
	tr, err := NewTranscriberFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(tr.Name(), "whisper-large-v3") {
		t.Errorf("provider name should reflect model, got %q", tr.Name())
	}
}

func TestNewTranscriberFromEnv_DeepgramRequiresKey(t *testing.T) {
	t.Setenv("TRANSCRIPTION_PROVIDER", "deepgram")
	t.Setenv("TRANSCRIPTION_API_KEY", "")
	_, err := NewTranscriberFromEnv()
	if err == nil {
		t.Fatal("expected error when API key missing")
	}
}

func TestNewTranscriberFromEnv_UnknownProvider(t *testing.T) {
	t.Setenv("TRANSCRIPTION_PROVIDER", "magic")
	_, err := NewTranscriberFromEnv()
	if err == nil {
		t.Fatal("expected error on unknown provider")
	}
}
