package server

import "net/http"

func securityHeaders(baseURL string) func(http.Handler) http.Handler {
	strictTransport := baseURL != "" && hasHTTPS(baseURL)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), screen-wake-lock=(), display-capture=(self)")

			// Allow only our origin resources; permit data: for webm playback previews
			w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; media-src 'self' data:; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'")

			if strictTransport {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}

func hasHTTPS(baseURL string) bool {
	return len(baseURL) >= 8 && baseURL[:8] == "https://"
}
