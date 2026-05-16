package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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

func isTranscriptionEnabled() bool {
	return os.Getenv("TRANSCRIPTION_ENABLED") == "true"
}

func isTranscriptionAvailable(t Transcriber) bool {
	if !isTranscriptionEnabled() {
		return false
	}
	return t != nil && t.Available()
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

func formatVTTTimestamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := seconds - float64(h*3600+m*60)
	return fmt.Sprintf("%02d:%02d:%06.3f", h, m, s)
}

func segmentsToVTT(segments []TranscriptSegment) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for _, seg := range segments {
		b.WriteString(formatVTTTimestamp(seg.Start))
		b.WriteString(" --> ")
		b.WriteString(formatVTTTimestamp(seg.End))
		b.WriteString("\n")
		b.WriteString(seg.Text)
		b.WriteString("\n\n")
	}
	return b.String()
}

func processTranscription(ctx context.Context, db database.DBTX, storage ObjectStorage, transcriber Transcriber, videoID, fileKey, userID, shareToken, language string, aiEnabled bool) {
	if !isTranscriptionAvailable(transcriber) {
		slog.Warn("transcribe: transcription not available, marking as failed", "video_id", videoID)
		if _, err := db.Exec(ctx,
			`UPDATE videos SET transcript_status = 'failed', transcript_started_at = NULL, updated_at = now() WHERE id = $1`,
			videoID,
		); err != nil {
			slog.Error("transcribe: failed to set failed status", "video_id", videoID, "error", err)
		}
		return
	}

	slog.Info("transcribe: starting", "video_id", videoID, "language", language, "provider", transcriber.Name())

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

	segments, err := transcriber.Transcribe(ctx, tmpAudioPath, language)
	if err != nil {
		if errors.Is(err, ErrNoAudio) {
			slog.Info("transcribe: provider reported no speech", "video_id", videoID)
			if _, dbErr := db.Exec(ctx,
				`UPDATE videos SET transcript_status = 'no_audio', transcript_started_at = NULL, updated_at = now() WHERE id = $1`,
				videoID,
			); dbErr != nil {
				slog.Error("transcribe: failed to set no_audio status", "video_id", videoID, "error", dbErr)
			}
			return
		}
		slog.Error("transcribe: provider failed", "video_id", videoID, "provider", transcriber.Name(), "error", err)
		setFailed()
		return
	}

	transcriptKey := transcriptFileKey(userID, shareToken)
	tmpVTT, err := os.CreateTemp("", "sendrec-transcribe-*.vtt")
	if err != nil {
		slog.Error("transcribe: failed to create temp vtt file", "error", err)
		setFailed()
		return
	}
	tmpVTTPath := tmpVTT.Name()
	defer func() { _ = os.Remove(tmpVTTPath) }()
	if _, err := tmpVTT.WriteString(segmentsToVTT(segments)); err != nil {
		_ = tmpVTT.Close()
		slog.Error("transcribe: failed to write VTT", "video_id", videoID, "error", err)
		setFailed()
		return
	}
	_ = tmpVTT.Close()
	if err := storage.UploadFile(ctx, transcriptKey, tmpVTTPath, "text/vtt"); err != nil {
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
