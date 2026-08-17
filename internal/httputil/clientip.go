package httputil

import (
	"net"
	"net/http"
	"strings"
)

// TrustProxyHeaders reports whether X-Forwarded-For may be believed. It is a
// deployment-wide fact (set once from TRUSTED_PROXY before the server starts),
// so it lives here rather than being threaded through every limiter and handler.
// Left false, the header is ignored entirely: any client can forge it.
var TrustProxyHeaders bool

// ClientIP returns the address used to key rate limiting and viewer dedup.
//
// Two things matter here. RemoteAddr carries an ephemeral port, so it must be
// split or every new TCP connection lands in its own rate-limit bucket. And
// X-Forwarded-For is only meaningful behind a proxy that appends to it — there
// the last entry is the peer the proxy actually saw, while every earlier entry
// is client-supplied and worthless for trust decisions.
func ClientIP(r *http.Request) string {
	if TrustProxyHeaders {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			entries := strings.Split(forwarded, ",")
			if last := strings.TrimSpace(entries[len(entries)-1]); last != "" {
				return last
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
