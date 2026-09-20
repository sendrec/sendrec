package video

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sendrec/sendrec/internal/httputil"
)

// A preview renders the real watch page with branding the operator has typed but
// not saved, so the settings form stops being a guess. The values arrive on an
// authenticated POST and are handed back an id; the iframe then GETs that id.
//
// The indirection is what keeps this safe to serve. A GET taking branding
// straight from the query string would let anyone dress a page on this domain in
// any company name and CSS they liked; an id minted only by an authenticated
// request cannot be crafted by a stranger. Each id renders once and expires
// quickly, so a leaked URL is worth nothing.
const (
	brandingPreviewTTL      = 2 * time.Minute
	brandingPreviewMaxItems = 256
)

type brandingPreviewEntry struct {
	branding brandingConfig
	expires  time.Time
}

type brandingPreviewStore struct {
	mu      sync.Mutex
	entries map[string]brandingPreviewEntry
}

func newBrandingPreviewStore() *brandingPreviewStore {
	return &brandingPreviewStore{entries: make(map[string]brandingPreviewEntry)}
}

// put returns the id the iframe will ask for. Expired entries are dropped on the
// way in, which is enough housekeeping for a store this small and short-lived.
func (s *brandingPreviewStore) put(cfg brandingConfig, now time.Time) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	id := hex.EncodeToString(raw)

	s.mu.Lock()
	defer s.mu.Unlock()

	for key, entry := range s.entries {
		if now.After(entry.expires) {
			delete(s.entries, key)
		}
	}
	// A burst of previews from one impatient operator should not grow without
	// bound; the oldest goes rather than refusing the newest.
	for len(s.entries) >= brandingPreviewMaxItems {
		var oldestKey string
		var oldest time.Time
		for key, entry := range s.entries {
			if oldestKey == "" || entry.expires.Before(oldest) {
				oldestKey, oldest = key, entry.expires
			}
		}
		delete(s.entries, oldestKey)
	}

	s.entries[id] = brandingPreviewEntry{branding: cfg, expires: now.Add(brandingPreviewTTL)}
	return id, nil
}

// take returns the entry and removes it: one id, one render.
func (s *brandingPreviewStore) take(id string, now time.Time) (brandingConfig, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[id]
	if !ok {
		return brandingConfig{}, false
	}
	delete(s.entries, id)
	if now.After(entry.expires) {
		return brandingConfig{}, false
	}
	return entry.branding, true
}

type brandingPreviewResponse struct {
	PreviewURL string `json:"previewUrl"`
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
	if msg := validateBrandingRequest(req); msg != "" {
		httputil.WriteError(w, http.StatusBadRequest, msg)
		return
	}

	// The same resolution the watch page performs, so the preview cannot drift
	// from the page it is previewing: submitted values over the defaults, with
	// no per-video override in play.
	cfg := resolveBranding(r.Context(), h.storage, brandingSettingsResponse(req), brandingSettingsResponse{})

	id, err := h.brandingPreviews.put(cfg, time.Now())
	if err != nil {
		slog.Error("branding-preview: failed to mint id", "error", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create preview")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, brandingPreviewResponse{
		PreviewURL: "/branding/preview/" + id,
	})
}

// BrandingPreviewPage renders the watch page for one previously created preview.
func (h *Handler) BrandingPreviewPage(w http.ResponseWriter, r *http.Request) {
	cfg, ok := h.brandingPreviews.take(chi.URLParam(r, "id"), time.Now())
	if !ok {
		http.NotFound(w, r)
		return
	}

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
	}); err != nil {
		slog.Error("branding-preview: failed to render preview", "error", err)
	}
}
