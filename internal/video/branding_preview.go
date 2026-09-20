package video

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
	"github.com/sendrec/sendrec/internal/organization"
)

// A preview renders the real watch page with branding the operator has typed but
// not saved, so the settings form stops being a guess. The values arrive on an
// authenticated POST and are handed back an id; the iframe then GETs that id.
//
// The indirection is what keeps this safe to serve. A GET taking branding
// straight from the query string would let anyone dress a page on this domain in
// any company name and CSS they liked; an id minted only by an authenticated
// request cannot be crafted by a stranger. Ids are unguessable and expire
// quickly, so a leaked URL is worth little and not for long.
const brandingPreviewTTL = 10 * time.Minute

type brandingPreviewResponse struct {
	PreviewURL string `json:"previewUrl"`
}

// storeBrandingPreview returns the id the iframe will ask for. The row goes to
// Postgres rather than process memory because the render is a second request:
// behind more than one replica, or across a restart, an id held in memory would
// be gone by the time the iframe asked for it. Expired rows are swept on the way
// in, which is enough housekeeping for a table this short-lived.
func (h *Handler) storeBrandingPreview(ctx context.Context, req setBrandingRequest, now time.Time) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	id := hex.EncodeToString(raw)

	branding, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	if _, err := h.db.Exec(ctx,
		`WITH swept AS (DELETE FROM branding_previews WHERE expires_at < now())
		 INSERT INTO branding_previews (id, branding, expires_at) VALUES ($1, $2, $3)`,
		id, string(branding), now.Add(brandingPreviewTTL),
	); err != nil {
		return "", err
	}
	return id, nil
}

// loadBrandingPreview returns the values behind an id while it is still live. An
// id renders as often as the operator reloads the frame in that window: a reload
// or a Back is not an attack, and refusing the second render only turned a
// working preview into an error page.
func (h *Handler) loadBrandingPreview(ctx context.Context, id string) (setBrandingRequest, bool) {
	var branding []byte
	err := h.db.QueryRow(ctx,
		`SELECT branding FROM branding_previews WHERE id = $1 AND expires_at > now()`,
		id,
	).Scan(&branding)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("branding-preview: failed to load preview", "error", err)
		}
		return setBrandingRequest{}, false
	}

	var req setBrandingRequest
	if err := json.Unmarshal(branding, &req); err != nil {
		slog.Error("branding-preview: stored preview is not readable", "error", err)
		return setBrandingRequest{}, false
	}
	return req, true
}

// CreateBrandingPreview accepts the values the settings form currently holds and
// returns the URL that renders them.
func (h *Handler) CreateBrandingPreview(w http.ResponseWriter, r *http.Request) {
	if !h.requireBrandingEnabled(w) {
		return
	}

	var req setBrandingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := validateBrandingRequest(r.Context(), req); msg != "" {
		httputil.WriteError(w, http.StatusBadRequest, msg)
		return
	}

	// Previewing workspace branding is editing it: the same values, the same
	// resolution, the same presigned logo. Whoever the save refuses must be
	// refused a rendered page too.
	if auth.OrgIDFromContext(r.Context()) != "" {
		if organization.RequireRole(w, r, "owner", "admin") == "" {
			return
		}
	}

	id, err := h.storeBrandingPreview(r.Context(), req, time.Now())
	if err != nil {
		slog.Error("branding-preview: failed to store preview", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create preview")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, brandingPreviewResponse{
		PreviewURL: "/branding/preview/" + id,
	})
}

// BrandingPreviewPage renders the watch page for one previously created preview.
func (h *Handler) BrandingPreviewPage(w http.ResponseWriter, r *http.Request) {
	req, ok := h.loadBrandingPreview(r.Context(), chi.URLParam(r, "id"))
	if !ok {
		h.renderBrandingPreviewExpired(w, r)
		return
	}

	// The same resolution the watch page performs, so the preview cannot drift
	// from the page it is previewing: submitted values over the defaults, with
	// no per-video override in play. The logo key was checked against the
	// account that submitted it before the row was written, so presigning it
	// here hands back nothing that account could not already read.
	cfg := resolveBranding(r.Context(), h.storage, brandingSettingsResponse(req), brandingSettingsResponse{})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := watchPageTemplate.Execute(w, watchPageData{
		Title:              "Your video title",
		Creator:            "You",
		CreatorInitials:    initials("You"),
		Date:               time.Now().Format("02/01/2006"),
		Nonce:              httputil.NonceFromContext(r.Context()),
		BaseURL:            h.baseURL,
		ContentType:        "video",
		Branding:           cfg,
		CustomCSS:          template.CSS(cfg.CustomCSS),
		Description:        "This is how a shared video looks to the people you send it to.",
		Duration:           96,
		ReactionEmojis:     quickReactionEmojis,
		ReactionEmojisJSON: template.JS("[]"),
		ChaptersJSON:       template.JS("[]"),
		JSONLD:             template.JS("{}"),
		Chapters:           []Chapter{},
		Segments:           []TranscriptSegment{},
		SubscriptionPlan:   "business",
		VideoStatus:        "ready",
		// There is no video, share token or comment thread behind this page.
		// Preview tells the template to stand in for them rather than ask the
		// watch API for a token it does not have.
		Preview: true,
	}); err != nil {
		slog.Error("branding-preview: failed to render preview", "error", err)
	}
}

// renderBrandingPreviewExpired answers an id that has lapsed. This lands inside
// the settings iframe, where chi's bare "404 page not found" reads as a broken
// product rather than a preview that timed out.
func (h *Handler) renderBrandingPreviewExpired(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if err := brandingPreviewExpiredTemplate.Execute(w, expiredPageData{
		Nonce: httputil.NonceFromContext(r.Context()),
	}); err != nil {
		slog.Error("branding-preview: failed to render expired notice", "error", err)
	}
}

var brandingPreviewExpiredTemplate = template.Must(template.New("branding-preview-expired").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Preview expired</title>
    <style nonce="{{.Nonce}}">
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            background: #0a1628;
            color: #ffffff;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 2rem;
        }
        .notice { text-align: center; max-width: 24rem; }
        .notice-title { font-size: 1.05rem; font-weight: 600; margin-bottom: 0.5rem; }
        .notice-body { font-size: 0.9rem; line-height: 1.5; opacity: 0.7; }
    </style>
</head>
<body>
    <div class="notice">
        <p class="notice-title">This preview has expired</p>
        <p class="notice-body">Previews are short-lived. Select Refresh preview to build one from the branding in the form.</p>
    </div>
</body>
</html>`))
