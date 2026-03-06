package sso

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
)

// newOIDCTestServer starts an httptest.Server that mimics an OIDC provider,
// serving discovery, JWKS, token, and userinfo endpoints.
func newOIDCTestServer(t *testing.T) (*httptest.Server, *rsa.PrivateKey) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	var serverURL string

	mux := http.NewServeMux()

	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		discovery := map[string]any{
			"issuer":                 serverURL,
			"authorization_endpoint": serverURL + "/authorize",
			"token_endpoint":         serverURL + "/token",
			"userinfo_endpoint":      serverURL + "/userinfo",
			"jwks_uri":               serverURL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(discovery)
	})

	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, r *http.Request) {
		jwk := jose.JSONWebKey{Key: &privateKey.PublicKey, KeyID: "test-key", Algorithm: "RS256", Use: "sig"}
		jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		signer, err := jose.NewSigner(
			jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
			(&jose.SignerOptions{}).WithHeader("kid", "test-key"),
		)
		if err != nil {
			http.Error(w, "signer error", 500)
			return
		}

		now := time.Now()
		claims := struct {
			josejwt.Claims
			Email string `json:"email"`
			Name  string `json:"name"`
		}{
			Claims: josejwt.Claims{
				Issuer:    serverURL,
				Subject:   "oidc-user-123",
				Audience:  josejwt.Audience{"test-client-id"},
				IssuedAt:  josejwt.NewNumericDate(now),
				Expiry:    josejwt.NewNumericDate(now.Add(time.Hour)),
				NotBefore: josejwt.NewNumericDate(now.Add(-time.Minute)),
			},
			Email: "oidc@example.com",
			Name:  "OIDC User",
		}

		rawToken, err := josejwt.Signed(signer).Claims(claims).Serialize()
		if err != nil {
			http.Error(w, "token error", 500)
			return
		}

		resp := map[string]any{
			"access_token": "mock-access-token",
			"token_type":   "Bearer",
			"id_token":     rawToken,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("GET /userinfo", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"sub":   "oidc-user-123",
			"email": "oidc@example.com",
			"name":  "OIDC User",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	serverURL = server.URL

	return server, privateKey
}

func TestNewOIDCProvider_Discovery(t *testing.T) {
	server, _ := newOIDCTestServer(t)
	defer server.Close()

	provider, err := NewOIDCProvider(context.Background(), OIDCConfig{
		IssuerURL:    server.URL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost/callback",
	})
	if err != nil {
		t.Fatalf("NewOIDCProvider() error: %v", err)
	}
	if provider == nil {
		t.Fatal("NewOIDCProvider() returned nil")
	}
}

func TestOIDCProvider_AuthURL(t *testing.T) {
	server, _ := newOIDCTestServer(t)
	defer server.Close()

	provider, err := NewOIDCProvider(context.Background(), OIDCConfig{
		IssuerURL:    server.URL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost/callback",
	})
	if err != nil {
		t.Fatalf("NewOIDCProvider() error: %v", err)
	}

	url := provider.AuthURL("test-state-value")
	if !strings.HasPrefix(url, server.URL+"/authorize") {
		t.Fatalf("AuthURL() = %q, want prefix %q", url, server.URL+"/authorize")
	}
	if !strings.Contains(url, "state=test-state-value") {
		t.Fatalf("AuthURL() = %q, want state parameter", url)
	}
	if !strings.Contains(url, "client_id=test-client-id") {
		t.Fatalf("AuthURL() = %q, want client_id parameter", url)
	}
}

func TestOIDCProvider_Exchange(t *testing.T) {
	server, _ := newOIDCTestServer(t)
	defer server.Close()

	provider, err := NewOIDCProvider(context.Background(), OIDCConfig{
		IssuerURL:    server.URL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost/callback",
	})
	if err != nil {
		t.Fatalf("NewOIDCProvider() error: %v", err)
	}

	info, err := provider.Exchange(context.Background(), "test-auth-code")
	if err != nil {
		t.Fatalf("Exchange() error: %v", err)
	}

	if info.ExternalID != "oidc-user-123" {
		t.Errorf("ExternalID = %q, want %q", info.ExternalID, "oidc-user-123")
	}
	if info.Email != "oidc@example.com" {
		t.Errorf("Email = %q, want %q", info.Email, "oidc@example.com")
	}
	if info.Name != "OIDC User" {
		t.Errorf("Name = %q, want %q", info.Name, "OIDC User")
	}
}

// newOIDCTestServerTrailingSlash starts an OIDC test server whose discovery
// document returns the issuer URL with a trailing slash, simulating Auth0.
func newOIDCTestServerTrailingSlash(t *testing.T) *httptest.Server {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	var serverURL string

	mux := http.NewServeMux()

	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		// Return issuer WITH trailing slash (Auth0 behavior)
		discovery := map[string]any{
			"issuer":                 serverURL + "/",
			"authorization_endpoint": serverURL + "/authorize",
			"token_endpoint":         serverURL + "/token",
			"userinfo_endpoint":      serverURL + "/userinfo",
			"jwks_uri":               serverURL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(discovery)
	})

	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, r *http.Request) {
		jwk := jose.JSONWebKey{Key: &privateKey.PublicKey, KeyID: "test-key", Algorithm: "RS256", Use: "sig"}
		jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	server := httptest.NewServer(mux)
	serverURL = server.URL

	return server
}

func TestNewOIDCProvider_TrailingSlashRetry(t *testing.T) {
	server := newOIDCTestServerTrailingSlash(t)
	defer server.Close()

	// Config has issuer WITHOUT trailing slash, but the provider returns it WITH one.
	provider, err := NewOIDCProvider(context.Background(), OIDCConfig{
		IssuerURL:    server.URL, // no trailing slash
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost/callback",
	})
	if err != nil {
		t.Fatalf("NewOIDCProvider() should succeed after trailing-slash retry, got: %v", err)
	}
	if provider == nil {
		t.Fatal("NewOIDCProvider() returned nil")
	}
}

func TestNewOIDCProvider_InvalidIssuer(t *testing.T) {
	_, err := NewOIDCProvider(context.Background(), OIDCConfig{
		IssuerURL:    "http://invalid.example.com:0",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost/callback",
	})
	if err == nil {
		t.Fatal("NewOIDCProvider() expected error for invalid issuer")
	}
}
