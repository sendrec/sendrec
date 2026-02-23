package httputil

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
)

type contextKey string

const nonceKey contextKey = "csp-nonce"

func GenerateNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		slog.Error("failed to generate CSP nonce", "error", err)
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func ContextWithNonce(ctx context.Context, nonce string) context.Context {
	return context.WithValue(ctx, nonceKey, nonce)
}

func NonceFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(nonceKey).(string); ok {
		return v
	}
	return ""
}
