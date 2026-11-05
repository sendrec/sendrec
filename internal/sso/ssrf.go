package sso

import (
	"net/http"

	"github.com/sendrec/sendrec/internal/webhook"
)

// AllowPrivateIssuers permits workspace SSO to reach identity providers on
// private addresses. Off by default; set from SSO_ALLOW_PRIVATE_ISSUERS for
// self-hosted instances whose identity provider is on an internal network.
var AllowPrivateIssuers bool

var safeWorkspaceClient = &http.Client{
	Timeout:   metadataFetchTimeout,
	Transport: webhook.NewSafeTransport(),
}

var privateWorkspaceClient = &http.Client{Timeout: metadataFetchTimeout}

// workspaceHTTPClient returns the client for URLs set by workspace admins: OIDC
// issuers and SAML metadata. Those admins are users, not operators, so their
// URLs must not reach internal services. Providers configured through the
// environment are trusted and don't use it.
func workspaceHTTPClient() *http.Client {
	if AllowPrivateIssuers {
		return privateWorkspaceClient
	}
	return safeWorkspaceClient
}
