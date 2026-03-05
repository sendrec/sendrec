package sso

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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
			"INSERT INTO external_identities (user_id, provider, external_id, email) VALUES ($1, $2, $3, $4)",
			userID, providerName, info.ExternalID, info.Email,
		); err != nil {
			return "", fmt.Errorf("link identity: %w", err)
		}
		return userID, nil
	}

	// 3. No existing user -- create one.
	err = h.db.QueryRow(ctx,
		"INSERT INTO users (email, password, name, email_verified) VALUES ($1, $2, $3, true) RETURNING id",
		info.Email, "", info.Name,
	).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}

	if _, err := h.db.Exec(ctx,
		"INSERT INTO external_identities (user_id, provider, external_id, email) VALUES ($1, $2, $3, $4)",
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

// --- Workspace SSO Config CRUD (Task 6) ---

type ssoConfigRequest struct {
	IssuerURL    string `json:"issuer_url"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	EnforceSSO   bool   `json:"enforce_sso"`
}

type ssoConfigResponse struct {
	Provider     string `json:"provider"`
	IssuerURL    string `json:"issuer_url"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	EnforceSSO   bool   `json:"enforce_sso"`
	Configured   bool   `json:"configured"`
}

// SaveConfig upserts the workspace SSO configuration for the caller's organization.
func (h *Handler) SaveConfig(w http.ResponseWriter, r *http.Request) {
	orgID := auth.OrgIDFromContext(r.Context())
	if orgID == "" {
		httputil.WriteError(w, http.StatusBadRequest, "organization context required")
		return
	}

	role := auth.OrgRoleFromContext(r.Context())
	if role != "owner" && role != "admin" {
		httputil.WriteError(w, http.StatusForbidden, "admin or owner role required")
		return
	}

	var req ssoConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

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
		`INSERT INTO organization_sso_configs (organization_id, provider, issuer_url, client_id, client_secret_encrypted, enforce_sso)
		 VALUES ($1, 'oidc', $2, $3, $4, $5)
		 ON CONFLICT (organization_id) DO UPDATE
		 SET issuer_url = EXCLUDED.issuer_url,
		     client_id = EXCLUDED.client_id,
		     client_secret_encrypted = EXCLUDED.client_secret_encrypted,
		     enforce_sso = EXCLUDED.enforce_sso,
		     updated_at = now()`,
		orgID, req.IssuerURL, req.ClientID, encryptedSecret, req.EnforceSSO,
	); err != nil {
		slog.Error("sso: failed to upsert SSO config", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to save SSO config")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetConfig returns the workspace SSO configuration for the caller's organization.
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	orgID := auth.OrgIDFromContext(r.Context())
	if orgID == "" {
		httputil.WriteError(w, http.StatusBadRequest, "organization context required")
		return
	}

	var provider, issuerURL, clientID string
	var enforceSSO bool
	err := h.db.QueryRow(r.Context(),
		"SELECT provider, issuer_url, client_id, enforce_sso FROM organization_sso_configs WHERE organization_id = $1",
		orgID,
	).Scan(&provider, &issuerURL, &clientID, &enforceSSO)
	if err != nil {
		if err == pgx.ErrNoRows {
			httputil.WriteJSON(w, http.StatusOK, ssoConfigResponse{Configured: false})
			return
		}
		slog.Error("sso: failed to load SSO config", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to load SSO config")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, ssoConfigResponse{
		Provider:     provider,
		IssuerURL:    issuerURL,
		ClientID:     integration.MaskToken(clientID),
		ClientSecret: "******",
		EnforceSSO:   enforceSSO,
		Configured:   true,
	})
}

// DeleteConfig removes the workspace SSO configuration for the caller's organization.
func (h *Handler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	orgID := auth.OrgIDFromContext(r.Context())
	if orgID == "" {
		httputil.WriteError(w, http.StatusBadRequest, "organization context required")
		return
	}

	role := auth.OrgRoleFromContext(r.Context())
	if role != "owner" {
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
