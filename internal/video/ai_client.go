package video

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Chapter struct {
	Title string  `json:"title"`
	Start float64 `json:"start"`
}

type SummaryResult struct {
	Summary  string    `json:"summary"`
	Chapters []Chapter `json:"chapters"`
}

type AIClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewAIClient(baseURL, apiKey, model string) *AIClient {
	return &AIClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

const summarySystemPrompt = `You are a video content analyzer. Given a timestamped transcript, produce a JSON object with:
- "summary": A 2-3 sentence overview of what the video covers.
- "chapters": An array of objects with "title" (string, 3-6 words) and "start" (number, seconds from transcript timestamps) marking major topic changes. Include 2-8 chapters depending on video length. The first chapter should start at 0.

Return ONLY valid JSON, no markdown formatting.`

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

func (c *AIClient) GenerateSummary(ctx context.Context, transcript string) (*SummaryResult, error) {
	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: summarySystemPrompt},
			{Role: "user", Content: transcript},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("AI API returned empty choices")
	}

	content := chatResp.Choices[0].Message.Content
	return parseSummaryJSON(content)
}

func parseSummaryJSON(content string) (*SummaryResult, error) {
	var result SummaryResult
	if err := json.Unmarshal([]byte(content), &result); err == nil {
		return &result, nil
	}

	stripped := stripMarkdownFences(content)
	if err := json.Unmarshal([]byte(stripped), &result); err != nil {
		return nil, fmt.Errorf("parse summary JSON: %w", err)
	}

	return &result, nil
}

func stripMarkdownFences(content string) string {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		firstNewline := strings.Index(trimmed, "\n")
		if firstNewline == -1 {
			return trimmed
		}
		trimmed = trimmed[firstNewline+1:]

		if idx := strings.LastIndex(trimmed, "```"); idx != -1 {
			trimmed = trimmed[:idx]
		}

		return strings.TrimSpace(trimmed)
	}
	return trimmed
}
