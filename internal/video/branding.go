package video

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
)

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

const (
	defaultColorBackground = "#0a1628"
	defaultColorSurface    = "#1e293b"
	defaultColorText       = "#ffffff"
	defaultColorAccent     = "#00b67a"
	defaultCompanyName     = "SendRec"
	defaultLogoPath        = "/images/logo.png"
	defaultFooterText      = ""

	maxCompanyNameLength = 200
	maxFooterTextLength  = 500
	maxLogoUploadBytes   = 512 * 1024
)

type brandingConfig struct {
	CompanyName     string `json:"companyName"`
	LogoURL         string `json:"logoUrl"`
	ColorBackground string `json:"colorBackground"`
	ColorSurface    string `json:"colorSurface"`
	ColorText       string `json:"colorText"`
	ColorAccent     string `json:"colorAccent"`
	FooterText      string `json:"footerText"`
	HasCustomLogo   bool   `json:"hasCustomLogo"`
}

type brandingSettingsResponse struct {
	CompanyName     *string `json:"companyName"`
	LogoKey         *string `json:"logoKey"`
	ColorBackground *string `json:"colorBackground"`
	ColorSurface    *string `json:"colorSurface"`
	ColorText       *string `json:"colorText"`
	ColorAccent     *string `json:"colorAccent"`
	FooterText      *string `json:"footerText"`
}

type setBrandingRequest struct {
	CompanyName     *string `json:"companyName"`
	LogoKey         *string `json:"logoKey"`
	ColorBackground *string `json:"colorBackground"`
	ColorSurface    *string `json:"colorSurface"`
	ColorText       *string `json:"colorText"`
	ColorAccent     *string `json:"colorAccent"`
	FooterText      *string `json:"footerText"`
}

type setVideoBrandingRequest struct {
	CompanyName     *string `json:"companyName"`
	ColorBackground *string `json:"colorBackground"`
	ColorSurface    *string `json:"colorSurface"`
	ColorText       *string `json:"colorText"`
	ColorAccent     *string `json:"colorAccent"`
	FooterText      *string `json:"footerText"`
}

type logoUploadResponse struct {
	UploadURL string `json:"uploadUrl"`
	LogoKey   string `json:"logoKey"`
}

func isValidHexColor(s string) bool {
	return hexColorPattern.MatchString(s)
}

func validateBrandingColors(bg, surface, text, accent *string) string {
	for _, pair := range []struct {
		val  *string
		name string
	}{
		{bg, "colorBackground"},
		{surface, "colorSurface"},
		{text, "colorText"},
		{accent, "colorAccent"},
	} {
		if pair.val != nil && *pair.val != "" && !isValidHexColor(*pair.val) {
			return "invalid " + pair.name + ": must be a hex color like #1a2b3c"
		}
	}
	return ""
}

func resolveBranding(ctx context.Context, storage ObjectStorage, userBranding brandingSettingsResponse, videoBranding brandingSettingsResponse) brandingConfig {
	cfg := brandingConfig{
		CompanyName:     defaultCompanyName,
		LogoURL:         defaultLogoPath,
		ColorBackground: defaultColorBackground,
		ColorSurface:    defaultColorSurface,
		ColorText:       defaultColorText,
		ColorAccent:     defaultColorAccent,
		FooterText:      defaultFooterText,
	}

	applyOverrides(&cfg, userBranding)
	applyOverrides(&cfg, videoBranding)

	logoKey := resolveLogoKey(userBranding.LogoKey, videoBranding.LogoKey)
	if logoKey == "none" {
		cfg.LogoURL = ""
		cfg.HasCustomLogo = true
	} else if logoKey != "" {
		url, err := storage.GenerateDownloadURL(ctx, logoKey, 1*time.Hour)
		if err == nil {
			cfg.LogoURL = url
			cfg.HasCustomLogo = true
		}
	}

	return cfg
}

