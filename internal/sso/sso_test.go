package sso

import (
	"testing"
)

func TestGenerateState(t *testing.T) {
	state, err := generateState()
	if err != nil {
		t.Fatalf("generateState() error: %v", err)
	}
	if state == "" {
		t.Fatal("generateState() returned empty string")
	}
	// 32 bytes base64-RawURL-encoded = 43 characters
	if len(state) < 32 {
		t.Fatalf("generateState() returned too short string: %d chars", len(state))
	}
}

func TestGenerateState_Unique(t *testing.T) {
	state1, err := generateState()
	if err != nil {
		t.Fatalf("first generateState() error: %v", err)
	}
	state2, err := generateState()
	if err != nil {
		t.Fatalf("second generateState() error: %v", err)
	}
	if state1 == state2 {
		t.Fatal("two calls to generateState() produced identical values")
	}
}
