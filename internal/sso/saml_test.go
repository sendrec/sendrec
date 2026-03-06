package sso

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testSAMLMetadataXML = `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata"
    entityID="https://idp.example.com">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor use="signing">
      <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
        <ds:X509Data>
          <ds:X509Certificate>MIICwTCCAamgAwIBAgIBATANBgkqhkiG9w0BAQsFADAaMRgwFgYDVQQDEw9pZHAuZXhhbXBsZS5jb20wHhcNMjYwMzA2MTgxOTEzWhcNMzYwMzAzMTkxOTEzWjAaMRgwFgYDVQQDEw9pZHAuZXhhbXBsZS5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCSfurTzXUu6cPlYMHmVrG1E868TWLBUp1WtXmE4hRSDm5LH6OYtsHn9Itz70wpcTEbThTyTsqMybkJkadFKVtuYooOfgJvljMfV3tXOndKuLhYeNI/OJp+K8Wlj3ft24Rs2LJnpkMFynMlHTminigBs56wFSLTDJamGANX5T3Cn6V5SxkZyK42BmfdxRUrE72c6o2steTuMYWjEkpObnYidcY0WSi7uoPvYC5ysP7w69RiMe4j65iEd7xuK82wpScgdV+ERtepwW2Kcxk5t0z9j4SLE5AiaruPkKl2zeYZaJS+9d6XUsPKlAlrtwSv9kl/qY5Jy5wOhGEZNOZ1pmDzAgMBAAGjEjAQMA4GA1UdDwEB/wQEAwIHgDANBgkqhkiG9w0BAQsFAAOCAQEANjUsNdU2gSnOX222b9wApF8aLIIeH5ubLt/bGIVog7ul7tuDanbP8PNP7uYC8UPFcKT4H2fmyOvZ/KpGcngDYJXZyVyWSaKuRLr9p+0AO2sNMYC42pcj7pqoREPeGbFUt5WbrGYVcOaod44SNjkuJpy/b+OjqHqI8WU4kN1UH7NSQkpX8lVFRaV1RCVwOsYiF4AzpgOMsBuP2omau8wy5V1+jrNtwnorS66OOaFi5kNtm/i34CcVwkA1pSapcMT3L2hdo7nbocq0o9dAzTDAlV6SM/PV9nOTzHH1I579L7eWJZylx9VmSJaFdXEB4Bf+nNizD0tnfUdjO4Xc9J3TjA==</ds:X509Certificate>
        </ds:X509Data>
      </ds:KeyInfo>
    </KeyDescriptor>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"
        Location="https://idp.example.com/sso"/>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
        Location="https://idp.example.com/sso/post"/>
  </IDPSSODescriptor>
</EntityDescriptor>`

