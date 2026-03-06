package sso

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig holds the settings needed to connect to an OpenID Connect provider.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// OIDCProvider authenticates users via the OpenID Connect protocol.
type OIDCProvider struct {
	provider    *oidc.Provider
	verifier    *oidc.IDTokenVerifier
	oauthConfig oauth2.Config
}

// NewOIDCProvider performs OIDC discovery on the issuer URL and returns a
// ready-to-use provider. The caller must pass a context that remains valid
// for the lifetime of the discovery HTTP call.
func NewOIDCProvider(ctx context.Context, cfg OIDCConfig) (*OIDCProvider, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		// Some providers (e.g. Auth0) return the issuer URL with a trailing
		// slash in their discovery document while the saved config may not
		// have one (or vice versa). Retry with the slash toggled.
		alt := cfg.IssuerURL
		if strings.HasSuffix(alt, "/") {
			alt = strings.TrimRight(alt, "/")
		} else {
			alt += "/"
		}
		provider, err = oidc.NewProvider(ctx, alt)
		if err != nil {
			return nil, fmt.Errorf("oidc discovery: %w", err)
		}
	}

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	oauthCfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})

	return &OIDCProvider{
		provider:    provider,
		verifier:    verifier,
		oauthConfig: oauthCfg,
	}, nil
}

// AuthURL returns the authorization URL the user should be redirected to.
func (p *OIDCProvider) AuthURL(state string) string {
	return p.oauthConfig.AuthCodeURL(state)
}

// Exchange trades an authorization code for user identity information.
// It first attempts to extract claims from the ID token. If the token
// response lacks an id_token, it falls back to the userinfo endpoint.
func (p *OIDCProvider) Exchange(ctx context.Context, code string) (*UserInfo, error) {
	token, err := p.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if ok && rawIDToken != "" {
		return p.extractFromIDToken(ctx, rawIDToken)
	}

	return p.extractFromUserInfo(ctx, token)
}

func (p *OIDCProvider) extractFromIDToken(ctx context.Context, rawIDToken string) (*UserInfo, error) {
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify id token: %w", err)
	}

	var claims struct {
		Subject string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("parse id token claims: %w", err)
	}

	return &UserInfo{
		ExternalID: claims.Subject,
		Email:      claims.Email,
		Name:       claims.Name,
	}, nil
}

func (p *OIDCProvider) extractFromUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	tokenSource := p.oauthConfig.TokenSource(ctx, token)

	userInfo, err := p.provider.UserInfo(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("fetch userinfo: %w", err)
	}

	var claims struct {
		Name string `json:"name"`
	}
	if err := userInfo.Claims(&claims); err != nil {
		return nil, fmt.Errorf("parse userinfo claims: %w", err)
	}

	return &UserInfo{
		ExternalID: userInfo.Subject,
		Email:      userInfo.Email,
		Name:       claims.Name,
	}, nil
}
