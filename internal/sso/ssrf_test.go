package sso

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pashagolub/pgxmock/v6"

	"github.com/sendrec/sendrec/internal/integration"
)

func allowPrivateIssuers(t *testing.T) {
	t.Helper()
	previous := AllowPrivateIssuers
	AllowPrivateIssuers = true
	t.Cleanup(func() { AllowPrivateIssuers = previous })
}

// newInternalServer stands in for a service on the private network. httptest
// listens on loopback, so any request it counts got past the guard.
func newInternalServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(testSAMLMetadataXML))
	}))
	t.Cleanup(server.Close)
	return server, &hits
}

func TestParseSAMLMetadataFromURL_RefusesInternalAddress(t *testing.T) {
	server, hits := newInternalServer(t)

	if _, err := ParseSAMLMetadataFromURL(server.URL); err == nil {
		t.Fatal("expected an error fetching metadata from an internal address")
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("internal server received %d requests, want 0", got)
	}
}

func TestInitiateOrgSSO_RefusesInternalIssuer(t *testing.T) {
	server, hits := newInternalServer(t)
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	encryptedSecret, err := integration.Encrypt(testEncryptionKey, "test-client-secret")
	if err != nil {
		t.Fatalf("encrypt client secret: %v", err)
	}
	issuerURL := server.URL
	clientID := "test-client-id"
	mock.ExpectQuery(`SELECT o.id, c.provider, c.issuer_url, c.client_id, c.client_secret_encrypted`).
		WithArgs("user@example.com").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "provider", "issuer_url", "client_id", "client_secret_encrypted",
			"saml_entity_id", "saml_sso_url", "saml_certificate",
		}).AddRow("org-1", "oidc", &issuerURL, &clientID, &encryptedSecret,
			(*string)(nil), (*string)(nil), (*string)(nil)))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/org?email=user@example.com", nil)
	rec := httptest.NewRecorder()
	handler.InitiateOrgSSO(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("internal server received %d requests, want 0", got)
	}
}

func TestOrgCallback_RefusesInternalIssuer(t *testing.T) {
	server, hits := newInternalServer(t)
	handler, mock := newTestHandlerWithKey(t)
	defer mock.Close()

	encryptedSecret, err := integration.Encrypt(testEncryptionKey, "test-client-secret")
	if err != nil {
		t.Fatalf("encrypt client secret: %v", err)
	}
	issuerURL := server.URL
	clientID := "test-client-id"
	mock.ExpectQuery(`SELECT issuer_url, client_id, client_secret_encrypted FROM organization_sso_configs`).
		WithArgs("org-1").
		WillReturnRows(pgxmock.NewRows([]string{"issuer_url", "client_id", "client_secret_encrypted"}).
			AddRow(&issuerURL, &clientID, &encryptedSecret))

	state := "valid-org-state"
	req := httptest.NewRequest(http.MethodGet, "/api/auth/sso/org/callback?code=auth-code&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: "sso_state", Value: state})
	req.AddCookie(&http.Cookie{Name: "sso_org", Value: "org-1"})
	rec := httptest.NewRecorder()
	handler.OrgCallback(rec, req)

	if !strings.Contains(rec.Header().Get("Location"), "sso_error=") {
		t.Fatalf("Location = %q, want sso_error=", rec.Header().Get("Location"))
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("internal server received %d requests, want 0", got)
	}
}
