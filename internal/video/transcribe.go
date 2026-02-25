package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sendrec/sendrec/internal/database"
)

type TranscriptSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

func transcriptFileKey(userID, shareToken string) string {
	return fmt.Sprintf("recordings/%s/%s.vtt", userID, shareToken)
}

func whisperModelPath() string {
	if p := os.Getenv("WHISPER_MODEL_PATH"); p != "" {
		return p
	}
	return "/models/ggml-small.bin"
}

func isTranscriptionAvailable() bool {
	if os.Getenv("TRANSCRIPTION_ENABLED") != "true" {
		return false
	}
	if _, err := exec.LookPath("whisper-cli"); err != nil {
		return false
	}
	if _, err := os.Stat(whisperModelPath()); err != nil {
		return false
	}
	return true
}

var errNoAudio = fmt.Errorf("video has no audio stream")

func hasAudioStream(inputPath string) bool {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "a",
		"-show_entries", "stream=codec_type",
		"-of", "csv=p=0",
		inputPath,
	)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

func extractAudio(inputPath, outputPath string) error {
	if !hasAudioStream(inputPath) {
		return errNoAudio
	}
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-ar", "16000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		"-y",
		outputPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg audio extraction: %w: %s", err, string(output))
	}
	return nil
}

func runWhisper(audioPath, outputPrefix, language string) error {
	cmd := exec.Command("whisper-cli",
		"-m", whisperModelPath(),
		"-f", audioPath,
		"--output-vtt",
		"--output-json",
		"-of", outputPrefix,
		"-t", "2",
		"-l", language,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("whisper: %w: %s", err, string(output))
	}
	return nil
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

func processTranscription(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, fileKey, userID, shareToken, language string, aiEnabled bool) {
	if !isTranscriptionAvailable() {
		slog.Warn("transcribe: transcription not available, marking as failed", "video_id", videoID)
		if _, err := db.Exec(ctx,
			`UPDATE videos SET transcript_status = 'failed', transcript_started_at = NULL, updated_at = now() WHERE id = $1`,
			videoID,
		); err != nil {
			slog.Error("transcribe: failed to set failed status", "video_id", videoID, "error", err)
		}
		return
	}

	slog.Info("transcribe: starting", "video_id", videoID, "language", language)

	setFailed := func() {
		if _, err := db.Exec(ctx,
			`UPDATE videos SET transcript_status = 'failed', transcript_started_at = NULL, updated_at = now() WHERE id = $1`,
			videoID,
		); err != nil {
			slog.Error("transcribe: failed to set failed status", "video_id", videoID, "error", err)
		}
	}

	tmpVideo, err := os.CreateTemp("", "sendrec-transcribe-*.webm")
	if err != nil {
		slog.Error("transcribe: failed to create temp video file", "error", err)
		setFailed()
		return
	}
	tmpVideoPath := tmpVideo.Name()
	_ = tmpVideo.Close()
	defer func() { _ = os.Remove(tmpVideoPath) }()

	if err := storage.DownloadToFile(ctx, fileKey, tmpVideoPath); err != nil {
		slog.Error("transcribe: failed to download video", "video_id", videoID, "error", err)
		setFailed()
		return
	}

	tmpAudio, err := os.CreateTemp("", "sendrec-transcribe-*.wav")
	if err != nil {
		slog.Error("transcribe: failed to create temp audio file", "error", err)
		setFailed()
		return
	}
	tmpAudioPath := tmpAudio.Name()
	_ = tmpAudio.Close()
	defer func() { _ = os.Remove(tmpAudioPath) }()

	if err := extractAudio(tmpVideoPath, tmpAudioPath); err != nil {
		if errors.Is(err, errNoAudio) {
			slog.Info("transcribe: video has no audio stream", "video_id", videoID)
			if _, dbErr := db.Exec(ctx,
				`UPDATE videos SET transcript_status = 'no_audio', transcript_started_at = NULL, updated_at = now() WHERE id = $1`,
				videoID,
			); dbErr != nil {
				slog.Error("transcribe: failed to set no_audio status", "video_id", videoID, "error", dbErr)
			}
			return
		}
		slog.Error("transcribe: audio extraction failed", "video_id", videoID, "error", err)
		setFailed()
		return
	}

	tmpOutput, err := os.CreateTemp("", "sendrec-transcribe-out-*")
	if err != nil {
		slog.Error("transcribe: failed to create temp output file", "error", err)
		setFailed()
		return
	}
	tmpOutputPrefix := tmpOutput.Name()
	_ = tmpOutput.Close()
	_ = os.Remove(tmpOutputPrefix)
	defer func() {
		_ = os.Remove(tmpOutputPrefix + ".vtt")
		_ = os.Remove(tmpOutputPrefix + ".json")
		_ = os.Remove(tmpOutputPrefix)
	}()

	if err := runWhisper(tmpAudioPath, tmpOutputPrefix, language); err != nil {
		slog.Error("transcribe: whisper failed", "video_id", videoID, "error", err)
		setFailed()
		return
	}

	segments, err := parseWhisperJSON(tmpOutputPrefix + ".json")
	if err != nil {
		slog.Error("transcribe: failed to parse whisper output", "video_id", videoID, "error", err)
		setFailed()
		return
	}

	transcriptKey := transcriptFileKey(userID, shareToken)
	vttPath := tmpOutputPrefix + ".vtt"
	if err := storage.UploadFile(ctx, transcriptKey, vttPath, "text/vtt"); err != nil {
		slog.Error("transcribe: failed to upload VTT", "video_id", videoID, "error", err)
		setFailed()
		return
	}

	segmentsJSON, err := json.Marshal(segments)
	if err != nil {
		slog.Error("transcribe: failed to marshal segments", "video_id", videoID, "error", err)
		setFailed()
		return
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET transcript_key = $1, transcript_json = $2, transcript_status = 'ready', transcript_started_at = NULL, updated_at = now() WHERE id = $3`,
		transcriptKey, string(segmentsJSON), videoID,
	); err != nil {
		slog.Error("transcribe: failed to update transcript data", "video_id", videoID, "error", err)
		setFailed()
		return
	}

	slog.Info("transcribe: completed", "video_id", videoID, "segments", len(segments))

	if aiEnabled {
		if _, err := db.Exec(ctx,
			`UPDATE videos SET summary_status = 'pending', updated_at = now() WHERE id = $1`,
			videoID,
		); err != nil {
			slog.Error("transcribe: failed to enqueue summary", "video_id", videoID, "error", err)
		}
	}
}
