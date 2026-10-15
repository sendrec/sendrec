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

// A self-hosted install has no billing, so nothing could ever raise its
// limits: it defaults to unlimited. The hosted service sells plans, sets none
// of these variables, and keeps the free plan's limits. An explicit value
// always wins. #255, audit 1.4.
func TestFreeLimit(t *testing.T) {
	for _, c := range []struct {
		name    string
		env     string
		billing bool
		want    int
	}{
		{"self-hosted default", "", false, 0},
		{"hosted default", "", true, 25},
		{"self-hosted explicit", "10", false, 10},
		{"hosted explicit", "0", true, 0},
		{"unparsable keeps the default", "lots", false, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("MAX_TEST_LIMIT", c.env)
			if got := freeLimit("MAX_TEST_LIMIT", 25, c.billing); got != c.want {
				t.Errorf("freeLimit = %d, want %d", got, c.want)
			}
		})
	}
}

func TestSocialRegistrationEnabled(t *testing.T) {
	for _, c := range []struct {
		name         string
		env          string
		registration bool
		want         bool
	}{
		{"unset follows open registration", "", true, true},
		{"unset follows invite-only", "", false, false},
		{"on while the form is closed", "true", false, true},
		{"off while the form is open", "false", true, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("SOCIAL_REGISTRATION_ENABLED", c.env)
			if got := socialRegistrationEnabled(c.registration); got != c.want {
				t.Errorf("socialRegistrationEnabled = %v, want %v", got, c.want)
			}
		})
	}
}
