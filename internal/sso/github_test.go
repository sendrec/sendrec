package sso

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newGitHubTestServer starts a mock server that simulates GitHub's OAuth2
// token exchange, /user, and /user/emails endpoints.
func newGitHubTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"access_token": "gho_mock_token",
			"token_type":   "bearer",
			"scope":        "read:user,user:email",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		user := githubUser{
			ID:    42,
			Login: "octocat",
			Name:  "The Octocat",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(user)
	})

	mux.HandleFunc("GET /user/emails", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		emails := []githubEmail{
			{Email: "secondary@example.com", Primary: false, Verified: true},
			{Email: "octocat@github.com", Primary: true, Verified: true},
			{Email: "unverified@example.com", Primary: false, Verified: false},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(emails)
	})

	return httptest.NewServer(mux)
}

func newGitHubProviderWithMock(t *testing.T, server *httptest.Server) *GitHubProvider {
	t.Helper()
	provider := NewGitHubProvider(GitHubConfig{
		ClientID:     "gh-client-id",
		ClientSecret: "gh-client-secret",
		RedirectURL:  "http://localhost/callback",
	})
	// Point OAuth2 endpoints and API base at the test server.
	provider.oauthConfig.Endpoint.AuthURL = server.URL + "/login/oauth/authorize"
	provider.oauthConfig.Endpoint.TokenURL = server.URL + "/login/oauth/access_token"
	provider.APIBase = server.URL
	return provider
}

func TestGitHubProvider_AuthURL(t *testing.T) {
	provider := NewGitHubProvider(GitHubConfig{
		ClientID:     "gh-client-id",
		ClientSecret: "gh-client-secret",
		RedirectURL:  "http://localhost/callback",
	})

	url := provider.AuthURL("test-state")
	if !strings.Contains(url, "github.com") {
		t.Fatalf("AuthURL() = %q, want to contain github.com", url)
	}
	if !strings.Contains(url, "state=test-state") {
		t.Fatalf("AuthURL() = %q, want state parameter", url)
	}
	if !strings.Contains(url, "client_id=gh-client-id") {
		t.Fatalf("AuthURL() = %q, want client_id parameter", url)
	}
}

func TestGitHubProvider_Exchange(t *testing.T) {
	server := newGitHubTestServer(t)
	defer server.Close()

	provider := newGitHubProviderWithMock(t, server)

	info, err := provider.Exchange(context.Background(), "test-auth-code")
	if err != nil {
		t.Fatalf("Exchange() error: %v", err)
	}

	if info.ExternalID != "42" {
		t.Errorf("ExternalID = %q, want %q", info.ExternalID, "42")
	}
	if info.Email != "octocat@github.com" {
		t.Errorf("Email = %q, want %q", info.Email, "octocat@github.com")
	}
	if info.Name != "The Octocat" {
		t.Errorf("Name = %q, want %q", info.Name, "The Octocat")
	}
}

func TestGitHubProvider_Exchange_FallsBackToLogin(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "gho_mock_token",
			"token_type":   "bearer",
		})
	})

	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		user := githubUser{ID: 99, Login: "loginonly", Name: ""}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(user)
	})

	mux.HandleFunc("GET /user/emails", func(w http.ResponseWriter, r *http.Request) {
		emails := []githubEmail{
			{Email: "loginonly@example.com", Primary: true, Verified: true},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(emails)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newGitHubProviderWithMock(t, server)
	// Re-point to this server
	provider.oauthConfig.Endpoint.TokenURL = server.URL + "/login/oauth/access_token"
	provider.APIBase = server.URL

	info, err := provider.Exchange(context.Background(), "test-code")
	if err != nil {
		t.Fatalf("Exchange() error: %v", err)
	}

	if info.Name != "loginonly" {
		t.Errorf("Name = %q, want login name fallback %q", info.Name, "loginonly")
	}
}

func TestGitHubProvider_Exchange_NoVerifiedEmail(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "gho_mock_token",
			"token_type":   "bearer",
		})
	})

	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(githubUser{ID: 1, Login: "nomail"})
	})

	mux.HandleFunc("GET /user/emails", func(w http.ResponseWriter, r *http.Request) {
		emails := []githubEmail{
			{Email: "nope@example.com", Primary: true, Verified: false},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(emails)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newGitHubProviderWithMock(t, server)
	provider.oauthConfig.Endpoint.TokenURL = server.URL + "/login/oauth/access_token"
	provider.APIBase = server.URL

	_, err := provider.Exchange(context.Background(), "test-code")
	if err == nil {
		t.Fatal("Exchange() expected error for no verified primary email")
	}
	if !strings.Contains(err.Error(), "no verified primary email") {
		t.Fatalf("Exchange() error = %q, want to contain 'no verified primary email'", err.Error())
	}
}

func TestGitHubUser_DisplayName(t *testing.T) {
	tests := []struct {
		name     string
		user     githubUser
		expected string
	}{
		{"prefers full name", githubUser{Name: "Full Name", Login: "login"}, "Full Name"},
		{"falls back to login", githubUser{Name: "", Login: "login"}, "login"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.user.displayName()
			if got != tt.expected {
				t.Errorf("displayName() = %q, want %q", got, tt.expected)
			}
		})
	}
}
