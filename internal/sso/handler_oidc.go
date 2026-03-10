package sso

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/integration"
)

// Initiate starts the SSO flow by generating a state parameter, setting
// the state cookie, and redirecting to the provider's authorization URL.
func (h *Handler) Initiate(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	provider, ok := h.providers[providerName]
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "unknown SSO provider")
		return
	}

	state, err := generateState()
	if err != nil {
		slog.Error("sso: failed to generate state", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to initiate SSO")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "sso_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})

	http.Redirect(w, r, provider.AuthURL(state), http.StatusFound)
}

// Callback handles the provider's redirect after authentication. It validates
// the state parameter, exchanges the authorization code for user info, resolves
// or creates the local user, issues JWT tokens, and redirects to the frontend.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	provider, ok := h.providers[providerName]
	if !ok {
		h.redirectWithError(w, r, "unknown provider")
		return
	}

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		if desc == "" {
			desc = "login was denied"
		}
		h.redirectWithError(w, r, desc)
		return
	}

	stateCookie, err := r.Cookie("sso_state")
	if err != nil {
		h.redirectWithError(w, r, "missing state cookie")
		return
	}

	// Clear the state cookie immediately.
	http.SetCookie(w, &http.Cookie{
		Name:     "sso_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	queryState := r.URL.Query().Get("state")
	if queryState == "" || queryState != stateCookie.Value {
		h.redirectWithError(w, r, "invalid state parameter")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		h.redirectWithError(w, r, "missing authorization code")
		return
	}

	info, err := provider.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("sso: exchange failed", "provider", providerName, "error", err)
		h.redirectWithError(w, r, "authentication failed")
		return
	}

	userID, err := h.resolveUser(r.Context(), providerName, info)
	if err != nil {
		slog.Error("sso: resolve user failed", "provider", providerName, "error", err)
		h.redirectWithError(w, r, err.Error())
		return
	}

	accessToken, refreshToken, err := auth.IssueTokens(r.Context(), h.db, h.jwtSecret, userID)
	if err != nil {
		slog.Error("sso: issue tokens failed", "error", err)
		h.redirectWithError(w, r, "failed to create session")
		return
	}

	auth.SetRefreshTokenCookie(w, refreshToken, h.secureCookies)
	redirectURL := h.baseURL + "/login?sso_token=" + url.QueryEscape(accessToken)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *Handler) saveOIDCConfig(w http.ResponseWriter, r *http.Request, orgID string, req *ssoConfigRequest) {
	if req.IssuerURL == "" || req.ClientID == "" {
		httputil.WriteError(w, http.StatusBadRequest, "issuer_url and client_id are required")
		return
	}

	// If client_secret is empty, preserve the existing encrypted secret.
	var encryptedSecret string
	if req.ClientSecret == "" {
		err := h.db.QueryRow(r.Context(),
			"SELECT client_secret_encrypted FROM organization_sso_configs WHERE organization_id = $1",
			orgID,
		).Scan(&encryptedSecret)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "client_secret is required")
			return
		}
	} else {
		var err error
		encryptedSecret, err = integration.Encrypt(h.encryptionKey, req.ClientSecret)
		if err != nil {
			slog.Error("sso: failed to encrypt client secret", "error", err)
			httputil.WriteError(w, http.StatusInternalServerError, "failed to save SSO config")
			return
		}
	}

	if _, err := h.db.Exec(r.Context(),
		`INSERT INTO organization_sso_configs (organization_id, provider, issuer_url, client_id, client_secret_encrypted, enforce_sso,
		     saml_metadata_url, saml_entity_id, saml_sso_url, saml_certificate, saml_metadata_xml)
		 VALUES ($1, 'oidc', $2, $3, $4, $5, NULL, NULL, NULL, NULL, NULL)
		 ON CONFLICT (organization_id) DO UPDATE
		 SET provider = 'oidc',
		     issuer_url = EXCLUDED.issuer_url,
		     client_id = EXCLUDED.client_id,
		     client_secret_encrypted = EXCLUDED.client_secret_encrypted,
		     enforce_sso = EXCLUDED.enforce_sso,
		     saml_metadata_url = NULL,
		     saml_entity_id = NULL,
		     saml_sso_url = NULL,
		     saml_certificate = NULL,
		     saml_metadata_xml = NULL,
		     updated_at = now()`,
		orgID, req.IssuerURL, req.ClientID, encryptedSecret, req.EnforceSSO,
	); err != nil {
		slog.Error("sso: failed to upsert SSO config", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to save SSO config")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// InitiateOrgSSO starts the SSO flow for a user based on their email's
// organization membership. It looks up the SSO config for the organization
// the user belongs to and redirects to the identity provider.
// If orgId is provided, it scopes the lookup to that specific organization.
func (h *Handler) InitiateOrgSSO(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		httputil.WriteError(w, http.StatusBadRequest, "email is required")
		return
	}

	var orgID string
	var configProvider string
	var issuerURL, clientID, encryptedSecret *string
	var samlEntityID, samlSSOURL, samlCertificate *string

	orgFilter := r.URL.Query().Get("org")
	if orgFilter != "" {
		err := h.db.QueryRow(r.Context(),
			`SELECT o.id, c.provider, c.issuer_url, c.client_id, c.client_secret_encrypted,
			        c.saml_entity_id, c.saml_sso_url, c.saml_certificate
			 FROM organization_sso_configs c
			 JOIN organizations o ON o.id = c.organization_id
			 JOIN organization_members m ON m.organization_id = o.id
			 JOIN users u ON u.id = m.user_id
			 WHERE u.email = $1 AND o.id = $2`,
			email, orgFilter,
		).Scan(&orgID, &configProvider, &issuerURL, &clientID, &encryptedSecret,
			&samlEntityID, &samlSSOURL, &samlCertificate)
		if err != nil {
			httputil.WriteError(w, http.StatusNotFound, "no SSO configuration found for this workspace")
			return
		}
	} else {
		err := h.db.QueryRow(r.Context(),
			`SELECT o.id, c.provider, c.issuer_url, c.client_id, c.client_secret_encrypted,
			        c.saml_entity_id, c.saml_sso_url, c.saml_certificate
			 FROM organization_sso_configs c
			 JOIN organizations o ON o.id = c.organization_id
			 JOIN organization_members m ON m.organization_id = o.id
			 JOIN users u ON u.id = m.user_id
			 WHERE u.email = $1 LIMIT 1`,
			email,
		).Scan(&orgID, &configProvider, &issuerURL, &clientID, &encryptedSecret,
			&samlEntityID, &samlSSOURL, &samlCertificate)
		if err != nil {
			httputil.WriteError(w, http.StatusNotFound, "no SSO configuration found for this email")
			return
		}
	}

	state, err := generateState()
	if err != nil {
		slog.Error("sso: failed to generate state", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to initiate SSO")
		return
	}

	if configProvider == "saml" {
		if samlEntityID == nil || samlSSOURL == nil || samlCertificate == nil {
			slog.Error("sso: incomplete SAML config", "orgID", orgID)
			httputil.WriteError(w, http.StatusInternalServerError, "incomplete SAML configuration")
			return
		}

		samlProvider, err := NewSAMLProvider(h.baseURL, orgID, &SAMLConfig{
			EntityID:    *samlEntityID,
			SSOURL:      *samlSSOURL,
			Certificate: *samlCertificate,
		})
		if err != nil {
			slog.Error("sso: failed to create SAML provider", "error", err)
			httputil.WriteError(w, http.StatusInternalServerError, "failed to initiate SSO")
			return
		}

		redirectURL, requestID, err := samlProvider.AuthRequestURL(state)
		if err != nil {
			slog.Error("sso: failed to build SAML AuthnRequest", "error", err)
			httputil.WriteError(w, http.StatusInternalServerError, "failed to initiate SSO")
			return
		}

		// Store state + requestID in cookie. The requestID is needed for
		// InResponseTo validation when the IdP POSTs back to the ACS.
		// SameSite=None is required because the IdP POSTs back cross-origin.
		http.SetCookie(w, &http.Cookie{
			Name:     "sso_state",
			Value:    state + "|" + requestID,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteNoneMode,
			MaxAge:   300,
		})

		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// OIDC flow.
	if issuerURL == nil || clientID == nil || encryptedSecret == nil {
		slog.Error("sso: incomplete OIDC config", "orgID", orgID)
		httputil.WriteError(w, http.StatusInternalServerError, "incomplete OIDC configuration")
		return
	}

	clientSecret, err := integration.Decrypt(h.encryptionKey, *encryptedSecret)
	if err != nil {
		slog.Error("sso: failed to decrypt client secret", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to initiate SSO")
		return
	}

	provider, err := NewOIDCProvider(r.Context(), OIDCConfig{
		IssuerURL:    *issuerURL,
		ClientID:     *clientID,
		ClientSecret: clientSecret,
		RedirectURL:  h.baseURL + "/api/auth/sso/org/callback",
	})
	if err != nil {
		slog.Error("sso: failed to create OIDC provider", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to initiate SSO")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "sso_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "sso_org",
		Value:    orgID,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})

	http.Redirect(w, r, provider.AuthURL(state), http.StatusFound)
}

// OrgCallback handles the OIDC redirect for workspace SSO login. It validates
// the state, exchanges the code, resolves the user, auto-provisions organization
// membership, and issues tokens.
func (h *Handler) OrgCallback(w http.ResponseWriter, r *http.Request) {
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		if desc == "" {
			desc = "login was denied"
		}
		h.redirectWithError(w, r, desc)
		return
	}

	stateCookie, err := r.Cookie("sso_state")
	if err != nil {
		h.redirectWithError(w, r, "missing state cookie")
		return
	}

	orgCookie, err := r.Cookie("sso_org")
	if err != nil {
		h.redirectWithError(w, r, "missing organization cookie")
		return
	}

	// Clear both cookies immediately.
	http.SetCookie(w, &http.Cookie{
		Name: "sso_state", Value: "", Path: "/",
		HttpOnly: true, Secure: h.secureCookies,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name: "sso_org", Value: "", Path: "/",
		HttpOnly: true, Secure: h.secureCookies,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})

	queryState := r.URL.Query().Get("state")
	if queryState == "" || queryState != stateCookie.Value {
		h.redirectWithError(w, r, "invalid state parameter")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		h.redirectWithError(w, r, "missing authorization code")
		return
	}

	orgID := orgCookie.Value

	// Load SSO config for the organization (OIDC columns are nullable since migration 000056).
	var issuerURL, clientID, encryptedSecret *string
	err = h.db.QueryRow(r.Context(),
		"SELECT issuer_url, client_id, client_secret_encrypted FROM organization_sso_configs WHERE organization_id = $1",
		orgID,
	).Scan(&issuerURL, &clientID, &encryptedSecret)
	if err != nil {
		slog.Error("sso: org callback config not found", "orgID", orgID, "error", err)
		h.redirectWithError(w, r, "SSO configuration not found")
		return
	}

	if issuerURL == nil || clientID == nil || encryptedSecret == nil {
		h.redirectWithError(w, r, "SSO configuration is not OIDC")
		return
	}

	clientSecret, err := integration.Decrypt(h.encryptionKey, *encryptedSecret)
	if err != nil {
		slog.Error("sso: failed to decrypt client secret", "error", err)
		h.redirectWithError(w, r, "authentication failed")
		return
	}

	provider, err := NewOIDCProvider(r.Context(), OIDCConfig{
		IssuerURL:    *issuerURL,
		ClientID:     *clientID,
		ClientSecret: clientSecret,
		RedirectURL:  h.baseURL + "/api/auth/sso/org/callback",
	})
	if err != nil {
		slog.Error("sso: failed to create OIDC provider for callback", "error", err)
		h.redirectWithError(w, r, "authentication failed")
		return
	}

	info, err := provider.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("sso: org exchange failed", "orgID", orgID, "error", err)
		h.redirectWithError(w, r, "authentication failed")
		return
	}

	// Use orgID as the provider name for external_identities.
	userID, err := h.resolveUser(r.Context(), orgID, info)
	if err != nil {
		slog.Error("sso: org resolve user failed", "orgID", orgID, "error", err)
		h.redirectWithError(w, r, err.Error())
		return
	}

	// Auto-add user as organization member.
	if _, err := h.db.Exec(r.Context(),
		"INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'member') ON CONFLICT DO NOTHING",
		orgID, userID,
	); err != nil {
		slog.Error("sso: failed to add org member", "orgID", orgID, "userID", userID, "error", err)
	}

	accessToken, refreshToken, err := auth.IssueTokens(r.Context(), h.db, h.jwtSecret, userID)
	if err != nil {
		slog.Error("sso: org issue tokens failed", "error", err)
		h.redirectWithError(w, r, "failed to create session")
		return
	}

	auth.SetRefreshTokenCookie(w, refreshToken, h.secureCookies)
	redirectURL := h.baseURL + "/login?sso_token=" + url.QueryEscape(accessToken)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
