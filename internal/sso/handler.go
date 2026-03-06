package sso

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/integration"
)

// Handler implements the HTTP endpoints for social login and SSO callbacks.
type Handler struct {
	db            database.DBTX
	jwtSecret     string
	baseURL       string
	secureCookies bool
	providers     map[string]Provider
	encryptionKey []byte
}

// NewHandler creates a new SSO handler with the given dependencies.
func NewHandler(db database.DBTX, jwtSecret, baseURL string, secureCookies bool, encryptionKey []byte) *Handler {
	return &Handler{
		db:            db,
		jwtSecret:     jwtSecret,
		baseURL:       baseURL,
		secureCookies: secureCookies,
		providers:     make(map[string]Provider),
		encryptionKey: encryptionKey,
	}
}

// RegisterProvider adds an SSO provider under the given name.
func (h *Handler) RegisterProvider(name string, provider Provider) {
	h.providers[name] = provider
}

type providersResponse struct {
	Providers []string `json:"providers"`
}

// Providers returns the list of enabled SSO provider names as JSON.
func (h *Handler) Providers(w http.ResponseWriter, r *http.Request) {
	names := make([]string, 0, len(h.providers))
	for name := range h.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	httputil.WriteJSON(w, http.StatusOK, providersResponse{Providers: names})
}

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

	accessToken, refreshToken, err := h.issueTokens(r.Context(), userID)
	if err != nil {
		slog.Error("sso: issue tokens failed", "error", err)
		h.redirectWithError(w, r, "failed to create session")
		return
	}

	h.setRefreshTokenCookie(w, refreshToken)
	redirectURL := h.baseURL + "/login?sso_token=" + url.QueryEscape(accessToken)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// resolveUser maps an external identity to a local user account, creating
// new users and identity links as needed.
func (h *Handler) resolveUser(ctx context.Context, providerName string, info *UserInfo) (string, error) {
	// 1. Check for an existing external identity link.
	var userID string
	err := h.db.QueryRow(ctx,
		"SELECT user_id FROM external_identities WHERE provider = $1 AND external_id = $2",
		providerName, info.ExternalID,
	).Scan(&userID)
	if err == nil {
		return userID, nil
	}

	// 2. Look up a user by email.
	var emailVerified bool
	err = h.db.QueryRow(ctx,
		"SELECT id, email_verified FROM users WHERE email = $1",
		info.Email,
	).Scan(&userID, &emailVerified)

	if err == nil {
		// User exists with this email.
		if !emailVerified {
			return "", fmt.Errorf("email not verified")
		}
		// Link the external identity to the existing verified user.
		if _, err := h.db.Exec(ctx,
			"INSERT INTO external_identities (user_id, provider, external_id, email) VALUES ($1, $2, $3, $4) ON CONFLICT (provider, external_id) DO NOTHING",
			userID, providerName, info.ExternalID, info.Email,
		); err != nil {
			return "", fmt.Errorf("link identity: %w", err)
		}
		return userID, nil
	}

	// 3. No existing user -- create one.
	err = h.db.QueryRow(ctx,
		"INSERT INTO users (email, password, name, email_verified) VALUES ($1, $2, $3, true) ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email RETURNING id",
		info.Email, "", info.Name,
	).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}

	if _, err := h.db.Exec(ctx,
		"INSERT INTO external_identities (user_id, provider, external_id, email) VALUES ($1, $2, $3, $4) ON CONFLICT (provider, external_id) DO NOTHING",
		userID, providerName, info.ExternalID, info.Email,
	); err != nil {
		return "", fmt.Errorf("create identity: %w", err)
	}

	return userID, nil
}

