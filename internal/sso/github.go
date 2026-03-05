package sso

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
)

// GitHubConfig holds the OAuth2 credentials for GitHub social login.
type GitHubConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// GitHubProvider authenticates users via GitHub OAuth2.
type GitHubProvider struct {
	oauthConfig oauth2.Config
	APIBase     string
}

// NewGitHubProvider returns a provider configured for the GitHub OAuth2 flow.
func NewGitHubProvider(cfg GitHubConfig) *GitHubProvider {
	return &GitHubProvider{
		oauthConfig: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     endpoints.GitHub,
			Scopes:       []string{"read:user", "user:email"},
		},
		APIBase: "https://api.github.com",
	}
}

// AuthURL returns the GitHub authorization URL the user should be redirected to.
func (p *GitHubProvider) AuthURL(state string) string {
	return p.oauthConfig.AuthCodeURL(state)
}

// Exchange trades an authorization code for GitHub user identity information.
// It fetches the authenticated user's profile and their verified primary email.
func (p *GitHubProvider) Exchange(ctx context.Context, code string) (*UserInfo, error) {
	token, err := p.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	client := p.oauthConfig.Client(ctx, token)

	user, err := p.fetchUser(client)
	if err != nil {
		return nil, err
	}

	email, err := p.fetchPrimaryEmail(client)
	if err != nil {
		return nil, err
	}

	return &UserInfo{
		ExternalID: fmt.Sprintf("%d", user.ID),
		Email:      email,
		Name:       user.displayName(),
	}, nil
}

type githubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
}

func (u githubUser) displayName() string {
	if u.Name != "" {
		return u.Name
	}
	return u.Login
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (p *GitHubProvider) fetchUser(client *http.Client) (*githubUser, error) {
	resp, err := client.Get(p.APIBase + "/user")
	if err != nil {
		return nil, fmt.Errorf("fetch github user: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github /user returned status %d", resp.StatusCode)
	}

	var user githubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode github user: %w", err)
	}
	return &user, nil
}

func (p *GitHubProvider) fetchPrimaryEmail(client *http.Client) (string, error) {
	resp, err := client.Get(p.APIBase + "/user/emails")
	if err != nil {
		return "", fmt.Errorf("fetch github emails: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github /user/emails returned status %d", resp.StatusCode)
	}

	var emails []githubEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("decode github emails: %w", err)
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	return "", fmt.Errorf("no verified primary email found on github account")
}
