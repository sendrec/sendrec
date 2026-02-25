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

func NewAIClient(baseURL, apiKey, model string, timeout time.Duration) *AIClient {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &AIClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

const summarySystemPrompt = `You are a video content analyzer. Given a timestamped transcript, produce a JSON object with:
- "summary": A 2-3 sentence overview of what the video covers.
- "chapters": An array of objects with "title" (string, 3-6 words) and "start" (number, seconds from transcript timestamps) marking major topic changes. Include 2-8 chapters depending on video length. The first chapter should start at 0.

Write the summary and chapter titles in the same language as the transcript.
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
	defer func() { _ = resp.Body.Close() }()

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

const titleSystemPrompt = `Given this video transcript, generate a concise title (3-8 words) that captures the main topic. Return ONLY the title text, no quotes, no explanation. Write in the same language as the transcript.`

func (c *AIClient) GenerateTitle(ctx context.Context, transcript string) (string, error) {
	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: titleSystemPrompt},
			{Role: "user", Content: transcript},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("AI API returned empty choices")
	}

	title := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	if len(title) >= 2 && title[0] == '"' && title[len(title)-1] == '"' {
		title = title[1 : len(title)-1]
	}
	if len(title) > 200 {
		title = title[:200]
	}

	return title, nil
}

const documentSystemPrompt = `You are a technical writer. Given a timestamped video transcript, convert it into a well-structured markdown document.

Guidelines:
- Use headings (## and ###) for major topic changes
- Use bullet points for key details and action items
- Include a brief introduction summarizing the video's purpose
- Group related content under clear headings
- Include timestamps as references where helpful (e.g., "at 2:30")
- End with a conclusions section listing 3-5 main points
- Write EVERYTHING in the same language as the transcript, including ALL headings, section titles, and bullet points — do not use any English words if the transcript is not in English
- Return ONLY markdown, no explanations or meta-commentary`

func (c *AIClient) GenerateDocument(ctx context.Context, transcript, language string) (string, error) {
	prompt := documentSystemPrompt
	if language != "" {
		prompt += fmt.Sprintf("\n- The transcript language is %s. Write the entire document in %s.", language, language)
	}

	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: prompt},
			{Role: "user", Content: transcript},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("AI API returned empty choices")
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	content = stripMarkdownFences(content)
	return content, nil
}

func isAutoGeneratedTitle(title string) bool {
	if title == "Untitled Recording" || title == "Untitled Video" {
		return true
	}
	if strings.HasPrefix(title, "Recording ") && len(title) > 10 && title[10] >= '0' && title[10] <= '9' {
		return true
	}
	return false
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
