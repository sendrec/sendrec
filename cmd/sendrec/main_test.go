package main

import (
	"testing"
)

func TestGetEnvReturnsValueWhenSet(t *testing.T) {
	const key = "TEST_GETENV_SET"
	const expected = "custom-value"

	t.Setenv(key, expected)

	result := getEnv(key, "fallback")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGetEnvReturnsFallbackWhenUnset(t *testing.T) {
	const key = "TEST_GETENV_UNSET"
	const fallback = "default-value"

	result := getEnv(key, fallback)
	if result != fallback {
		t.Errorf("expected fallback %q, got %q", fallback, result)
	}
}

func TestGetEnvReturnsFallbackWhenEmpty(t *testing.T) {
	const key = "TEST_GETENV_EMPTY"
	const fallback = "default-value"

	t.Setenv(key, "")

	result := getEnv(key, fallback)
	if result != fallback {
		t.Errorf("expected fallback %q for empty env var, got %q", fallback, result)
	}
}
