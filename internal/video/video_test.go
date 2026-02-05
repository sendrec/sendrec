package video

import (
	"testing"
)

func TestGenerateShareToken(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		token, err := generateShareToken()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}

		if len(token) != 12 {
			t.Errorf("iteration %d: expected 12-character token, got %d characters: %q", i, len(token), token)
		}

		for _, c := range token {
			if !isURLSafe(c) {
				t.Errorf("iteration %d: token contains non-URL-safe character %q in %q", i, string(c), token)
			}
		}

		if seen[token] {
			t.Errorf("iteration %d: duplicate token %q", i, token)
		}
		seen[token] = true
	}
}

func isURLSafe(c rune) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_'
}
