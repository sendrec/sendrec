package sso

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	const maxMetadataSize = 1 << 20 // 1 MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataSize))
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
// first KeyDescriptor with use="signing" or unspecified use (empty string).
// Some IdPs like Azure AD/Entra ID omit the use attribute. Whitespace is
// stripped from the raw certificate data.
func findSigningCertificate(descriptors []saml.KeyDescriptor) string {
	for _, kd := range descriptors {
		if kd.Use != "signing" && kd.Use != "" {
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
// metadata into an *x509.Certificate. It tolerates missing padding and
// embedded whitespace, which are common in SAML metadata.
func parseCertificate(certBase64 string) (*x509.Certificate, error) {
	cleaned := strings.NewReplacer(" ", "", "\n", "", "\r", "", "\t", "").Replace(certBase64)

	decoded, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(cleaned, "="))
	if err != nil {
		return nil, fmt.Errorf("decode certificate base64: %w", err)
	}

	cert, err := x509.ParseCertificate(decoded)
	if err != nil {
		return nil, fmt.Errorf("parse X.509 certificate: %w", err)
	}

	return cert, nil
}

// SAMLProvider implements the Provider interface for SAML 2.0 identity
// providers using the crewjam/saml service provider.
type SAMLProvider struct {
	sp saml.ServiceProvider
}

// NewSAMLProvider creates a SAMLProvider from the given base URL, organisation
// ID, and IdP configuration. It parses the IdP certificate and constructs a
// crewjam/saml ServiceProvider with the correct entity ID, ACS URL, and IdP
// metadata.
func NewSAMLProvider(baseURL, orgID string, cfg *SAMLConfig) (*SAMLProvider, error) {
	cert, err := parseCertificate(cfg.Certificate)
	if err != nil {
		return nil, fmt.Errorf("parse IdP certificate: %w", err)
	}

	metadataPath := baseURL + "/api/auth/saml/" + orgID + "/metadata"
	acsPath := baseURL + "/api/auth/saml/" + orgID + "/acs"

	metadataURL, err := url.Parse(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("parse metadata URL: %w", err)
	}

	acsURL, err := url.Parse(acsPath)
	if err != nil {
		return nil, fmt.Errorf("parse ACS URL: %w", err)
	}

	sp := saml.ServiceProvider{
		EntityID:    metadataPath,
		MetadataURL: *metadataURL,
		AcsURL:      *acsURL,
		IDPMetadata: &saml.EntityDescriptor{
			EntityID: cfg.EntityID,
			IDPSSODescriptors: []saml.IDPSSODescriptor{
				{
					SingleSignOnServices: []saml.Endpoint{
						{
							Binding:  saml.HTTPRedirectBinding,
							Location: cfg.SSOURL,
						},
					},
					SSODescriptor: saml.SSODescriptor{
						RoleDescriptor: saml.RoleDescriptor{
							KeyDescriptors: []saml.KeyDescriptor{
								{
									Use: "signing",
									KeyInfo: saml.KeyInfo{
										X509Data: saml.X509Data{
											X509Certificates: []saml.X509Certificate{
												{Data: cfg.Certificate},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		IDPCertificate: ptrString(base64.StdEncoding.EncodeToString(cert.Raw)),
	}

	return &SAMLProvider{sp: sp}, nil
}

func ptrString(s string) *string {
	return &s
}

// AuthURL generates a SAML authentication request URL that redirects the user
// to the IdP. The state parameter is passed as RelayState.
// This satisfies the Provider interface but discards the request ID.
func (p *SAMLProvider) AuthURL(state string) string {
	redirectURL, _, _ := p.AuthRequestURL(state)
	return redirectURL
}

// AuthRequestURL generates a SAML authentication request URL and returns
// both the redirect URL and the AuthnRequest ID. The handler must store
// the request ID so it can be passed to ExchangeWithRequestID for
// InResponseTo validation.
func (p *SAMLProvider) AuthRequestURL(state string) (string, string, error) {
	authReq, err := p.sp.MakeAuthenticationRequest(
		p.sp.GetSSOBindingLocation(saml.HTTPRedirectBinding),
		saml.HTTPRedirectBinding,
		saml.HTTPPostBinding,
	)
	if err != nil {
		return "", "", fmt.Errorf("make AuthnRequest: %w", err)
	}

	redirectURL, err := authReq.Redirect(state, &p.sp)
	if err != nil {
		return "", "", fmt.Errorf("build redirect URL: %w", err)
	}

	return redirectURL.String(), authReq.ID, nil
}

// SAML attribute URIs and short names used to extract email and display name
// from assertion attribute statements.
var (
	emailAttributeNames = []string{
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
		"email",
		"Email",
		"mail",
	}
	nameAttributeNames = []string{
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
		"displayName",
		"name",
		"Name",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname",
	}
)

// Exchange validates a base64-encoded SAMLResponse and extracts user identity
// claims from the resulting assertion. This satisfies the Provider interface
// but skips InResponseTo validation. Prefer ExchangeWithRequestID.
func (p *SAMLProvider) Exchange(_ context.Context, samlResponse string) (*UserInfo, error) {
	return p.exchange(samlResponse, nil)
}

// ExchangeWithRequestID validates a SAMLResponse with InResponseTo checking
// against the given request ID from the original AuthnRequest.
func (p *SAMLProvider) ExchangeWithRequestID(_ context.Context, samlResponse, requestID string) (*UserInfo, error) {
	return p.exchange(samlResponse, []string{requestID})
}

func (p *SAMLProvider) exchange(samlResponse string, requestIDs []string) (*UserInfo, error) {
	req, err := http.NewRequest(http.MethodPost, p.sp.AcsURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build synthetic request: %w", err)
	}
	req.PostForm = url.Values{
		"SAMLResponse": {samlResponse},
	}

	assertion, err := p.sp.ParseResponse(req, requestIDs)
	if err != nil {
		// crewjam/saml hides details in InvalidResponseError.PrivateErr;
		// surface the private error for server-side logging.
		var invalidResp *saml.InvalidResponseError
		if errors.As(err, &invalidResp) && invalidResp.PrivateErr != nil {
			return nil, fmt.Errorf("parse SAML response: %v: %w", invalidResp.PrivateErr, err)
		}
		return nil, fmt.Errorf("parse SAML response: %w", err)
	}

	email := extractAttribute(assertion, emailAttributeNames)
	name := extractAttribute(assertion, nameAttributeNames)

	nameID := ""
	if assertion.Subject != nil && assertion.Subject.NameID != nil {
		nameID = assertion.Subject.NameID.Value
	}

	if email == "" && nameID != "" && strings.Contains(nameID, "@") {
		email = nameID
	}
	if email == "" {
		return nil, fmt.Errorf("no email found in SAML assertion")
	}

	externalID := nameID
	if externalID == "" {
		externalID = email
	}

	return &UserInfo{
		ExternalID: externalID,
		Email:      email,
		Name:       name,
	}, nil
}

// extractAttribute searches assertion attribute statements for the first
// matching attribute name and returns its value.
func extractAttribute(assertion *saml.Assertion, names []string) string {
	for _, stmt := range assertion.AttributeStatements {
		for _, attr := range stmt.Attributes {
			for _, target := range names {
				if attr.Name == target {
					if len(attr.Values) > 0 && attr.Values[0].Value != "" {
						return attr.Values[0].Value
					}
				}
			}
		}
	}
	return ""
}
