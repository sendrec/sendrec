package docs

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var specYAML []byte

func HandleSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(specYAML)
}

func HandleDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; "+
			"script-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; "+
			"style-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; "+
			"font-src 'self' https://cdn.jsdelivr.net data:; "+
			"img-src 'self' data:; connect-src 'self'; frame-ancestors 'self';")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(docsHTML))
}

const docsHTML = `<!DOCTYPE html>
<html><head>
  <title>SendRec API Reference</title>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
</head><body>
  <script id="api-reference" data-url="/api/docs/openapi.yaml"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body></html>`
