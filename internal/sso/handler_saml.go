package sso

import (
	"encoding/xml"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
)

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

	accessToken, refreshToken, err := auth.IssueTokens(r.Context(), h.db, h.jwtSecret, userID)
	if err != nil {
		slog.Error("sso: SAML issue tokens failed", "error", err)
		h.redirectWithError(w, r, "failed to create session")
		return
	}

	auth.SetRefreshTokenCookie(w, refreshToken, h.secureCookies)
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
