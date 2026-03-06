package sso

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const testSAMLMetadataXML = `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata"
    entityID="https://idp.example.com">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor use="signing">
      <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
        <ds:X509Data>
          <ds:X509Certificate>MIIDpDCCAoygAwIBAgIGAX0zjYiPMA0GCSqGSIb3DQEBCwUAMIGSMQswCQYDVQQGEwJVUzETMBEGA1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzENMAsGA1UECgwET2t0YTEUMBIGA1UECwwLU1NPUHJvdmlkZXIxEzARBgNVBAMMCmRldi04NDI4NTcxHDAaBgkqhkiG9w0BCQEWDWluZm9Ab2t0YS5jb20wHhcNMjEwMTA1MTcwMjI3WhcNMzEwMTA1MTcwMzI3WjCBkjELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNVBAoMBE9rdGExFDASBgNVBAsMC1NTT1Byb3ZpZGVyMRMwEQYDVQQDDApkZXYtODQyODU3MRwwGgYJKoZIhvcNAQkBFg1pbmZvQG9rdGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA2kGLOaSVCxMR0jNDw8PSXO7kkxMO6hKl17OEFgGkhMOBZ0kNpMgSJgUUNJOxjmNSP8aRnRJFBOqXnEF1JFBX0MI0GxEu1MxejqVP9PjWUPVQEWLhGbh6aEsnI1T9Sq7hQmbxLWeX03RoIUJSt9ICOI0r3TBdJuXkHwHB5B+HQKWM1SyBk+vVNq7aGIKhTz+8hUozkv4K2YPzCSf5eHzVjwdNqOpjINdLEKfDBSjzKrOljBCXCSflxCezd80kMySL5prl+XJ6yeEZ5xQf/MGDM5F2DOaAfXNvEVTOz0zNNOMEfS+FzJC1wElszKlMPBx4bJq5LVF+SVzf1QIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQBIAHLmI/7MX/OJSNz7cR+HJ+BQ9NVS3J1fOtSYO0mRIZVK9aQyzASPSazyXL4IH8arXKSPz2q7YvHMHLJg7g+Y2yEfGjPn5MZk+M9iHAnIUH+C69UFBR4NLXLV7j7sSR6LMjMIBMbFEWSeXpHzJqfMs5cY/crgBDuJt6l07jBEELeXEyOCq6SvJIGbPDtVCxL7qRZ3LBhMWJlUZ+gQEzKMDL/MCHrGWVW4rbTLXRnLh0plRanGlKRhOQJPnvmDqdEBY15TZzYBN69v2K+GhdlN0b/CQCQAB7N8e9Fdc9H0mWRWYaXMNjCOcELj+JxE7BR7nRJqsxncToh03H</ds:X509Certificate>
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
