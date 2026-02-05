package httputil

import (
	"context"
	"testing"
)

func TestGenerateNonce_ReturnsNonEmptyString(t *testing.T) {
	nonce := GenerateNonce()
	if nonce == "" {
		t.Error("expected non-empty nonce")
	}
}

func TestGenerateNonce_ReturnsUniqueValues(t *testing.T) {
	a := GenerateNonce()
	b := GenerateNonce()
	if a == b {
		t.Errorf("expected unique nonces, got %q twice", a)
	}
}

func TestGenerateNonce_Returns22Characters(t *testing.T) {
	nonce := GenerateNonce()
	// 16 bytes base64url-encoded without padding = 22 characters
	if len(nonce) != 22 {
		t.Errorf("expected 22-character nonce, got %d: %q", len(nonce), nonce)
	}
}

func TestNonceFromContext_ReturnsStoredValue(t *testing.T) {
	ctx := ContextWithNonce(context.Background(), "test-nonce-abc")
	got := NonceFromContext(ctx)
	if got != "test-nonce-abc" {
		t.Errorf("expected %q, got %q", "test-nonce-abc", got)
	}
}

func TestNonceFromContext_ReturnsEmptyWhenMissing(t *testing.T) {
	got := NonceFromContext(context.Background())
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}
