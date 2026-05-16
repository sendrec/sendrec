package video

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func writeTempWav(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "sendrec-test-*.wav")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	return f.Name()
}

func TestOpenAIWhisper_Transcribe(t *testing.T) {
	var receivedAuth, receivedContentType, receivedModel, receivedLanguage, receivedFormat string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedContentType = r.Header.Get("Content-Type")

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		receivedModel = r.MultipartForm.Value["model"][0]
		receivedFormat = r.MultipartForm.Value["response_format"][0]
		if langs := r.MultipartForm.Value["language"]; len(langs) > 0 {
			receivedLanguage = langs[0]
		}

		resp := openaiTranscriptionResponse{
			Text: "Hello world",
			Segments: []openaiTranscriptionSegment{
				{Start: 0, End: 1.5, Text: "Hello"},
				{Start: 1.5, End: 2.8, Text: " world"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tr := newOpenAIWhisper(server.URL, "test-key", "whisper-1", 0)
	audio := writeTempWav(t, "fake audio bytes")

	segments, err := tr.Transcribe(context.Background(), audio, "ro")
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}

	if receivedAuth != "Bearer test-key" {
		t.Errorf("Authorization = %q, want Bearer test-key", receivedAuth)
	}
	if !strings.HasPrefix(receivedContentType, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want multipart/form-data", receivedContentType)
	}
	if receivedModel != "whisper-1" {
		t.Errorf("model = %q, want whisper-1", receivedModel)
	}
	if receivedFormat != "verbose_json" {
		t.Errorf("response_format = %q, want verbose_json", receivedFormat)
	}
	if receivedLanguage != "ro" {
		t.Errorf("language = %q, want ro", receivedLanguage)
	}

	if len(segments) != 2 {
		t.Fatalf("segments = %d, want 2", len(segments))
	}
	if segments[0].Text != "Hello" || segments[1].Text != "world" {
		t.Errorf("segment text mismatch: %+v", segments)
	}
	if segments[1].End != 2.8 {
		t.Errorf("segment[1].End = %v, want 2.8", segments[1].End)
	}
}

func TestOpenAIWhisper_AutoLanguageOmitted(t *testing.T) {
	var hasLanguageField bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, hasLanguageField = r.MultipartForm.Value["language"]
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"text":"hi","segments":[{"start":0,"end":1,"text":"hi"}]}`)
	}))
	defer server.Close()

	tr := newOpenAIWhisper(server.URL, "key", "whisper-1", 0)
	audio := writeTempWav(t, "audio")

	_, err := tr.Transcribe(context.Background(), audio, "auto")
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
	if hasLanguageField {
		t.Errorf("language field should be omitted when language is 'auto'")
	}
}

func TestOpenAIWhisper_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"invalid key"}`)
	}))
	defer server.Close()

	tr := newOpenAIWhisper(server.URL, "bad", "whisper-1", 0)
	audio := writeTempWav(t, "audio")

	_, err := tr.Transcribe(context.Background(), audio, "en")
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention status code, got: %v", err)
	}
}

func TestOpenAIWhisper_EmptyResponseReturnsErrNoAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"text":"","segments":[]}`)
	}))
	defer server.Close()

	tr := newOpenAIWhisper(server.URL, "k", "whisper-1", 0)
	audio := writeTempWav(t, "audio")

	_, err := tr.Transcribe(context.Background(), audio, "en")
	if err != ErrNoAudio {
		t.Errorf("expected ErrNoAudio, got: %v", err)
	}
}

func TestOpenAIWhisper_NotAvailableWithoutKey(t *testing.T) {
	tr := newOpenAIWhisper("", "", "", 0)
	if tr.Available() {
		t.Error("openai whisper should not be available without API key")
	}
}
