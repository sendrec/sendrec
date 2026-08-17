package webhook

import (
	"fmt"
	"net"
	"syscall"
)

// AllowPrivateTargets permits webhooks to reach loopback and private addresses.
// Off in production; set from WEBHOOK_ALLOW_PRIVATE_TARGETS so local development
// can point a webhook at a listener on the developer's own machine.
var AllowPrivateTargets bool

// IsBlockedIP reports whether an address belongs to the infrastructure rather
// than the internet — loopback, RFC1918, link-local (including the cloud
// metadata endpoint at 169.254.169.254), and unique-local IPv6.
func IsBlockedIP(ip net.IP) bool {
	if AllowPrivateTargets {
		return false
	}

	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsUnspecified()
}

// safeDialControl is the net.Dialer Control hook wired into the webhook client;
// it rejects connections to internal addresses at dial time.
//
// Validating the configured URL is not enough: the hostname may resolve to an
// internal address, may resolve differently on the second lookup, or the target
// may redirect. The dialer sees the address actually being connected to on every
// hop, so this is the one place the check cannot be routed around.
func safeDialControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("webhook: cannot parse dial address %q: %w", address, err)
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("webhook: unresolved dial address %q", host)
	}

	if IsBlockedIP(ip) {
		return fmt.Errorf("webhook: refusing to connect to internal address %s", ip)
	}

	return nil
}
