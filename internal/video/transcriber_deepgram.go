package video

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type deepgram struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func newDeepgram(apiKey, model string, timeout time.Duration) *deepgram {
	if model == "" {
		model = "nova-3"
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &deepgram{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (d *deepgram) Name() string { return "deepgram:" + d.model }

func (d *deepgram) Available() bool { return d.apiKey != "" }

type deepgramResponse struct {
	Results struct {
		Channels []struct {
			Alternatives []struct {
				Transcript string `json:"transcript"`
				Paragraphs struct {
					Paragraphs []struct {
						Sentences []struct {
							Text  string  `json:"text"`
							Start float64 `json:"start"`
							End   float64 `json:"end"`
						} `json:"sentences"`
					} `json:"paragraphs"`
				} `json:"paragraphs"`
			} `json:"alternatives"`
		} `json:"channels"`
	} `json:"results"`
}

func (d *deepgram) Transcribe(ctx context.Context, audioPath, language string) ([]TranscriptSegment, error) {
	f, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("open audio: %w", err)
	}
	defer func() { _ = f.Close() }()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat audio: %w", err)
	}

	params := url.Values{}
	params.Set("model", d.model)
	params.Set("smart_format", "true")
	params.Set("punctuate", "true")
	params.Set("paragraphs", "true")
	if language == "" || language == "auto" {
		params.Set("detect_language", "true")
	} else {
		params.Set("language", language)
	}

	endpoint := "https://api.deepgram.com/v1/listen?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, f)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.ContentLength = stat.Size()
	req.Header.Set("Authorization", "Token "+d.apiKey)
	req.Header.Set("Content-Type", "audio/wav")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deepgram transcription: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deepgram returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed deepgramResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse deepgram response: %w", err)
	}

	if len(parsed.Results.Channels) == 0 || len(parsed.Results.Channels[0].Alternatives) == 0 {
		return nil, ErrNoAudio
	}

	alt := parsed.Results.Channels[0].Alternatives[0]
	if strings.TrimSpace(alt.Transcript) == "" {
		return nil, ErrNoAudio
	}

	var segments []TranscriptSegment
	for _, para := range alt.Paragraphs.Paragraphs {
		for _, s := range para.Sentences {
			text := strings.TrimSpace(s.Text)
			if text == "" {
				continue
			}
			segments = append(segments, TranscriptSegment{
				Start: s.Start,
				End:   s.End,
				Text:  text,
			})
		}
	}

	if len(segments) == 0 {
		segments = append(segments, TranscriptSegment{Text: strings.TrimSpace(alt.Transcript)})
	}

	return segments, nil
}
