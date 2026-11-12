package sso

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

// Regression tests for GO-2026-6909 (crewjam/saml): a validly signed assertion
// must not panic when it omits <saml:Issuer>, and a NameID in a foreign
// namespace must not be read as the subject. govulncheck still lists the
// advisory as unfixed; these tests show the version we use is not affected.

func TestSAMLProvider_Exchange_SignedAssertionWithoutIssuer(t *testing.T) {
	_, err := exchangeSignedAssertion(t, `<saml:Subject><saml:NameID>user@example.com</saml:NameID></saml:Subject>`)
	if err == nil {
		t.Fatal("expected error for assertion without Issuer")
	}
}

func TestSAMLProvider_Exchange_ForeignNamespaceNameID(t *testing.T) {
	info, err := exchangeSignedAssertion(t,
		`<saml:Issuer>https://idp.example.com</saml:Issuer>`+
			`<saml:Subject><x:NameID xmlns:x="urn:example:foreign">victim@example.com</x:NameID></saml:Subject>`)
	if err == nil {
		t.Fatalf("expected error for foreign-namespace NameID, got user %+v", info)
	}
}

// exchangeSignedAssertion signs an assertion with the given inner XML using
// the IdP key the provider trusts, wraps it in a Response and exchanges it.
func exchangeSignedAssertion(t *testing.T, assertionInner string) (*UserInfo, error) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "idp.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	provider, err := NewSAMLProvider("http://localhost:8080", "org-1", &SAMLConfig{
		EntityID:    "https://idp.example.com",
		SSOURL:      "https://idp.example.com/sso",
		Certificate: base64.StdEncoding.EncodeToString(certDER),
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	assertionDoc := etree.NewDocument()
	if err := assertionDoc.ReadFromString(fmt.Sprintf(
		`<saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="a1" Version="2.0" IssueInstant="%s">%s</saml:Assertion>`,
		now, assertionInner)); err != nil {
		t.Fatal(err)
	}
	signer := dsig.NewDefaultSigningContext(dsig.TLSCertKeyStore(tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}))
	signer.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	signed, err := signer.SignEnveloped(assertionDoc.Root())
	if err != nil {
		t.Fatal(err)
	}

	responseDoc := etree.NewDocument()
	if err := responseDoc.ReadFromString(fmt.Sprintf(
		`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ID="r1" Version="2.0" IssueInstant="%s" Destination="%s" InResponseTo="req-1">`+
			`<samlp:Status><samlp:StatusCode Value="urn:oasis:names:tc:SAML:2.0:status:Success"/></samlp:Status>`+
			`</samlp:Response>`, now, "http://localhost:8080/api/auth/saml/org-1/acs")); err != nil {
		t.Fatal(err)
	}
	responseDoc.Root().AddChild(signed)
	responseXML, err := responseDoc.WriteToBytes()
	if err != nil {
		t.Fatal(err)
	}

	info, err := provider.ExchangeWithRequestID(context.Background(), base64.StdEncoding.EncodeToString(responseXML), "req-1")
	if err != nil && !strings.Contains(err.Error(), "parse SAML response") {
		t.Fatalf("unexpected error kind: %v", err)
	}
	return info, err
}