func TestParseSAMLMetadataFromXML(t *testing.T) {
	cfg, err := ParseSAMLMetadataFromXML([]byte(testSAMLMetadataXML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.EntityID != "https://idp.example.com" {
		t.Errorf("EntityID = %q, want %q", cfg.EntityID, "https://idp.example.com")
	}

	if cfg.SSOURL != "https://idp.example.com/sso" {
		t.Errorf("SSOURL = %q, want %q", cfg.SSOURL, "https://idp.example.com/sso")
	}

	if cfg.Certificate == "" {
		t.Error("Certificate is empty, want non-empty base64 cert")
	}
}

// Azure AD/Entra ID often omits the use attribute on KeyDescriptor.
func TestParseSAMLMetadataFromXML_UnspecifiedKeyUse(t *testing.T) {
	xml := `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata"
    entityID="https://idp.example.com">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor>
      <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
        <ds:X509Data>
          <ds:X509Certificate>MIICwTCCAamgAwIBAgIBATANBgkqhkiG9w0BAQsFADAaMRgwFgYDVQQDEw9pZHAuZXhhbXBsZS5jb20wHhcNMjYwMzA2MTgxOTEzWhcNMzYwMzAzMTkxOTEzWjAaMRgwFgYDVQQDEw9pZHAuZXhhbXBsZS5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCSfurTzXUu6cPlYMHmVrG1E868TWLBUp1WtXmE4hRSDm5LH6OYtsHn9Itz70wpcTEbThTyTsqMybkJkadFKVtuYooOfgJvljMfV3tXOndKuLhYeNI/OJp+K8Wlj3ft24Rs2LJnpkMFynMlHTminigBs56wFSLTDJamGANX5T3Cn6V5SxkZyK42BmfdxRUrE72c6o2steTuMYWjEkpObnYidcY0WSi7uoPvYC5ysP7w69RiMe4j65iEd7xuK82wpScgdV+ERtepwW2Kcxk5t0z9j4SLE5AiaruPkKl2zeYZaJS+9d6XUsPKlAlrtwSv9kl/qY5Jy5wOhGEZNOZ1pmDzAgMBAAGjEjAQMA4GA1UdDwEB/wQEAwIHgDANBgkqhkiG9w0BAQsFAAOCAQEANjUsNdU2gSnOX222b9wApF8aLIIeH5ubLt/bGIVog7ul7tuDanbP8PNP7uYC8UPFcKT4H2fmyOvZ/KpGcngDYJXZyVyWSaKuRLr9p+0AO2sNMYC42pcj7pqoREPeGbFUt5WbrGYVcOaod44SNjkuJpy/b+OjqHqI8WU4kN1UH7NSQkpX8lVFRaV1RCVwOsYiF4AzpgOMsBuP2omau8wy5V1+jrNtwnorS66OOaFi5kNtm/i34CcVwkA1pSapcMT3L2hdo7nbocq0o9dAzTDAlV6SM/PV9nOTzHH1I579L7eWJZylx9VmSJaFdXEB4Bf+nNizD0tnfUdjO4Xc9J3TjA==</ds:X509Certificate>
        </ds:X509Data>
      </ds:KeyInfo>
    </KeyDescriptor>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"
        Location="https://idp.example.com/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`

	cfg, err := ParseSAMLMetadataFromXML([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Certificate == "" {
		t.Error("Certificate is empty, want non-empty base64 cert from unspecified-use KeyDescriptor")
	}
}

func TestParseSAMLMetadataFromXML_InvalidXML(t *testing.T) {
	_, err := ParseSAMLMetadataFromXML([]byte("not xml"))
	if err == nil {
		t.Fatal("expected error for invalid XML, got nil")
	}
}

func TestParseSAMLMetadataFromURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(testSAMLMetadataXML))
	}))
	defer server.Close()

	cfg, err := ParseSAMLMetadataFromURL(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.EntityID != "https://idp.example.com" {
		t.Errorf("EntityID = %q, want %q", cfg.EntityID, "https://idp.example.com")
	}

	if cfg.SSOURL != "https://idp.example.com/sso" {
		t.Errorf("SSOURL = %q, want %q", cfg.SSOURL, "https://idp.example.com/sso")
	}

	if cfg.Certificate == "" {
		t.Error("Certificate is empty, want non-empty base64 cert")
	}
}

func TestParseSAMLMetadataFromURL_BadURL(t *testing.T) {
	_, err := ParseSAMLMetadataFromURL("http://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected error for unreachable URL, got nil")
	}
}

func TestNewSAMLProvider(t *testing.T) {
	cfg, err := ParseSAMLMetadataFromXML([]byte(testSAMLMetadataXML))
	if err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	provider, err := NewSAMLProvider("http://localhost:8080", "org-1", cfg)
	if err != nil {
		t.Fatalf("NewSAMLProvider() error: %v", err)
	}
	if provider == nil {
		t.Fatal("NewSAMLProvider() returned nil")
	}
}

func TestSAMLProvider_AuthURL(t *testing.T) {
	cfg, _ := ParseSAMLMetadataFromXML([]byte(testSAMLMetadataXML))
	provider, _ := NewSAMLProvider("http://localhost:8080", "org-1", cfg)

	authURL := provider.AuthURL("test-state-123")
	if authURL == "" {
		t.Fatal("AuthURL() returned empty string")
	}
	// Should contain the IdP SSO URL
	if !strings.Contains(authURL, "idp.example.com/sso") {
		t.Errorf("AuthURL() = %q, want to contain IdP SSO URL", authURL)
	}
	// RelayState should be present
	if !strings.Contains(authURL, "RelayState=") {
		t.Errorf("AuthURL() = %q, want RelayState parameter", authURL)
	}
}

func TestSAMLProvider_Exchange_InvalidResponse(t *testing.T) {
	cfg, _ := ParseSAMLMetadataFromXML([]byte(testSAMLMetadataXML))
	provider, _ := NewSAMLProvider("http://localhost:8080", "org-1", cfg)

	_, err := provider.Exchange(context.Background(), "not-a-valid-saml-response")
	if err == nil {
		t.Fatal("Exchange() expected error for invalid SAML response")
	}
}
