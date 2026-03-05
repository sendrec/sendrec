package sso

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// UserInfo holds the identity claims returned by an SSO provider after
// a successful authentication exchange.
type UserInfo struct {
	ExternalID string
	Email      string
	Name       string
}

// Provider abstracts an SSO identity provider (OIDC, GitHub, etc.).
type Provider interface {
	AuthURL(state string) string
	Exchange(ctx context.Context, code string) (*UserInfo, error)
}

const stateBytes = 32

// generateState produces a cryptographically random, URL-safe state parameter
// for the OAuth2 authorization flow.
func generateState() (string, error) {
	b := make([]byte, stateBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
