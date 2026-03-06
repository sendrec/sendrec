package sso

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/crewjam/saml"
)

// SAMLConfig holds the essential identity provider settings extracted from
// SAML metadata XML: entity ID, SSO endpoint URL, and the base64-encoded
// X.509 signing certificate.
type SAMLConfig struct {
	EntityID    string
	SSOURL      string
	Certificate string
}

// ParseSAMLMetadataFromXML parses SAML IdP metadata XML and extracts the
// entity ID, preferred SSO URL (HTTP-Redirect preferred, HTTP-POST fallback),
// and the signing certificate.
func ParseSAMLMetadataFromXML(data []byte) (*SAMLConfig, error) {
	var descriptor saml.EntityDescriptor
	if err := xml.Unmarshal(data, &descriptor); err != nil {
		return nil, fmt.Errorf("unmarshal SAML metadata: %w", err)
	}

	if len(descriptor.IDPSSODescriptors) == 0 {
		return nil, fmt.Errorf("no IDPSSODescriptor found in metadata")
	}

	idp := descriptor.IDPSSODescriptors[0]

	ssoURL := findSSOURL(idp.SingleSignOnServices)
	if ssoURL == "" {
		return nil, fmt.Errorf("no SSO service endpoint found in metadata")
	}

	cert := findSigningCertificate(idp.KeyDescriptors)
	if cert == "" {
		return nil, fmt.Errorf("no signing certificate found in metadata")
	}

	return &SAMLConfig{
		EntityID:    descriptor.EntityID,
		SSOURL:      ssoURL,
		Certificate: cert,
	}, nil
}

const metadataFetchTimeout = 30 * time.Second

// ParseSAMLMetadataFromURL fetches SAML IdP metadata from the given URL and
// parses it using ParseSAMLMetadataFromXML.
func ParseSAMLMetadataFromURL(metadataURL string) (*SAMLConfig, error) {
	client := &http.Client{Timeout: metadataFetchTimeout}

	resp, err := client.Get(metadataURL)
	if err != nil {
		return nil, fmt.Errorf("fetch SAML metadata: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch SAML metadata: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read SAML metadata response: %w", err)
	}

	return ParseSAMLMetadataFromXML(body)
}

// findSSOURL returns the SSO endpoint URL, preferring HTTP-Redirect binding
// over HTTP-POST. Returns an empty string if no suitable endpoint is found.
func findSSOURL(services []saml.Endpoint) string {
	var postURL string
	for _, svc := range services {
		if svc.Binding == saml.HTTPRedirectBinding {
			return svc.Location
		}
		if svc.Binding == saml.HTTPPostBinding && postURL == "" {
			postURL = svc.Location
		}
	}
	return postURL
}

// findSigningCertificate returns the base64-encoded certificate data from the
// first KeyDescriptor with use="signing". Whitespace is stripped from the
// raw certificate data.
func findSigningCertificate(descriptors []saml.KeyDescriptor) string {
	for _, kd := range descriptors {
		if kd.Use != "signing" {
			continue
		}
		for _, cert := range kd.KeyInfo.X509Data.X509Certificates {
			data := strings.TrimSpace(cert.Data)
			if data != "" {
				return data
			}
		}
	}
	return ""
}

// parseCertificate decodes a base64-encoded X.509 certificate from SAML
// metadata into an *x509.Certificate.
func parseCertificate(certBase64 string) (*x509.Certificate, error) {
	decoded, err := base64.StdEncoding.DecodeString(certBase64)
	if err != nil {
		return nil, fmt.Errorf("decode certificate base64: %w", err)
	}

	cert, err := x509.ParseCertificate(decoded)
	if err != nil {
		return nil, fmt.Errorf("parse X.509 certificate: %w", err)
	}

	return cert, nil
}
