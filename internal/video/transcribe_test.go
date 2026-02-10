package video

import (
	"os"
	"testing"
)

func TestTranscriptFileKey(t *testing.T) {
	key := transcriptFileKey("user1", "abc123")
	expected := "recordings/user1/abc123.vtt"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestParseTimestampToSeconds(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"00:00:05,000", 5.0},
		{"00:01:30,500", 90.5},
		{"01:00:00,000", 3600.0},
		{"00:00:00.000", 0.0},
		{"", 0.0},
		{"invalid", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseTimestampToSeconds(tt.input)
			if result != tt.expected {
				t.Errorf("parseTimestampToSeconds(%q) = %f, want %f", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseWhisperJSON(t *testing.T) {
	content := `{
  "transcription": [
    {"timestamps": {"from": "00:00:00,000", "to": "00:00:03,500"}, "text": " Hello world"},
    {"timestamps": {"from": "00:00:03,500", "to": "00:00:07,000"}, "text": " This is a test"},
    {"timestamps": {"from": "00:00:07,000", "to": "00:00:07,500"}, "text": " "}
  ]
}`

	tmpFile, err := os.CreateTemp("", "whisper-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = tmpFile.Close()

	segments, err := parseWhisperJSON(tmpFile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}

	if segments[0].Start != 0.0 {
		t.Errorf("segment[0].Start = %f, want 0.0", segments[0].Start)
	}
	if segments[0].End != 3.5 {
		t.Errorf("segment[0].End = %f, want 3.5", segments[0].End)
	}
	if segments[0].Text != "Hello world" {
		t.Errorf("segment[0].Text = %q, want %q", segments[0].Text, "Hello world")
	}

	if segments[1].Start != 3.5 {
		t.Errorf("segment[1].Start = %f, want 3.5", segments[1].Start)
	}
	if segments[1].End != 7.0 {
		t.Errorf("segment[1].End = %f, want 7.0", segments[1].End)
	}
	if segments[1].Text != "This is a test" {
		t.Errorf("segment[1].Text = %q, want %q", segments[1].Text, "This is a test")
	}
}

func TestParseWhisperJSON_InvalidFile(t *testing.T) {
	_, err := parseWhisperJSON("/nonexistent/whisper-output.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestParseWhisperJSON_InvalidJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "whisper-bad-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString("not valid json{{{"); err != nil {
		t.Fatal(err)
	}
	_ = tmpFile.Close()

	_, err = parseWhisperJSON(tmpFile.Name())
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestWhisperModelPath(t *testing.T) {
	defaultPath := whisperModelPath()
	if defaultPath != "/models/ggml-small.bin" {
		t.Errorf("expected default path %q, got %q", "/models/ggml-small.bin", defaultPath)
	}

	t.Setenv("WHISPER_MODEL_PATH", "/custom/model.bin")

	customPath := whisperModelPath()
	if customPath != "/custom/model.bin" {
		t.Errorf("expected custom path %q, got %q", "/custom/model.bin", customPath)
	}
}
