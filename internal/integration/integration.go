package integration

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"
)

// IssueCreator creates issues in external project-management tools.
type IssueCreator interface {
	CreateIssue(ctx context.Context, req CreateIssueRequest) (*CreateIssueResponse, error)
	ValidateConfig(ctx context.Context) error
}

// CreateIssueRequest holds the data needed to file an issue.
type CreateIssueRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	VideoURL    string `json:"videoUrl"`
}

// CreateIssueResponse holds the result of filing an issue.
type CreateIssueResponse struct {
	IssueURL string `json:"issueUrl"`
	IssueKey string `json:"issueKey"`
}

// Integration represents a user's configured integration with an external service.
type Integration struct {
	ID        string         `json:"id"`
	UserID    string         `json:"-"`
	Provider  string         `json:"provider"`
	Config    map[string]any `json:"config"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// DeriveKey produces a 32-byte AES-256 key from the given secret using SHA-256.
func DeriveKey(secret string) []byte {
	hash := sha256.Sum256([]byte(secret))
	return hash[:]
}

// Encrypt encrypts plaintext with AES-256-GCM using a random nonce and returns
// the result as a base64-encoded string (nonce + ciphertext).
func Encrypt(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decodes a base64 ciphertext and decrypts it with AES-256-GCM.
func Decrypt(key []byte, ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, sealed := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// MaskToken reveals the first 4 characters and replaces the rest with asterisks.
// Tokens with 4 or fewer characters are fully masked.
func MaskToken(token string) string {
	if len(token) == 0 {
		return ""
	}
	if len(token) <= 4 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", 6)
}
