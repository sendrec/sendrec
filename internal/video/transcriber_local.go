package video

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type localWhisper struct{}

func newLocalWhisper() *localWhisper { return &localWhisper{} }

func (l *localWhisper) Name() string { return "local-whisper" }

func (l *localWhisper) Available() bool {
	if _, err := exec.LookPath("whisper-cli"); err != nil {
		return false
	}
	if _, err := os.Stat(whisperModelPath()); err != nil {
		return false
	}
	return true
}

func (l *localWhisper) Transcribe(ctx context.Context, audioPath, language string) ([]TranscriptSegment, error) {
	tmpOutput, err := os.CreateTemp("", "sendrec-whisper-out-*")
	if err != nil {
		return nil, fmt.Errorf("create temp output: %w", err)
	}
	outputPrefix := tmpOutput.Name()
	_ = tmpOutput.Close()
	_ = os.Remove(outputPrefix)
	defer func() {
		_ = os.Remove(outputPrefix + ".vtt")
		_ = os.Remove(outputPrefix + ".json")
	}()

	cmd := exec.CommandContext(ctx, "whisper-cli",
		"-m", whisperModelPath(),
		"-f", audioPath,
		"--output-json",
		"-of", outputPrefix,
		"-t", "2",
		"-l", language,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("whisper-cli: %w: %s", err, string(output))
	}

	return parseWhisperJSON(outputPrefix + ".json")
}

func whisperModelPath() string {
	if p := os.Getenv("WHISPER_MODEL_PATH"); p != "" {
		return p
	}
	return "/models/ggml-small.bin"
}

func parseTimestampToSeconds(ts string) float64 {
	if ts == "" {
		return 0.0
	}

	normalized := strings.Replace(ts, ",", ".", 1)

	parts := strings.Split(normalized, ":")
	if len(parts) != 3 {
		return 0.0
	}

	hours, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0.0
	}

	minutes, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0.0
	}

	seconds, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0.0
	}

	return hours*3600 + minutes*60 + seconds
}

type whisperJSON struct {
	Transcription []whisperSegment `json:"transcription"`
}

type whisperSegment struct {
	Timestamps whisperTimestamps `json:"timestamps"`
	Text       string            `json:"text"`
}

type whisperTimestamps struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func parseWhisperJSON(jsonPath string) ([]TranscriptSegment, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("read whisper JSON: %w", err)
	}

	var result whisperJSON
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse whisper JSON: %w", err)
	}

	segments := make([]TranscriptSegment, 0)
	for _, seg := range result.Transcription {
		text := strings.TrimSpace(seg.Text)
		if text == "" {
			continue
		}
		segments = append(segments, TranscriptSegment{
			Start: parseTimestampToSeconds(seg.Timestamps.From),
			End:   parseTimestampToSeconds(seg.Timestamps.To),
			Text:  text,
		})
	}

	return segments, nil
}
