package auth_test

import (
	"testing"

	"github.com/sendrec/sendrec/internal/auth"
)

func TestGenerateAndValidateAccessToken(t *testing.T) {
	secret := "test-secret"
	userID := "550e8400-e29b-41d4-a716-446655440000"

	token, err := auth.GenerateAccessToken(secret, userID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	claims, err := auth.ValidateToken(secret, token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected userID %q, got %q", userID, claims.UserID)
	}
}

func TestValidateTokenWrongSecret(t *testing.T) {
	token, _ := auth.GenerateAccessToken("secret-1", "user-id")

	_, err := auth.ValidateToken("secret-2", token)
	if err == nil {
		t.Error("expected error for wrong secret, got nil")
	}
}

func TestValidateTokenInvalidString(t *testing.T) {
	_, err := auth.ValidateToken("secret", "not-a-token")
	if err == nil {
		t.Error("expected error for invalid token, got nil")
	}
}
