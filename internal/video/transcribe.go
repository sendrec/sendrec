package video

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	if os.Getenv("TRANSCRIPTION_ENABLED") == "false" {
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

func extractAudio(inputPath, outputPath string) error {
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

func runWhisper(audioPath, outputPrefix string) error {
	cmd := exec.Command("whisper-cli",
		"-m", whisperModelPath(),
		"-f", audioPath,
		"--output-vtt",
		"--output-json",
		"-of", outputPrefix,
		"-t", "2",
		"-l", "auto",
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

	var segments []TranscriptSegment
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

var transcriptionSemaphore = make(chan struct{}, 1)

func TranscribeVideo(ctx context.Context, db database.DBTX, storage ObjectStorage, videoID, fileKey, userID, shareToken string) {
	select {
	case transcriptionSemaphore <- struct{}{}:
		defer func() { <-transcriptionSemaphore }()
	case <-ctx.Done():
		log.Printf("transcribe: context cancelled waiting for semaphore for video %s", videoID)
		return
	}

	if !isTranscriptionAvailable() {
		log.Printf("transcribe: transcription not available, skipping video %s", videoID)
		return
	}

	log.Printf("transcribe: starting for video %s", videoID)

	if _, err := db.Exec(ctx,
		`UPDATE videos SET transcript_status = 'processing', updated_at = now() WHERE id = $1`,
		videoID,
	); err != nil {
		log.Printf("transcribe: failed to set processing status for %s: %v", videoID, err)
		return
	}

	setFailed := func() {
		if _, err := db.Exec(ctx,
			`UPDATE videos SET transcript_status = 'failed', updated_at = now() WHERE id = $1`,
			videoID,
		); err != nil {
			log.Printf("transcribe: failed to set failed status for %s: %v", videoID, err)
		}
	}

	tmpVideo, err := os.CreateTemp("", "sendrec-transcribe-*.webm")
	if err != nil {
		log.Printf("transcribe: failed to create temp video file: %v", err)
		setFailed()
		return
	}
	tmpVideoPath := tmpVideo.Name()
	_ = tmpVideo.Close()
	defer func() { _ = os.Remove(tmpVideoPath) }()

	if err := storage.DownloadToFile(ctx, fileKey, tmpVideoPath); err != nil {
		log.Printf("transcribe: failed to download video %s: %v", videoID, err)
		setFailed()
		return
	}

	tmpAudio, err := os.CreateTemp("", "sendrec-transcribe-*.wav")
	if err != nil {
		log.Printf("transcribe: failed to create temp audio file: %v", err)
		setFailed()
		return
	}
	tmpAudioPath := tmpAudio.Name()
	_ = tmpAudio.Close()
	defer func() { _ = os.Remove(tmpAudioPath) }()

	if err := extractAudio(tmpVideoPath, tmpAudioPath); err != nil {
		log.Printf("transcribe: audio extraction failed for %s: %v", videoID, err)
		setFailed()
		return
	}

	tmpOutput, err := os.CreateTemp("", "sendrec-transcribe-out-*")
	if err != nil {
		log.Printf("transcribe: failed to create temp output file: %v", err)
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

	if err := runWhisper(tmpAudioPath, tmpOutputPrefix); err != nil {
		log.Printf("transcribe: whisper failed for %s: %v", videoID, err)
		setFailed()
		return
	}

	segments, err := parseWhisperJSON(tmpOutputPrefix + ".json")
	if err != nil {
		log.Printf("transcribe: failed to parse whisper output for %s: %v", videoID, err)
		setFailed()
		return
	}

	transcriptKey := transcriptFileKey(userID, shareToken)
	vttPath := tmpOutputPrefix + ".vtt"
	if err := storage.UploadFile(ctx, transcriptKey, vttPath, "text/vtt"); err != nil {
		log.Printf("transcribe: failed to upload VTT for %s: %v", videoID, err)
		setFailed()
		return
	}

	segmentsJSON, err := json.Marshal(segments)
	if err != nil {
		log.Printf("transcribe: failed to marshal segments for %s: %v", videoID, err)
		setFailed()
		return
	}

	if _, err := db.Exec(ctx,
		`UPDATE videos SET transcript_key = $1, transcript_json = $2, transcript_status = 'ready', updated_at = now() WHERE id = $3`,
		transcriptKey, string(segmentsJSON), videoID,
	); err != nil {
		log.Printf("transcribe: failed to update transcript data for %s: %v", videoID, err)
		setFailed()
		return
	}

	log.Printf("transcribe: completed for video %s (%d segments)", videoID, len(segments))
}