// issueTokens generates an access/refresh token pair and persists the
// refresh token in the database. It reuses the auth package's exported
// token generation functions for compatibility.
func (h *Handler) issueTokens(ctx context.Context, userID string) (accessToken, refreshToken string, err error) {
	tokenID, err := newTokenID()
	if err != nil {
		return "", "", err
	}

	expiresAt := time.Now().Add(auth.RefreshTokenDuration)
	if _, err := h.db.Exec(ctx,
		"INSERT INTO refresh_tokens (token_id, user_id, expires_at, revoked) VALUES ($1, $2, $3, false)",
		tokenID, userID, expiresAt,
	); err != nil {
		return "", "", fmt.Errorf("store refresh token: %w", err)
	}

	accessToken, err = auth.GenerateAccessToken(h.jwtSecret, userID)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err = auth.GenerateRefreshToken(h.jwtSecret, userID, tokenID)
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

func (h *Handler) setRefreshTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(auth.RefreshTokenDuration / time.Second),
	})
}

func (h *Handler) redirectWithError(w http.ResponseWriter, r *http.Request, message string) {
	redirectURL := h.baseURL + "/login?sso_error=" + url.QueryEscape(message)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// newTokenID generates a random 16-byte hex-encoded token identifier.
func newTokenID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

type ssoConfigRequest struct {
	Provider        string `json:"provider"`
	IssuerURL       string `json:"issuerUrl"`
	ClientID        string `json:"clientId"`
	ClientSecret    string `json:"clientSecret"`
	EnforceSSO      bool   `json:"enforceSso"`
	SAMLMetadataURL string `json:"samlMetadataUrl"`
	SAMLMetadataXML string `json:"samlMetadataXml"`
}

type ssoConfigResponse struct {
	Provider        string `json:"provider"`
	IssuerURL       string `json:"issuerUrl,omitempty"`
	ClientID        string `json:"clientId,omitempty"`
	ClientSecret    string `json:"clientSecret,omitempty"`
	EnforceSSO      bool   `json:"enforceSso"`
	Configured      bool   `json:"configured"`
	SAMLMetadataURL string `json:"samlMetadataUrl,omitempty"`
	SAMLEntityID    string `json:"samlEntityId,omitempty"`
	SAMLSSOURL      string `json:"samlSsoUrl,omitempty"`
	SPMetadataURL   string `json:"spMetadataUrl,omitempty"`
}

// SaveConfig upserts the workspace SSO configuration for the caller's organization.
// It dispatches to saveOIDCConfig or saveSAMLConfig based on the provider field.
func (h *Handler) SaveConfig(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")

	var role string
	err := h.db.QueryRow(r.Context(),
		"SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2",
		orgID, userID,
	).Scan(&role)
	if err != nil || (role != "owner" && role != "admin") {
		httputil.WriteError(w, http.StatusForbidden, "admin or owner role required")
		return
	}

	var req ssoConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Provider == "saml" {
		h.saveSAMLConfig(w, r, orgID, &req)
		return
	}

	h.saveOIDCConfig(w, r, orgID, &req)
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

func (h *Handler) saveSAMLConfig(w http.ResponseWriter, r *http.Request, orgID string, req *ssoConfigRequest) {
	if req.SAMLMetadataURL == "" && req.SAMLMetadataXML == "" {
		httputil.WriteError(w, http.StatusBadRequest, "saml_metadata_url or saml_metadata_xml is required")
		return
	}

	var cfg *SAMLConfig
	var metadataXML string
	if req.SAMLMetadataXML != "" {
		var err error
		cfg, err = ParseSAMLMetadataFromXML([]byte(req.SAMLMetadataXML))
		if err != nil {
			slog.Error("sso: failed to parse SAML metadata XML", "error", err)
			httputil.WriteError(w, http.StatusBadRequest, "invalid SAML metadata XML")
			return
		}
		metadataXML = req.SAMLMetadataXML
	} else {
		var err error
		cfg, err = ParseSAMLMetadataFromURL(req.SAMLMetadataURL)
		if err != nil {
			slog.Error("sso: failed to fetch SAML metadata", "error", err)
			httputil.WriteError(w, http.StatusBadRequest, "failed to fetch SAML metadata from URL")
			return
		}
	}

	if _, err := h.db.Exec(r.Context(),
		`INSERT INTO organization_sso_configs (organization_id, provider, enforce_sso,
		     saml_metadata_url, saml_entity_id, saml_sso_url, saml_certificate, saml_metadata_xml,
		     issuer_url, client_id, client_secret_encrypted)
		 VALUES ($1, 'saml', $2, $3, $4, $5, $6, $7, NULL, NULL, NULL)
		 ON CONFLICT (organization_id) DO UPDATE
		 SET provider = 'saml',
		     enforce_sso = EXCLUDED.enforce_sso,
		     saml_metadata_url = EXCLUDED.saml_metadata_url,
		     saml_entity_id = EXCLUDED.saml_entity_id,
		     saml_sso_url = EXCLUDED.saml_sso_url,
		     saml_certificate = EXCLUDED.saml_certificate,
		     saml_metadata_xml = EXCLUDED.saml_metadata_xml,
		     issuer_url = NULL,
		     client_id = NULL,
		     client_secret_encrypted = NULL,
		     updated_at = now()`,
		orgID, req.EnforceSSO, req.SAMLMetadataURL, cfg.EntityID, cfg.SSOURL, cfg.Certificate, metadataXML,
	); err != nil {
		slog.Error("sso: failed to upsert SAML config", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to save SSO config")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetConfig returns the workspace SSO configuration for the caller's organization.
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	var provider string
	var issuerURL, clientID, samlMetadataURL, samlEntityID, samlSSOURL *string
	var enforceSSO bool
	err := h.db.QueryRow(r.Context(),
		`SELECT provider, issuer_url, client_id, enforce_sso,
		        saml_metadata_url, saml_entity_id, saml_sso_url
		 FROM organization_sso_configs WHERE organization_id = $1`,
		orgID,
	).Scan(&provider, &issuerURL, &clientID, &enforceSSO,
		&samlMetadataURL, &samlEntityID, &samlSSOURL)
	if err != nil {
		if err == pgx.ErrNoRows {
			httputil.WriteJSON(w, http.StatusOK, ssoConfigResponse{Configured: false})
			return
		}
		slog.Error("sso: failed to load SSO config", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to load SSO config")
		return
	}

	resp := ssoConfigResponse{
		Provider:   provider,
		EnforceSSO: enforceSSO,
		Configured: true,
	}

	if provider == "saml" {
		if samlMetadataURL != nil {
			resp.SAMLMetadataURL = *samlMetadataURL
		}
		if samlEntityID != nil {
			resp.SAMLEntityID = *samlEntityID
		}
		if samlSSOURL != nil {
			resp.SAMLSSOURL = *samlSSOURL
		}
		resp.SPMetadataURL = h.baseURL + "/api/auth/saml/" + orgID + "/metadata"
	} else {
		if issuerURL != nil {
			resp.IssuerURL = *issuerURL
		}
		if clientID != nil {
			resp.ClientID = *clientID
		}
		resp.ClientSecret = "******"
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// DeleteConfig removes the workspace SSO configuration for the caller's organization.
func (h *Handler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")

	var role string
	err := h.db.QueryRow(r.Context(),
		"SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2",
		orgID, userID,
	).Scan(&role)
	if err != nil || role != "owner" {
		httputil.WriteError(w, http.StatusForbidden, "owner role required")
		return
	}

	if _, err := h.db.Exec(r.Context(),
		"DELETE FROM organization_sso_configs WHERE organization_id = $1",
		orgID,
	); err != nil {
		slog.Error("sso: failed to delete SSO config", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to delete SSO config")
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

	accessToken, refreshToken, err := h.issueTokens(r.Context(), userID)
	if err != nil {
		slog.Error("sso: org issue tokens failed", "error", err)
		h.redirectWithError(w, r, "failed to create session")
		return
	}

	h.setRefreshTokenCookie(w, refreshToken)
	redirectURL := h.baseURL + "/login?sso_token=" + url.QueryEscape(accessToken)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// OrgSAMLCallback handles the SAML ACS (Assertion Consumer Service) POST for
// workspace SSO login. It validates the RelayState against the sso_state cookie,
// loads the SAML config, validates the SAMLResponse, resolves the user,
// auto-provisions organization membership, and issues tokens.
func (h *Handler) OrgSAMLCallback(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	if err := r.ParseForm(); err != nil {
		h.redirectWithError(w, r, "invalid form data")
		return
	}

	samlResponse := r.FormValue("SAMLResponse")
	relayState := r.FormValue("RelayState")

	stateCookie, err := r.Cookie("sso_state")
	if err != nil {
		h.redirectWithError(w, r, "missing state cookie")
		return
	}

	// Clear the state cookie immediately (must match SameSite=None from initiation).
	http.SetCookie(w, &http.Cookie{
		Name:     "sso_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   -1,
	})

	// Cookie value is "state|requestID" — split to extract both.
	cookieParts := strings.SplitN(stateCookie.Value, "|", 2)
	cookieState := cookieParts[0]
	var samlRequestID string
	if len(cookieParts) == 2 {
		samlRequestID = cookieParts[1]
	}

	if relayState == "" || relayState != cookieState {
		h.redirectWithError(w, r, "invalid state parameter")
		return
	}

	// Load SAML config for the organization.
	var configProvider string
	var samlEntityID, samlSSOURL, samlCertificate *string
	err = h.db.QueryRow(r.Context(),
		`SELECT provider, saml_entity_id, saml_sso_url, saml_certificate
		 FROM organization_sso_configs WHERE organization_id = $1`,
		orgID,
	).Scan(&configProvider, &samlEntityID, &samlSSOURL, &samlCertificate)
	if err != nil {
		slog.Error("sso: SAML callback config not found", "orgID", orgID, "error", err)
		h.redirectWithError(w, r, "SSO configuration not found")
		return
	}

	if configProvider != "saml" {
		h.redirectWithError(w, r, "SSO configuration is not SAML")
		return
	}

	if samlEntityID == nil || samlSSOURL == nil || samlCertificate == nil {
		slog.Error("sso: incomplete SAML config for callback", "orgID", orgID)
		h.redirectWithError(w, r, "incomplete SAML configuration")
		return
	}

	samlProvider, err := NewSAMLProvider(h.baseURL, orgID, &SAMLConfig{
		EntityID:    *samlEntityID,
		SSOURL:      *samlSSOURL,
		Certificate: *samlCertificate,
	})
	if err != nil {
		slog.Error("sso: failed to create SAML provider for callback", "error", err)
		h.redirectWithError(w, r, "authentication failed")
		return
	}

	info, err := samlProvider.ExchangeWithRequestID(r.Context(), samlResponse, samlRequestID)
	if err != nil {
		slog.Error("sso: SAML exchange failed", "orgID", orgID, "error", err)
		h.redirectWithError(w, r, "authentication failed")
		return
	}

	userID, err := h.resolveUser(r.Context(), orgID, info)
	if err != nil {
		slog.Error("sso: SAML resolve user failed", "orgID", orgID, "error", err)
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

	accessToken, refreshToken, err := h.issueTokens(r.Context(), userID)
	if err != nil {
		slog.Error("sso: SAML issue tokens failed", "error", err)
		h.redirectWithError(w, r, "failed to create session")
		return
	}

	h.setRefreshTokenCookie(w, refreshToken)
	redirectURL := h.baseURL + "/login?sso_token=" + url.QueryEscape(accessToken)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// SPMetadata serves the SAML Service Provider metadata XML for the given
// organization. Identity providers use this to configure their side of the
// SAML trust relationship.
func (h *Handler) SPMetadata(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	var samlEntityID, samlSSOURL, samlCertificate *string
	err := h.db.QueryRow(r.Context(),
		`SELECT saml_entity_id, saml_sso_url, saml_certificate
		 FROM organization_sso_configs
		 WHERE organization_id = $1 AND provider = 'saml'`,
		orgID,
	).Scan(&samlEntityID, &samlSSOURL, &samlCertificate)
	if err != nil {
		if err == pgx.ErrNoRows {
			httputil.WriteError(w, http.StatusNotFound, "SAML configuration not found")
			return
		}
		slog.Error("sso: failed to load SAML config for SP metadata", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to load SAML configuration")
		return
	}

	if samlEntityID == nil || samlSSOURL == nil || samlCertificate == nil {
		httputil.WriteError(w, http.StatusNotFound, "incomplete SAML configuration")
		return
	}

	samlProvider, err := NewSAMLProvider(h.baseURL, orgID, &SAMLConfig{
		EntityID:    *samlEntityID,
		SSOURL:      *samlSSOURL,
		Certificate: *samlCertificate,
	})
	if err != nil {
		slog.Error("sso: failed to create SAML provider for SP metadata", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate SP metadata")
		return
	}

	metadata := samlProvider.sp.Metadata()
	xmlBytes, err := xml.MarshalIndent(metadata, "", "  ")
	if err != nil {
		slog.Error("sso: failed to marshal SP metadata", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate SP metadata")
		return
	}

	w.Header().Set("Content-Type", "application/samlmetadata+xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(xmlBytes)
}

type identityResponse struct {
	Provider   string    `json:"provider"`
	ExternalID string    `json:"externalId"`
	Email      string    `json:"email"`
	CreatedAt  time.Time `json:"createdAt"`
}

// ListIdentities returns all external identity links for the authenticated user.
func (h *Handler) ListIdentities(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var password string
	if err := h.db.QueryRow(r.Context(),
		"SELECT password FROM users WHERE id = $1", userID,
	).Scan(&password); err != nil {
		slog.Error("sso: failed to check user password", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list identities")
		return
	}

	rows, err := h.db.Query(r.Context(),
		"SELECT provider, external_id, email, created_at FROM external_identities WHERE user_id = $1 ORDER BY created_at",
		userID,
	)
	if err != nil {
		slog.Error("sso: failed to list identities", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list identities")
		return
	}
	defer rows.Close()

	identities := make([]identityResponse, 0)
	for rows.Next() {
		var identity identityResponse
		if err := rows.Scan(&identity.Provider, &identity.ExternalID, &identity.Email, &identity.CreatedAt); err != nil {
			slog.Error("sso: failed to scan identity row", "error", err)
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list identities")
			return
		}
		identities = append(identities, identity)
	}
	if err := rows.Err(); err != nil {
		slog.Error("sso: rows iteration error", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list identities")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"identities":  identities,
		"hasPassword": password != "",
	})
}

// UnlinkIdentity removes an external identity link for the authenticated user.
func (h *Handler) UnlinkIdentity(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	providerName := chi.URLParam(r, "provider")

	// Check if user has a password set.
	var password string
	err := h.db.QueryRow(r.Context(),
		"SELECT password FROM users WHERE id = $1",
		userID,
	).Scan(&password)
	if err != nil {
		slog.Error("sso: failed to check user password", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to unlink identity")
		return
	}

	// Count remaining identities.
	var identityCount int
	err = h.db.QueryRow(r.Context(),
		"SELECT count(*) FROM external_identities WHERE user_id = $1",
		userID,
	).Scan(&identityCount)
	if err != nil {
		slog.Error("sso: failed to count identities", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to unlink identity")
		return
	}

	if password == "" && identityCount <= 1 {
		httputil.WriteError(w, http.StatusBadRequest, "cannot unlink last identity without a password set")
		return
	}

	if _, err := h.db.Exec(r.Context(),
		"DELETE FROM external_identities WHERE user_id = $1 AND provider = $2",
		userID, providerName,
	); err != nil {
		slog.Error("sso: failed to delete identity", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to unlink identity")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
