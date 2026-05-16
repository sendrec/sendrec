package video

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

type openaiWhisper struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func newOpenAIWhisper(baseURL, apiKey, model string, timeout time.Duration) *openaiWhisper {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	if model == "" {
		model = "whisper-1"
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &openaiWhisper{
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (o *openaiWhisper) Name() string { return "openai-whisper:" + o.model }

func (o *openaiWhisper) Available() bool { return o.apiKey != "" }

type openaiTranscriptionResponse struct {
	Text     string                       `json:"text"`
	Segments []openaiTranscriptionSegment `json:"segments"`
}

type openaiTranscriptionSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

func (o *openaiWhisper) Transcribe(ctx context.Context, audioPath, language string) ([]TranscriptSegment, error) {
	f, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("open audio: %w", err)
	}
	defer func() { _ = f.Close() }()

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		defer func() { _ = pw.Close() }()
		defer func() { _ = mw.Close() }()

		fileWriter, err := mw.CreateFormFile("file", "audio.wav")
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(fileWriter, f); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if err := mw.WriteField("model", o.model); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if err := mw.WriteField("response_format", "verbose_json"); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if language != "" && language != "auto" {
			if err := mw.WriteField("language", language); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/v1/audio/transcriptions", pr)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai transcription: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai transcription returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed openaiTranscriptionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse openai response: %w", err)
	}

	if strings.TrimSpace(parsed.Text) == "" && len(parsed.Segments) == 0 {
		return nil, ErrNoAudio
	}

	segments := make([]TranscriptSegment, 0, len(parsed.Segments))
	for _, seg := range parsed.Segments {
		text := strings.TrimSpace(seg.Text)
		if text == "" {
			continue
		}
		segments = append(segments, TranscriptSegment{
			Start: seg.Start,
			End:   seg.End,
			Text:  text,
		})
	}

	if len(segments) == 0 && strings.TrimSpace(parsed.Text) != "" {
		segments = append(segments, TranscriptSegment{Text: strings.TrimSpace(parsed.Text)})
	}

	return segments, nil
}
