package sso

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/httputil"
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

func (h *Handler) redirectWithError(w http.ResponseWriter, r *http.Request, message string) {
	redirectURL := h.baseURL + "/login?sso_error=" + url.QueryEscape(message)
	http.Redirect(w, r, redirectURL, http.StatusFound)
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

func (h *Handler) ensureSCIMAdminAccess(w http.ResponseWriter, r *http.Request, orgID, userID string) bool {
	var role, subscriptionPlan string
	err := h.db.QueryRow(r.Context(),
		`SELECT om.role, o.subscription_plan
		 FROM organization_members om
		 JOIN organizations o ON o.id = om.organization_id
		 WHERE om.organization_id = $1 AND om.user_id = $2`,
		orgID, userID,
	).Scan(&role, &subscriptionPlan)
	if err != nil {
		if err == pgx.ErrNoRows {
			httputil.WriteError(w, http.StatusForbidden, "admin or owner role required")
			return false
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to load organization access")
		return false
	}
	if role != "owner" && role != "admin" {
		httputil.WriteError(w, http.StatusForbidden, "admin or owner role required")
		return false
	}
	if subscriptionPlan != "business" {
		httputil.WriteError(w, http.StatusForbidden, "business plan required")
		return false
	}
	return true
}

// GenerateSCIMToken creates a new SCIM bearer token for the organization,
// replacing any existing token.
func (h *Handler) GenerateSCIMToken(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")
	if !h.ensureSCIMAdminAccess(w, r, orgID, userID) {
		return
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "generate token failed")
		return
	}
	token := "scim_" + hex.EncodeToString(b)

	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	if _, err := h.db.Exec(r.Context(),
		`INSERT INTO organization_scim_tokens (organization_id, token_hash)
		 VALUES ($1, $2)
		 ON CONFLICT (organization_id) DO UPDATE SET token_hash = EXCLUDED.token_hash, created_at = now()`,
		orgID, tokenHash,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "save token failed")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"token": token})
}

// RevokeSCIMToken deletes the SCIM bearer token for the organization.
func (h *Handler) RevokeSCIMToken(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")
	if !h.ensureSCIMAdminAccess(w, r, orgID, userID) {
		return
	}

	if _, err := h.db.Exec(r.Context(),
		"DELETE FROM organization_scim_tokens WHERE organization_id = $1",
		orgID,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "revoke token failed")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "SCIM token revoked"})
}

// GetSCIMToken returns whether a SCIM token is configured and when it was created.
func (h *Handler) GetSCIMToken(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")
	if !h.ensureSCIMAdminAccess(w, r, orgID, userID) {
		return
	}

	var createdAt time.Time
	err := h.db.QueryRow(r.Context(),
		"SELECT created_at FROM organization_scim_tokens WHERE organization_id = $1",
		orgID,
	).Scan(&createdAt)
	if err != nil {
		if err != pgx.ErrNoRows {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to load token status")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{"configured": false})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"configured": true,
		"createdAt":  createdAt,
	})
}
