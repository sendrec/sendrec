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
	"regexp"
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

// An id is 32 random bytes in hex and nothing else, so anything of another
// shape could not have been minted here. The route is unauthenticated, and
// checking the shape first keeps a stranger from turning it into a query.
var brandingPreviewIDPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// errBrandingPreviewGone separates an id that has lapsed from a database that
// cannot answer. Both end the render, but only one is the operator's to fix.
var errBrandingPreviewGone = errors.New("branding preview not found or expired")

type brandingPreviewResponse struct {
	PreviewURL string `json:"previewUrl"`
}

// brandingPreviewPayload is what a preview row holds: the values the operator
// typed, and the plan of the account that typed them. The render is
// unauthenticated and cannot look the plan up for itself, but the page it
// stands for shows attribution below a free plan and hides it above one.
type brandingPreviewPayload struct {
	Branding         setBrandingRequest `json:"branding"`
	SubscriptionPlan string             `json:"subscriptionPlan"`
}

// storeBrandingPreview returns the id the iframe will ask for. The row goes to
// Postgres rather than process memory because the render is a second request:
// behind more than one replica, or across a restart, an id held in memory would
// be gone by the time the iframe asked for it. Expired rows are swept on the way
// in, which is enough housekeeping for a table this short-lived.
func (h *Handler) storeBrandingPreview(ctx context.Context, payload brandingPreviewPayload) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	id := hex.EncodeToString(raw)

	branding, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	// Written and judged by the same clock. Taking the expiry from the app host
	// instead would let a few minutes of drift between it and the database mint
	// previews that are already expired, or ones that outlive the window.
	if _, err := h.db.Exec(ctx,
		`WITH swept AS (DELETE FROM branding_previews WHERE expires_at < now())
		 INSERT INTO branding_previews (id, branding, expires_at)
		 VALUES ($1, $2, now() + make_interval(secs => $3))`,
		id, string(branding), brandingPreviewTTL.Seconds(),
	); err != nil {
		return "", err
	}
	return id, nil
}

// loadBrandingPreview returns the values behind an id while it is still live. An
// id renders as often as the operator reloads the frame in that window: a reload
// or a Back is not an attack, and refusing the second render only turned a
// working preview into an error page.
func (h *Handler) loadBrandingPreview(ctx context.Context, id string) (brandingPreviewPayload, error) {
	if !brandingPreviewIDPattern.MatchString(id) {
		return brandingPreviewPayload{}, errBrandingPreviewGone
	}

	var branding []byte
	err := h.db.QueryRow(ctx,
		`SELECT branding FROM branding_previews WHERE id = $1 AND expires_at > now()`,
		id,
	).Scan(&branding)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return brandingPreviewPayload{}, errBrandingPreviewGone
		}
		return brandingPreviewPayload{}, err
	}

	var payload brandingPreviewPayload
	if err := json.Unmarshal(branding, &payload); err != nil {
		return brandingPreviewPayload{}, err
	}
	return payload, nil
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

	// The render is unauthenticated, so whatever it needs to know about this
	// account has to be settled here, while there still is one.
	var plan string
	if orgID := auth.OrgIDFromContext(r.Context()); orgID != "" {
		plan, _ = h.getOrgPlan(r.Context(), orgID)
	} else {
		plan, _ = h.getUserPlan(r.Context(), auth.UserIDFromContext(r.Context()))
	}

	id, err := h.storeBrandingPreview(r.Context(), brandingPreviewPayload{
		Branding:         req,
		SubscriptionPlan: plan,
	})
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
	payload, err := h.loadBrandingPreview(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, errBrandingPreviewGone) {
		h.renderBrandingPreviewExpired(w, r)
		return
	}
	if err != nil {
		slog.Error("branding-preview: failed to load preview", "error", err)
		http.Error(w, "failed to load preview", http.StatusInternalServerError)
		return
	}

	// The same resolution the watch page performs, so the preview cannot drift
	// from the page it is previewing: submitted values over the defaults, with
	// no per-video override in play. The logo key was checked against the
	// account that submitted it before the row was written, so presigning it
	// here hands back nothing that account could not already read.
	cfg := resolveBranding(r.Context(), h.storage, brandingSettingsResponse(payload.Branding), brandingSettingsResponse{})

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
		// Below a paid plan the page carries attribution the operator cannot
		// style away, and a preview that assumed otherwise would be standing
		// for somebody else's page.
		SubscriptionPlan: payload.SubscriptionPlan,
		VideoStatus:      "ready",
		// Comment mode belongs to a video, and this stands for no video in
		// particular. The fullest thread puts every part an operator can style
		// on screen; a narrower one would hide fields their videos may ask for.
		CommentMode: "name_email_required",
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
