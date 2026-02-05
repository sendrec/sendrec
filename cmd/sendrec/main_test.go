package main

import (
	"os"
	"testing"
)

func TestGetEnvReturnsValueWhenSet(t *testing.T) {
	const key = "TEST_GETENV_SET"
	const expected = "custom-value"

	os.Setenv(key, expected)
	defer os.Unsetenv(key)

	result := getEnv(key, "fallback")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGetEnvReturnsFallbackWhenUnset(t *testing.T) {
	const key = "TEST_GETENV_UNSET"
	const fallback = "default-value"

	os.Unsetenv(key)

	result := getEnv(key, fallback)
	if result != fallback {
		t.Errorf("expected fallback %q, got %q", fallback, result)
	}
}

func TestGetEnvReturnsFallbackWhenEmpty(t *testing.T) {
	const key = "TEST_GETENV_EMPTY"
	const fallback = "default-value"

	os.Setenv(key, "")
	defer os.Unsetenv(key)

	result := getEnv(key, fallback)
	if result != fallback {
		t.Errorf("expected fallback %q for empty env var, got %q", fallback, result)
	}
}
