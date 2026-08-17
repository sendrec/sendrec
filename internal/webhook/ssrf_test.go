package webhook

import (
	"net"
	"testing"
)

// SR-06: user-controlled webhook URLs are an outbound request primitive. Blocking
// has to happen at dial time, not just on the configured string, because DNS
// names and redirects both resolve to addresses the validator never sees.

func TestBlockedDialTargets(t *testing.T) {
	blocked := []string{
		"127.0.0.1",       // loopback
		"127.5.5.5",       // loopback range
		"::1",             // IPv6 loopback
		"10.0.0.5",        // RFC1918
		"172.16.4.1",      // RFC1918
		"192.168.1.10",    // RFC1918
		"169.254.169.254", // link-local / cloud metadata
		"fd00::1",         // IPv6 unique-local
		"fe80::1",         // IPv6 link-local
		"0.0.0.0",         // unspecified
	}

	for _, addr := range blocked {
		ip := net.ParseIP(addr)
		if ip == nil {
			t.Fatalf("bad test fixture %q", addr)
		}
		if !IsBlockedIP(ip) {
			t.Errorf("expected %s to be blocked as an internal target", addr)
		}
	}
}

func TestAllowedDialTargets(t *testing.T) {
	allowed := []string{
		"93.184.216.34",     // public IPv4
		"2606:2800:220:1::", // public IPv6
		"8.8.8.8",
	}

	for _, addr := range allowed {
		ip := net.ParseIP(addr)
		if ip == nil {
			t.Fatalf("bad test fixture %q", addr)
		}
		if IsBlockedIP(ip) {
			t.Errorf("expected %s to be allowed", addr)
		}
	}
}

func TestAllowPrivateTargetsOptsOutForLocalDevelopment(t *testing.T) {
	previous := AllowPrivateTargets
	AllowPrivateTargets = true
	t.Cleanup(func() { AllowPrivateTargets = previous })

	if IsBlockedIP(net.ParseIP("127.0.0.1")) {
		t.Error("loopback must be reachable when explicitly allowed for local development")
	}
}
