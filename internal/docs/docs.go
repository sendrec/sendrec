package docs

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var specYAML []byte

// Vendored from @scalar/api-reference 1.69.2. It is served from this binary
// rather than a CDN because our sub-processors page names every company that
// receives data, and a CDN serving this page to a browser is one of them.
// Refresh it with:
//
//	curl -sL https://cdn.jsdelivr.net/npm/@scalar/api-reference@<version> \
//	  -o internal/docs/scalar.js
//
//go:embed scalar.js
var scalarJS []byte

func HandleSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(specYAML)
}

func HandleDocs(w http.ResponseWriter, r *http.Request) {
	// Nothing off this origin. That is the claim the sub-processors page makes,
	// and this header is what keeps it true if Scalar grows a new fetch.
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; "+
			"script-src 'self' 'unsafe-inline'; "+
			"style-src 'self' 'unsafe-inline'; "+
			"font-src 'self' data:; "+
			"img-src 'self' data:; connect-src 'self'; frame-ancestors 'self';")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(docsHTML))
}

func HandleScalarBundle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	// The version is pinned in the repository, so the bytes only change when we
	// change them.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(scalarJS)
}

const docsHTML = `<!DOCTYPE html>
<html><head>
  <title>SendRec API Reference</title>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
</head><body>
  <script id="api-reference" data-url="/api/docs/openapi.yaml"
    data-configuration='{"withDefaultFonts":false,"proxyUrl":""}'></script>
  <script src="/api/docs/scalar.js"></script>
</body></html>`
