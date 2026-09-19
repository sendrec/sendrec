package video

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/sendrec/sendrec/internal/httputil"
)

var playlistEmbedColumns = []string{
	"id", "title", "share_password", "require_email", "organization_id",
	"ub_company_name", "ub_logo_key", "ub_color_background", "ub_color_surface",
	"ub_color_text", "ub_color_accent", "ub_footer_text", "ub_custom_css",
	"ob_company_name", "ob_logo_key", "ob_color_background", "ob_color_surface",
	"ob_color_text", "ob_color_accent", "ob_footer_text", "ob_custom_css",
}

func servePlaylistEmbedPage(handler *Handler, req *http.Request) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Get("/embed/playlist/{shareToken}", handler.PlaylistEmbedPage)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestPlaylistEmbedPage_RendersWorkspaceAccent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "plembbrand1"

	orgID := "42"
	orgAccent := "#ff0000"

	mock.ExpectQuery(`SELECT p.id, p.title, p.share_password, p.require_email`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(playlistEmbedColumns).AddRow(
			"playlist-1", "Branded Playlist", (*string)(nil), false,
			&orgID,
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), &orgAccent, (*string)(nil), (*string)(nil),
		))

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.share_token, v.content_type, v.user_id`).
		WithArgs("playlist-1").
		WillReturnRows(pgxmock.NewRows(playlistVideosColumns).
			AddRow("vid-1", "First Video", 120, "vtoken1abcde", "video/webm", "user-1", (*string)(nil)))

	req := httptest.NewRequest(http.MethodGet, "/embed/playlist/"+shareToken, nil)
	req = req.WithContext(httputil.ContextWithNonce(req.Context(), "test-nonce"))

	rec := servePlaylistEmbedPage(handler, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "--player-accent: #ff0000") {
		t.Error("expected workspace accent color on playlist embed player")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPlaylistEmbedPage_PasswordGate_RendersWorkspaceAccent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "plembgate1"

	passwordHash, _ := hashSharePassword("secret123")
	orgID := "42"
	orgAccent := "#ff0000"

	mock.ExpectQuery(`SELECT p.id, p.title, p.share_password, p.require_email`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(playlistEmbedColumns).AddRow(
			"playlist-1", "Gated Playlist", &passwordHash, false,
			&orgID,
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), &orgAccent, (*string)(nil), (*string)(nil),
		))

	req := httptest.NewRequest(http.MethodGet, "/embed/playlist/"+shareToken, nil)
	req = req.WithContext(httputil.ContextWithNonce(req.Context(), "test-nonce"))

	rec := servePlaylistEmbedPage(handler, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "password") {
		t.Error("expected password form on gate page")
	}
	if !strings.Contains(body, "--player-accent: #ff0000") {
		t.Error("expected workspace accent color on playlist embed password gate")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPlaylistEmbedPage_DefaultAccentWithoutBranding(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "plembplain1"

	mock.ExpectQuery(`SELECT p.id, p.title, p.share_password, p.require_email`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(playlistEmbedColumns).AddRow(
			"playlist-1", "Plain Playlist", (*string)(nil), false,
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

	req := httptest.NewRequest(http.MethodGet, "/embed/playlist/"+shareToken, nil)
	req = req.WithContext(httputil.ContextWithNonce(req.Context(), "test-nonce"))

	rec := servePlaylistEmbedPage(handler, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "--player-accent: #00b67a") {
		t.Error("expected default accent color when no branding is configured")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
