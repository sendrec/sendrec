package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GitHubClient implements IssueCreator for GitHub Issues.
type GitHubClient struct {
	token   string
	owner   string
	repo    string
	baseURL string
}

// NewGitHubClient returns a GitHubClient configured with the given token and repository.
func NewGitHubClient(token, owner, repo string) *GitHubClient {
	return &GitHubClient{token: token, owner: owner, repo: repo, baseURL: "https://api.github.com"}
}

func (c *GitHubClient) CreateIssue(ctx context.Context, req CreateIssueRequest) (*CreateIssueResponse, error) {
	body := map[string]any{
		"title": req.Title,
		"body":  formatGitHubBody(req),
	}
	bodyJSON, _ := json.Marshal(body)

	url := fmt.Sprintf("%s/repos/%s/%s/issues", c.baseURL, c.owner, c.repo)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("could not connect to GitHub")
	}
	defer resp.Body.Close() //nolint:errcheck

	if err := checkProviderResponse(resp); err != nil {
		return nil, err
	}

	var result struct {
		HTMLURL string `json:"html_url"`
		Number  int    `json:"number"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid response from GitHub")
	}

	return &CreateIssueResponse{IssueURL: result.HTMLURL, IssueKey: fmt.Sprintf("#%d", result.Number)}, nil
}

func (c *GitHubClient) ValidateConfig(ctx context.Context) error {
	if err := c.apiGet(ctx, "/user"); err != nil {
		return err
	}
	if err := c.apiGet(ctx, fmt.Sprintf("/repos/%s/%s", c.owner, c.repo)); err != nil {
		return fmt.Errorf("repository not found or not accessible")
	}
	return nil
}

func (c *GitHubClient) apiGet(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not connect to GitHub")
	}
	defer resp.Body.Close() //nolint:errcheck
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed, check your token")
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GitHub API error: %d", resp.StatusCode)
	}
	return nil
}

func formatGitHubBody(req CreateIssueRequest) string {
	body := fmt.Sprintf("**Video:** %s\n\n", req.VideoURL)
	if req.Description != "" {
		body += "<details>\n<summary>Transcript</summary>\n\n"
		body += req.Description
		body += "\n\n</details>"
	}
	return body
}

// checkProviderResponse maps common HTTP errors to user-friendly messages.
// Shared between GitHub and Jira clients.
func checkProviderResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed, check your API token")
	case http.StatusNotFound:
		return fmt.Errorf("project/repo not found, check your configuration")
	case http.StatusTooManyRequests:
		return fmt.Errorf("rate limited, try again in a moment")
	default:
		return fmt.Errorf("provider returned status %d", resp.StatusCode)
	}
}
