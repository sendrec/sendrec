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
	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/httputil"
)

var playlistWatchColumns = []string{
	"id", "title", "description", "share_password", "require_email",
}

var playlistVideosColumns = []string{
	"id", "title", "duration", "share_token", "content_type", "user_id", "has_thumbnail",
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "pltoken12345"

	mock.ExpectQuery(`SELECT p.id, p.title, p.description, p.share_password, p.require_email`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(playlistWatchColumns).AddRow(
			"playlist-1", "My Playlist", (*string)(nil), (*string)(nil), false,
		))

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.share_token, v.content_type, v.user_id`).
		WithArgs("playlist-1").
		WillReturnRows(pgxmock.NewRows(playlistVideosColumns).
			AddRow("vid-1", "First Video", 120, "vtoken1abcde", "video/webm", "user-1", false).
			AddRow("vid-2", "Second Video", 300, "vtoken2abcde", "video/mp4", "user-1", true),
		)

	rec := servePlaylistWatchPage(handler, playlistWatchRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	checks := map[string]string{
		"playlist title":   "My Playlist",
		"first video":      "First Video",
		"second video":     "Second Video",
		"video element":    "<video",
		"video source":     `src="https://storage.example.com/test-url"`,
		"video list":       `class="video-list"`,
		"player counter":   "1 of 2",
		"nonce in style":   `nonce="test-nonce"`,
		"nonce in script":  `<script nonce="test-nonce">`,
		"branding":         "SendRec",
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

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testHMACSecret, false)

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

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testHMACSecret, false)

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
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "pwdtoken1234"

	mock.ExpectQuery(`SELECT p.id, p.title, p.description, p.share_password, p.require_email`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(playlistWatchColumns).AddRow(
			"playlist-2", "Protected Playlist", (*string)(nil), &passwordHash, false,
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
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testHMACSecret, false)
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
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testHMACSecret, false)
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

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testHMACSecret, false)
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
