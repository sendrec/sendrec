package integration

import (
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := DeriveKey("test-jwt-secret")
	plaintext := "my-api-token-12345"

	encrypted, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if encrypted == plaintext {
		t.Error("encrypted text should differ from plaintext")
	}

	decrypted, err := Decrypt(key, encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	key := DeriveKey("test-jwt-secret")
	c1, _ := Encrypt(key, "same-token")
	c2, _ := Encrypt(key, "same-token")
	if c1 == c2 {
		t.Error("two encryptions of same plaintext should differ (unique nonce)")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1 := DeriveKey("secret-one")
	key2 := DeriveKey("secret-two")
	encrypted, _ := Encrypt(key1, "my-token")
	_, err := Decrypt(key2, encrypted)
	if err == nil {
		t.Error("decryption with wrong key should fail")
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"abcdefghij", "abcd******"},
		{"abc", "***"},
		{"", ""},
	}
	for _, tc := range tests {
		got := MaskToken(tc.input)
		if got != tc.expected {
			t.Errorf("MaskToken(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
