package video

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/sendrec/sendrec/internal/httputil"
)

var playlistWatchColumns = []string{
	"id", "title", "description", "share_password", "require_email",
	"organization_id",
	"ub_company_name", "ub_logo_key", "ub_color_background", "ub_color_surface",
	"ub_color_text", "ub_color_accent", "ub_footer_text", "ub_custom_css",
	"ob_company_name", "ob_logo_key", "ob_color_background", "ob_color_surface",
	"ob_color_text", "ob_color_accent", "ob_footer_text", "ob_custom_css",
}

var playlistVideosColumns = []string{
	"id", "title", "duration", "share_token", "content_type", "user_id", "thumbnail_key",
}

func playlistWatchRequest(shareToken string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/watch/playlist/"+shareToken, nil)
	ctx := httputil.ContextWithNonce(req.Context(), "test-nonce")
	return req.WithContext(ctx)
}

func servePlaylistWatchPage(handler *Handler, req *http.Request) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Get("/watch/playlist/{shareToken}", handler.PlaylistWatchPage)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestPlaylistWatchPage_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://storage.example.com/test-url"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "pltoken12345"

	mock.ExpectQuery(`SELECT p.id, p.title, p.description, p.share_password, p.require_email`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(playlistWatchColumns).AddRow(
			"playlist-1",
			"My Playlist",
			(*string)(nil),
			(*string)(nil),
			false,
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
		))

	thumbKey := "recordings/user-1/vtoken2abcde.jpg"
	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.share_token, v.content_type, v.user_id`).
		WithArgs("playlist-1").
		WillReturnRows(pgxmock.NewRows(playlistVideosColumns).
			AddRow("vid-1", "First Video", 120, "vtoken1abcde", "video/webm", "user-1", (*string)(nil)).
			AddRow("vid-2", "Second Video", 300, "vtoken2abcde", "video/mp4", "user-1", &thumbKey),
		)

	rec := servePlaylistWatchPage(handler, playlistWatchRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	checks := map[string]string{
		"playlist title":  "My Playlist",
		"first video":     "First Video",
		"second video":    "Second Video",
		"video element":   "<video",
		"video source":    `src="https://storage.example.com/test-url"`,
		"video list":      `class="video-list"`,
		"player counter":  "1 of 2",
		"nonce in style":  `nonce="test-nonce"`,
		"nonce in script": `<script nonce="test-nonce">`,
		"branding":        "SendRec",
	}
	for name, want := range checks {
		if !strings.Contains(body, want) {
			t.Errorf("expected %s (%q) in response body", name, want)
		}
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content type, got %s", ct)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPlaylistWatchPage_NotShared(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)

	mock.ExpectQuery(`SELECT p.id, p.title, p.description, p.share_password, p.require_email`).
		WithArgs("notshared123").
		WillReturnError(errors.New("no rows"))

	rec := servePlaylistWatchPage(handler, playlistWatchRequest("notshared123"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPlaylistWatchPage_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)

	mock.ExpectQuery(`SELECT p.id, p.title, p.description, p.share_password, p.require_email`).
		WithArgs("nonexistent1").
		WillReturnError(errors.New("no rows"))

	rec := servePlaylistWatchPage(handler, playlistWatchRequest("nonexistent1"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPlaylistWatchPage_PasswordProtected(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	passwordHash, _ := hashSharePassword("secret123")
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "pwdtoken1234"

	mock.ExpectQuery(`SELECT p.id, p.title, p.description, p.share_password, p.require_email`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(playlistWatchColumns).AddRow(
			"playlist-2",
			"Protected Playlist",
			(*string)(nil),
			&passwordHash,
			false,
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
		))

	rec := servePlaylistWatchPage(handler, playlistWatchRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "password") {
		t.Error("expected password form in response")
	}
	if !strings.Contains(body, "Protected Playlist") {
		t.Error("expected playlist title in password page")
	}
	if !strings.Contains(body, "/api/watch/playlist/"+shareToken+"/verify") {
		t.Error("expected verify endpoint URL in password form")
	}
	if strings.Contains(body, "<video") {
		t.Error("should not show video player when password is required")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestVerifyPlaylistWatchPassword_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	passwordHash, _ := hashSharePassword("secret123")
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "plpwdtoken12"

	mock.ExpectQuery(`SELECT share_password FROM playlists WHERE share_token = \$1 AND is_shared = true`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows([]string{"share_password"}).AddRow(&passwordHash))

	body, _ := json.Marshal(map[string]string{"password": "secret123"})
	r := chi.NewRouter()
	r.Post("/api/watch/playlist/{shareToken}/verify", handler.VerifyPlaylistWatchPassword)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/playlist/"+shareToken+"/verify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Name != watchCookieName(shareToken) {
		t.Errorf("expected cookie name %s, got %s", watchCookieName(shareToken), cookies[0].Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestVerifyPlaylistWatchPassword_Wrong(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	passwordHash, _ := hashSharePassword("secret123")
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "plpwdtoken12"

	mock.ExpectQuery(`SELECT share_password FROM playlists WHERE share_token = \$1 AND is_shared = true`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows([]string{"share_password"}).AddRow(&passwordHash))

	body, _ := json.Marshal(map[string]string{"password": "wrongpassword"})
	r := chi.NewRouter()
	r.Post("/api/watch/playlist/{shareToken}/verify", handler.VerifyPlaylistWatchPassword)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/playlist/"+shareToken+"/verify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestIdentifyPlaylistViewer_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "plidtoken123"

	mock.ExpectQuery(`SELECT id FROM playlists WHERE share_token = \$1 AND is_shared = true`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("playlist-3"))

	body := strings.NewReader(`{"email":"viewer@example.com"}`)
	r := chi.NewRouter()
	r.Post("/api/watch/playlist/{shareToken}/identify", handler.IdentifyPlaylistViewer)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/playlist/"+shareToken+"/identify", body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == emailGateCookieName(shareToken) {
			found = true
		}
	}
	if !found {
		t.Error("expected email gate cookie to be set")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPlaylistWatchPage_RendersWorkspaceBranding(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://storage.example.com/logo.png"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "plbrand1234"

	orgID := "42"
	companyName := "ACME Inc"
	logoKey := "branding/org42/logo.png"
	orgAccent := "#ff0000"
	footerText := "Powered by ACME"

	mock.ExpectQuery(`SELECT p.id, p.title, p.description, p.share_password, p.require_email`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(playlistWatchColumns).AddRow(
			"playlist-1", "Branded Playlist", (*string)(nil), (*string)(nil), false,
			&orgID,
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			&companyName, &logoKey,
			(*string)(nil), (*string)(nil), (*string)(nil), &orgAccent, &footerText, (*string)(nil),
		))

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.share_token, v.content_type, v.user_id`).
		WithArgs("playlist-1").
		WillReturnRows(pgxmock.NewRows(playlistVideosColumns).
			AddRow("vid-1", "First Video", 120, "vtoken1abcde", "video/webm", "user-1", (*string)(nil)))

	rec := servePlaylistWatchPage(handler, playlistWatchRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, "<title>Branded Playlist — ACME Inc</title>") {
		t.Error("expected workspace company name in page title")
	}
	if !strings.Contains(body, "--brand-accent: #ff0000") {
		t.Error("expected workspace accent color")
	}
	if !strings.Contains(body, `src="https://storage.example.com/logo.png"`) {
		t.Error("expected workspace logo URL in sidebar")
	}
	if !strings.Contains(body, "Powered by ACME") {
		t.Error("expected custom footer text")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPlaylistWatchPage_UsesPersonalBrandingWithoutWorkspace(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "plbrand1256"

	personalAccent := "#00ccff"
	personalCompany := "Solo Creator"

	mock.ExpectQuery(`SELECT p.id, p.title, p.description, p.share_password, p.require_email`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(playlistWatchColumns).AddRow(
			"playlist-1", "My Playlist", (*string)(nil), (*string)(nil), false,
			(*string)(nil),
			&personalCompany, (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), &personalAccent, (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
		))

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.share_token, v.content_type, v.user_id`).
		WithArgs("playlist-1").
		WillReturnRows(pgxmock.NewRows(playlistVideosColumns).
			AddRow("vid-1", "First Video", 120, "vtoken1abcde", "video/webm", "user-1", (*string)(nil)))

	rec := servePlaylistWatchPage(handler, playlistWatchRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, "--brand-accent: #00ccff") {
		t.Error("expected personal accent color for workspace-less playlist")
	}
	if !strings.Contains(body, "Solo Creator") {
		t.Error("expected personal company name")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPlaylistWatchPage_DefaultSurfaceWithoutBranding(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "plsurf1234"

	mock.ExpectQuery(`SELECT p.id, p.title, p.description, p.share_password, p.require_email`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(playlistWatchColumns).AddRow(
			"playlist-1", "Unbranded Playlist", (*string)(nil), (*string)(nil), false,
			(*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
		))

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.share_token, v.content_type, v.user_id`).
		WithArgs("playlist-1").
		WillReturnRows(pgxmock.NewRows(playlistVideosColumns).
			AddRow("vid-1", "First Video", 120, "vtoken1abcde", "video/webm", "user-1", (*string)(nil)))

	rec := servePlaylistWatchPage(handler, playlistWatchRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, "--brand-surface: #111d32") {
		t.Error("expected original navy panel shade as default brand surface")
	}
	if strings.Contains(body, "--brand-surface: #1e293b") {
		t.Error("default brand surface must not change to the shared slate surface")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPlaylistWatchPage_CustomSurfaceOverridesDefault(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "plsurf1256"

	customSurface := "#222233"

	mock.ExpectQuery(`SELECT p.id, p.title, p.description, p.share_password, p.require_email`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(playlistWatchColumns).AddRow(
			"playlist-1", "Custom Surface Playlist", (*string)(nil), (*string)(nil), false,
			(*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), &customSurface,
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
		))

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.share_token, v.content_type, v.user_id`).
		WithArgs("playlist-1").
		WillReturnRows(pgxmock.NewRows(playlistVideosColumns).
			AddRow("vid-1", "First Video", 120, "vtoken1abcde", "video/webm", "user-1", (*string)(nil)))

	rec := servePlaylistWatchPage(handler, playlistWatchRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, "--brand-surface: #222233") {
		t.Error("expected custom branded surface color to override the navy default")
	}
	if strings.Contains(body, "--brand-surface: #111d32") {
		t.Error("navy default must not appear when a custom surface is configured")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPlaylistWatchPage_LightThemeGateInputUsesBrandText(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	passwordHash, _ := hashSharePassword("secret123")
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "lightgate12"

	lightBg := "#f5f5f5"
	mock.ExpectQuery(`SELECT p.id, p.title, p.description, p.share_password, p.require_email`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(playlistWatchColumns).
			AddRow(lightBrandPlaylistRow("Light Gate Playlist", &passwordHash, &lightBg)...))

	rec := servePlaylistWatchPage(handler, playlistWatchRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, "--brand-surface: #ffffff") {
		t.Error("expected light brand surface to reach the gate page")
	}
	if !strings.Contains(body, "--brand-text: #000000") {
		t.Error("expected light brand text color to reach the gate page")
	}
	rule := cssRuleBody(body, `.gate-container input[type="password"]`)
	if !strings.Contains(rule, "background: var(--brand-surface)") {
		t.Error("gate input must use the brand surface for its background")
	}
	if !strings.Contains(rule, "color: var(--brand-text)") {
		t.Error("gate input must use the brand text color for its foreground")
	}
	if strings.Contains(rule, "color: #fff") {
		t.Error("gate input must not hardcode a white foreground (unreadable on light surfaces)")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPlaylistWatchPage_LightThemeActiveRowDerivesBrandColors(t *testing.T) {
	mock, body := renderLightPlaylistWatchPage(t, "lightrow123")
	defer mock.Close()

	if !strings.Contains(body, "--brand-text: #000000") {
		t.Error("expected light brand text color to reach the playlist page")
	}
	hoverRule := cssRuleBody(body, ".video-list-item:hover")
	if !strings.Contains(hoverRule, "background: color-mix(in srgb, var(--brand-text) 8%, var(--brand-surface))") {
		t.Error("hover row must derive its background from the brand palette")
	}
	activeRule := cssRuleBody(body, ".video-list-item.active")
	if !strings.Contains(activeRule, "background: color-mix(in srgb, var(--brand-text) 20%, var(--brand-surface))") {
		t.Error("active row must derive from the brand palette with a stronger tint than the hover row (identical colours make two rows look active, and the default navy row becomes too subtle)")
	}
	if strings.Contains(activeRule, "background: #1e3a5f") {
		t.Error("active row must not keep a fixed dark background")
	}
	for selector, want := range map[string]string{
		".video-list-item.active .video-title":     "font-weight: 600",
		".video-list-item.active .position":        "font-weight: 600",
		".video-list-item.active .now-playing-tag": "display: block",
	} {
		rule := cssRuleBody(body, selector)
		if !strings.Contains(rule, want) {
			t.Errorf("%s must declare %q", selector, want)
		}
		if strings.Contains(rule, "color:") {
			t.Errorf("%s must not set a foreground color (it must follow the brand palette, readable on a derived light background)", selector)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPlaylistWatchPage_LightThemeNextOverlayForeground(t *testing.T) {
	mock, body := renderLightPlaylistWatchPage(t, "lightnext123")
	defer mock.Close()

	if !strings.Contains(body, "--brand-text: #000000") {
		t.Error("expected light brand text color to reach the playlist page")
	}
	overlayRule := cssRuleBody(body, ".next-overlay")
	if !strings.Contains(overlayRule, "background: rgba(0, 0, 0, 0.85)") {
		t.Error("next overlay must keep its fixed dark background")
	}
	if !strings.Contains(overlayRule, "color: #fff") {
		t.Error("next overlay must use white text (brand text on the dark overlay is unreadable in a light theme)")
	}
	if strings.Contains(overlayRule, "color: var(--brand-text)") {
		t.Error("next overlay must not inherit brand text (black on the dark overlay is unreadable)")
	}
	titleRule := cssRuleBody(body, ".next-overlay .next-title")
	if !strings.Contains(titleRule, "color: #fff") {
		t.Error("next overlay title must override the brand text color")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// lightBrandPlaylistRow returns the watch-page query row values for a
// workspace-branded light-theme playlist (white surface, black text, blue accent).
func lightBrandPlaylistRow(title string, sharePassword, colorBackground *string) []any {
	lightCo := "Light Co"
	lightSurface := "#ffffff"
	lightText := "#000000"
	lightAccent := "#1d4ed8"
	return []any{
		"playlist-1", title, (*string)(nil), sharePassword, false,
		(*string)(nil),
		&lightCo, (*string)(nil), colorBackground, &lightSurface,
		&lightText, &lightAccent, (*string)(nil), (*string)(nil),
		(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
		(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
	}
}

// renderLightPlaylistWatchPage renders the watch page for an ungated
// light-themed workspace playlist and returns the mock pool and response body.
func renderLightPlaylistWatchPage(t *testing.T, shareToken string) (pgxmock.PgxPoolIface, string) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)

	mock.ExpectQuery(`SELECT p.id, p.title, p.description, p.share_password, p.require_email`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(playlistWatchColumns).
			AddRow(lightBrandPlaylistRow("Light Playlist", nil, nil)...))
	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.share_token, v.content_type, v.user_id`).
		WithArgs("playlist-1").
		WillReturnRows(pgxmock.NewRows(playlistVideosColumns).
			AddRow("vid-1", "First Video", 120, "vtoken1abcde", "video/webm", "user-1", (*string)(nil)))

	rec := servePlaylistWatchPage(handler, playlistWatchRequest(shareToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	return mock, rec.Body.String()
}

// cssRuleBody returns the declarations of the first CSS rule with the given
// selector, whitespace collapsed, so assertions are not sensitive to CSS
// reformatting. It returns "" when the rule does not exist.
func cssRuleBody(body, selector string) string {
	idx := strings.Index(body, selector)
	if idx == -1 {
		return ""
	}
	open := strings.IndexByte(body[idx:], '{')
	if open == -1 {
		return ""
	}
	open += idx
	closeIdx := strings.IndexByte(body[open+1:], '}')
	if closeIdx == -1 {
		return ""
	}
	return strings.Join(strings.Fields(body[open+1:open+1+closeIdx]), " ")
}
