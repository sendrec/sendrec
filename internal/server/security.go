package server

import (
	"fmt"
	"net/http"

	"github.com/sendrec/sendrec/internal/httputil"
)

type SecurityConfig struct {
	BaseURL         string
	StorageEndpoint string
}

func securityHeaders(cfg SecurityConfig) func(http.Handler) http.Handler {
	strictTransport := cfg.BaseURL != "" && hasHTTPS(cfg.BaseURL)

	storageSuffix := ""
	if cfg.StorageEndpoint != "" {
		storageSuffix = " " + cfg.StorageEndpoint
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nonce := httputil.GenerateNonce()
			ctx := httputil.ContextWithNonce(r.Context(), nonce)

			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			w.Header().Set("Permissions-Policy", "camera=(self), microphone=(self), geolocation=(), screen-wake-lock=(), display-capture=(self)")

			csp := fmt.Sprintf(
				"default-src 'self'; img-src 'self' data:%s; media-src 'self' data:%s; script-src 'self' 'nonce-%s'; style-src 'self' 'nonce-%s'; connect-src 'self'%s; frame-ancestors 'self';",
				storageSuffix, storageSuffix, nonce, nonce, storageSuffix,
			)
			w.Header().Set("Content-Security-Policy", csp)

			if strictTransport {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func hasHTTPS(baseURL string) bool {
	return len(baseURL) >= 8 && baseURL[:8] == "https://"
}