func resolveLogoKey(userLogoKey, videoLogoKey *string) string {
	if videoLogoKey != nil && *videoLogoKey != "" {
		return *videoLogoKey
	}
	if userLogoKey != nil && *userLogoKey != "" {
		return *userLogoKey
	}
	return ""
}

func applyOverrides(cfg *brandingConfig, src brandingSettingsResponse) {
	if src.CompanyName != nil && *src.CompanyName != "" {
		cfg.CompanyName = *src.CompanyName
	}
	if src.ColorBackground != nil && *src.ColorBackground != "" {
		cfg.ColorBackground = *src.ColorBackground
	}
	if src.ColorSurface != nil && *src.ColorSurface != "" {
		cfg.ColorSurface = *src.ColorSurface
	}
	if src.ColorText != nil && *src.ColorText != "" {
		cfg.ColorText = *src.ColorText
	}
	if src.ColorAccent != nil && *src.ColorAccent != "" {
		cfg.ColorAccent = *src.ColorAccent
	}
	if src.FooterText != nil && *src.FooterText != "" {
		cfg.FooterText = *src.FooterText
	}
}

func (h *Handler) requireBrandingEnabled(w http.ResponseWriter) bool {
	if !h.brandingEnabled {
		httputil.WriteError(w, http.StatusForbidden, "branding requires a paid plan")
		return false
	}
	return true
}

