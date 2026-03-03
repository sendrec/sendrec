package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// JiraClient implements IssueCreator for Jira Cloud using the REST API v3.
type JiraClient struct {
	baseURL    string
	email      string
	apiToken   string
	projectKey string
}

// NewJiraClient returns a JiraClient configured for the given Jira instance.
func NewJiraClient(baseURL, email, apiToken, projectKey string) *JiraClient {
	return &JiraClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		email:      email,
		apiToken:   apiToken,
		projectKey: projectKey,
	}
}

func (c *JiraClient) CreateIssue(ctx context.Context, req CreateIssueRequest) (*CreateIssueResponse, error) {
	body := map[string]any{
		"fields": map[string]any{
			"project":     map[string]string{"key": c.projectKey},
			"summary":     req.Title,
			"description": buildADFDescription(req),
			"issuetype":   map[string]string{"name": "Task"},
		},
	}

	bodyJSON, _ := json.Marshal(body)

	url := c.baseURL + "/rest/api/3/issue"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	c.setAuth(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("could not connect to Jira")
	}
	defer resp.Body.Close() //nolint:errcheck

	if err := checkProviderResponse(resp); err != nil {
		return nil, err
	}

	var result struct {
		Key  string `json:"key"`
		Self string `json:"self"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid response from Jira")
	}

	return &CreateIssueResponse{
		IssueURL: c.baseURL + "/browse/" + result.Key,
		IssueKey: result.Key,
	}, nil
}

func (c *JiraClient) ValidateConfig(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/rest/api/3/myself", nil)
	if err != nil {
		return err
	}
	c.setAuth(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not connect to Jira")
	}
	defer resp.Body.Close() //nolint:errcheck
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed, check your email and API token")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("jira API error: %d", resp.StatusCode)
	}
	return nil
}

func (c *JiraClient) setAuth(req *http.Request) {
	auth := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.apiToken))
	req.Header.Set("Authorization", "Basic "+auth)
}

func buildADFDescription(req CreateIssueRequest) map[string]any {
	content := []any{adfParagraph(adfInlineCard(req.VideoURL))}
	if req.Description != "" {
		content = append(content, adfParagraph(adfText("Transcript:")), adfCodeBlock(req.Description))
	}
	return map[string]any{"version": 1, "type": "doc", "content": content}
}

func adfParagraph(content ...any) map[string]any {
	return map[string]any{"type": "paragraph", "content": content}
}

func adfText(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}

func adfInlineCard(url string) map[string]any {
	return map[string]any{"type": "inlineCard", "attrs": map[string]string{"url": url}}
}

func adfCodeBlock(text string) map[string]any {
	return map[string]any{"type": "codeBlock", "content": []any{adfText(text)}}
}
