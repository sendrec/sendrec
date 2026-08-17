package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// trustProxy sets the deployment-wide trust flag for one test and restores it.
func trustProxy(t *testing.T, trusted bool) {
	t.Helper()
	previous := TrustProxyHeaders
	TrustProxyHeaders = trusted
	t.Cleanup(func() { TrustProxyHeaders = previous })
}

func TestClientIPStripsEphemeralPort(t *testing.T) {
	trustProxy(t, false)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.10:54321"

	if got := ClientIP(request); got != "192.0.2.10" {
		t.Errorf("expected port stripped from RemoteAddr, got %q", got)
	}
}

func TestClientIPIsStableAcrossConnectionsFromSameHost(t *testing.T) {
	trustProxy(t, false)

	first := httptest.NewRequest(http.MethodGet, "/", nil)
	first.RemoteAddr = "192.0.2.10:1111"
	second := httptest.NewRequest(http.MethodGet, "/", nil)
	second.RemoteAddr = "192.0.2.10:2222"

	if ClientIP(first) != ClientIP(second) {
		t.Errorf("same host on new connections must key alike, got %q and %q", ClientIP(first), ClientIP(second))
	}
}

func TestClientIPIgnoresForwardedForWhenProxyUntrusted(t *testing.T) {
	trustProxy(t, false)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.10:54321"
	request.Header.Set("X-Forwarded-For", "203.0.113.7")

	if got := ClientIP(request); got != "192.0.2.10" {
		t.Errorf("client-supplied X-Forwarded-For must be ignored, got %q", got)
	}
}

func TestClientIPUsesLastForwardedEntryWhenProxyTrusted(t *testing.T) {
	trustProxy(t, true)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.1:54321"
	// The client forged the first entry; the trusted proxy appended the peer it saw.
	request.Header.Set("X-Forwarded-For", "1.2.3.4, 203.0.113.7")

	if got := ClientIP(request); got != "203.0.113.7" {
		t.Errorf("expected proxy-appended last entry, got %q", got)
	}
}

func TestClientIPFallsBackToRemoteAddrWhenForwardedAbsent(t *testing.T) {
	trustProxy(t, true)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.10:54321"

	if got := ClientIP(request); got != "192.0.2.10" {
		t.Errorf("expected RemoteAddr host when header absent, got %q", got)
	}
}

func TestClientIPHandlesIPv6(t *testing.T) {
	trustProxy(t, false)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "[2001:db8::1]:54321"

	if got := ClientIP(request); got != "2001:db8::1" {
		t.Errorf("expected IPv6 host without port, got %q", got)
	}
}

func TestClientIPIgnoresEmptyForwardedEntry(t *testing.T) {
	trustProxy(t, true)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.10:54321"
	request.Header.Set("X-Forwarded-For", "203.0.113.7, ")

	if got := ClientIP(request); got != "192.0.2.10" {
		t.Errorf("expected fallback when trailing entry is blank, got %q", got)
	}
}