func (h *Handler) GetBrandingSettings(w http.ResponseWriter, r *http.Request) {
	if !h.requireBrandingEnabled(w) {
		return
	}
	userID := auth.UserIDFromContext(r.Context())

	var resp brandingSettingsResponse
	err := h.db.QueryRow(r.Context(),
		`SELECT company_name, logo_key, color_background, color_surface, color_text, color_accent, footer_text
		 FROM user_branding WHERE user_id = $1`,
		userID,
	).Scan(&resp.CompanyName, &resp.LogoKey, &resp.ColorBackground, &resp.ColorSurface, &resp.ColorText, &resp.ColorAccent, &resp.FooterText)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteJSON(w, http.StatusOK, brandingSettingsResponse{})
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get branding settings")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) PutBrandingSettings(w http.ResponseWriter, r *http.Request) {
	if !h.requireBrandingEnabled(w) {
		return
	}
	userID := auth.UserIDFromContext(r.Context())

	var req setBrandingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CompanyName != nil && len(*req.CompanyName) > maxCompanyNameLength {
		httputil.WriteError(w, http.StatusBadRequest, "company name must be 200 characters or fewer")
		return
	}
	if req.FooterText != nil && len(*req.FooterText) > maxFooterTextLength {
		httputil.WriteError(w, http.StatusBadRequest, "footer text must be 500 characters or fewer")
		return
	}
	if errMsg := validateBrandingColors(req.ColorBackground, req.ColorSurface, req.ColorText, req.ColorAccent); errMsg != "" {
		httputil.WriteError(w, http.StatusBadRequest, errMsg)
		return
	}

	if _, err := h.db.Exec(r.Context(),
		`INSERT INTO user_branding (user_id, company_name, logo_key, color_background, color_surface, color_text, color_accent, footer_text)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (user_id) DO UPDATE SET
		   company_name = $2, logo_key = $3, color_background = $4, color_surface = $5,
		   color_text = $6, color_accent = $7, footer_text = $8, updated_at = now()`,
		userID, req.CompanyName, req.LogoKey, req.ColorBackground, req.ColorSurface, req.ColorText, req.ColorAccent, req.FooterText,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to save branding settings")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UploadBrandingLogo(w http.ResponseWriter, r *http.Request) {
	if !h.requireBrandingEnabled(w) {
		return
	}
	userID := auth.UserIDFromContext(r.Context())

	var req struct {
		ContentType   string `json:"contentType"`
		ContentLength int64  `json:"contentLength"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ContentType != "image/png" && req.ContentType != "image/svg+xml" {
		httputil.WriteError(w, http.StatusBadRequest, "logo must be PNG or SVG")
		return
	}
	if req.ContentLength <= 0 || req.ContentLength > maxLogoUploadBytes {
		httputil.WriteError(w, http.StatusBadRequest, "logo must be 512KB or smaller")
		return
	}

	ext := ".png"
	if req.ContentType == "image/svg+xml" {
		ext = ".svg"
	}
	logoKey := "branding/" + userID + "/logo" + ext

	uploadURL, err := h.storage.GenerateUploadURL(r.Context(), logoKey, req.ContentType, req.ContentLength, 15*time.Minute)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate upload URL")
		return
	}

	if _, err := h.db.Exec(r.Context(),
		`INSERT INTO user_branding (user_id, logo_key)
		 VALUES ($1, $2)
		 ON CONFLICT (user_id) DO UPDATE SET logo_key = $2, updated_at = now()`,
		userID, logoKey,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to save logo key")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, logoUploadResponse{
		UploadURL: uploadURL,
		LogoKey:   logoKey,
	})
}

func (h *Handler) DeleteBrandingLogo(w http.ResponseWriter, r *http.Request) {
	if !h.requireBrandingEnabled(w) {
		return
	}
	userID := auth.UserIDFromContext(r.Context())

	var logoKey *string
	err := h.db.QueryRow(r.Context(),
		`SELECT logo_key FROM user_branding WHERE user_id = $1`,
		userID,
	).Scan(&logoKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get branding")
		return
	}

	if logoKey != nil && *logoKey != "" {
		_ = h.storage.DeleteObject(r.Context(), *logoKey)
	}

	if _, err := h.db.Exec(r.Context(),
		`UPDATE user_branding SET logo_key = NULL, updated_at = now() WHERE user_id = $1`,
		userID,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to remove logo")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetVideoBranding(w http.ResponseWriter, r *http.Request) {
	if !h.requireBrandingEnabled(w) {
		return
	}
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var resp brandingSettingsResponse
	err := h.db.QueryRow(r.Context(),
		`SELECT branding_company_name, branding_logo_key, branding_color_background, branding_color_surface,
		        branding_color_text, branding_color_accent, branding_footer_text
		 FROM videos WHERE id = $1 AND user_id = $2 AND status != 'deleted'`,
		videoID, userID,
	).Scan(&resp.CompanyName, &resp.LogoKey, &resp.ColorBackground, &resp.ColorSurface, &resp.ColorText, &resp.ColorAccent, &resp.FooterText)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "video not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get video branding")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) SetVideoBranding(w http.ResponseWriter, r *http.Request) {
	if !h.requireBrandingEnabled(w) {
		return
	}
	userID := auth.UserIDFromContext(r.Context())
	videoID := chi.URLParam(r, "id")

	var req setVideoBrandingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CompanyName != nil && len(*req.CompanyName) > maxCompanyNameLength {
		httputil.WriteError(w, http.StatusBadRequest, "company name must be 200 characters or fewer")
		return
	}
	if req.FooterText != nil && len(*req.FooterText) > maxFooterTextLength {
		httputil.WriteError(w, http.StatusBadRequest, "footer text must be 500 characters or fewer")
		return
	}
	if errMsg := validateBrandingColors(req.ColorBackground, req.ColorSurface, req.ColorText, req.ColorAccent); errMsg != "" {
		httputil.WriteError(w, http.StatusBadRequest, errMsg)
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET
		   branding_company_name = $1, branding_color_background = $2, branding_color_surface = $3,
		   branding_color_text = $4, branding_color_accent = $5, branding_footer_text = $6
		 WHERE id = $7 AND user_id = $8 AND status != 'deleted'`,
		req.CompanyName, req.ColorBackground, req.ColorSurface, req.ColorText, req.ColorAccent, req.FooterText, videoID, userID,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to save video branding")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
