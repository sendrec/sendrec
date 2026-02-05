package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAccessToken_ReturnsValidToken(t *testing.T) {
	token, err := GenerateAccessToken("test-secret", "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestGenerateAccessToken_SetsAccessType(t *testing.T) {
	token, err := GenerateAccessToken("test-secret", "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	claims, err := ValidateToken("test-secret", token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.TokenType != "access" {
		t.Errorf("expected token type %q, got %q", "access", claims.TokenType)
	}
}

func TestGenerateRefreshToken_SetsRefreshType(t *testing.T) {
	token, err := GenerateRefreshToken("test-secret", "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	claims, err := ValidateToken("test-secret", token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.TokenType != "refresh" {
		t.Errorf("expected token type %q, got %q", "refresh", claims.TokenType)
	}
}

func TestValidateToken_CorrectSecret(t *testing.T) {
	secret := "test-secret"
	token, _ := GenerateAccessToken(secret, "user-123")

	claims, err := ValidateToken(secret, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims == nil {
		t.Fatal("expected non-nil claims")
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	token, _ := GenerateAccessToken("secret-one", "user-123")

	_, err := ValidateToken("secret-two", token)
	if err == nil {
		t.Error("expected error for wrong secret, got nil")
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	claims := &Claims{
		UserID:    "user-123",
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("unexpected error signing token: %v", err)
	}

	_, err = ValidateToken("test-secret", signed)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}

func TestValidateToken_InvalidString(t *testing.T) {
	_, err := ValidateToken("test-secret", "not-a-valid-jwt")
	if err == nil {
		t.Error("expected error for invalid token string, got nil")
	}
}

func TestValidateToken_PreservesUserID(t *testing.T) {
	userID := "550e8400-e29b-41d4-a716-446655440000"
	token, _ := GenerateAccessToken("test-secret", userID)

	claims, err := ValidateToken("test-secret", token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("expected userID %q, got %q", userID, claims.UserID)
	}
}

func TestAccessToken_HasCorrectDuration(t *testing.T) {
	token, _ := GenerateAccessToken("test-secret", "user-123")

	claims, err := ValidateToken("test-secret", token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedExpiry := time.Now().Add(AccessTokenDuration)
	actualExpiry := claims.ExpiresAt.Time
	delta := expectedExpiry.Sub(actualExpiry).Abs()

	if delta > 2*time.Second {
		t.Errorf("access token expiry off by %v; expected ~%v, got %v", delta, expectedExpiry, actualExpiry)
	}
}

func TestRefreshToken_HasCorrectDuration(t *testing.T) {
	token, _ := GenerateRefreshToken("test-secret", "user-123")

	claims, err := ValidateToken("test-secret", token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedExpiry := time.Now().Add(RefreshTokenDuration)
	actualExpiry := claims.ExpiresAt.Time
	delta := expectedExpiry.Sub(actualExpiry).Abs()

	if delta > 2*time.Second {
		t.Errorf("refresh token expiry off by %v; expected ~%v, got %v", delta, expectedExpiry, actualExpiry)
	}
}

func TestValidateToken_RejectsNonHMACSigning(t *testing.T) {
	// Create a token with "none" algorithm to verify the signing method check
	claims := &Claims{
		UserID:    "user-123",
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("unexpected error signing token: %v", err)
	}

	_, err = ValidateToken("test-secret", signed)
	if err == nil {
		t.Error("expected error for non-HMAC signing method, got nil")
	}
}
