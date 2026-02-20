package video

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
)

const testHMACSecret = "test-hmac-secret-for-watch-auth"

func TestHashSharePassword_ReturnsBcryptHash(t *testing.T) {
	hash, err := hashSharePassword("mysecret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if hash == "mysecret" {
		t.Error("hash should not equal plaintext password")
	}
}

func TestCheckSharePassword_CorrectPassword(t *testing.T) {
	hash, err := hashSharePassword("mysecret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !checkSharePassword(hash, "mysecret") {
		t.Error("expected correct password to match")
	}
}

func TestCheckSharePassword_WrongPassword(t *testing.T) {
	hash, err := hashSharePassword("mysecret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checkSharePassword(hash, "wrongpassword") {
		t.Error("expected wrong password to not match")
	}
}

func TestWatchCookieName_UsesTokenPrefix(t *testing.T) {
	name := watchCookieName("abcdefghijkl")
	if name != "wa_abcdefgh" {
		t.Errorf("expected wa_abcdefgh, got %s", name)
	}
}

func TestWatchCookieName_ShortToken(t *testing.T) {
	name := watchCookieName("abc")
	if name != "wa_abc" {
		t.Errorf("expected wa_abc, got %s", name)
	}
}

func TestSignWatchCookie_ReturnsNonEmpty(t *testing.T) {
	sig := signWatchCookie(testHMACSecret, "sharetoken", "$2a$10$abcdefghij1234567890ab")
	if sig == "" {
		t.Error("expected non-empty signature")
	}
}

func TestSignWatchCookie_DeterministicForSameInput(t *testing.T) {
	sig1 := signWatchCookie(testHMACSecret, "sharetoken", "$2a$10$abcdefghij1234567890ab")
	sig2 := signWatchCookie(testHMACSecret, "sharetoken", "$2a$10$abcdefghij1234567890ab")
	if sig1 != sig2 {
		t.Error("expected same signature for same input")
	}
}

func TestSignWatchCookie_DifferentForDifferentPasswordHash(t *testing.T) {
	sig1 := signWatchCookie(testHMACSecret, "sharetoken", "$2a$10$aaaaaaaaaaaaaaaaaaaaa")
	sig2 := signWatchCookie(testHMACSecret, "sharetoken", "$2a$10$bbbbbbbbbbbbbbbbbbbbb")
	if sig1 == sig2 {
		t.Error("expected different signatures for different password hashes")
	}
}

func TestVerifyWatchCookie_ValidSignature(t *testing.T) {
	sig := signWatchCookie(testHMACSecret, "sharetoken", "$2a$10$abcdefghij1234567890ab")
	if !verifyWatchCookie(testHMACSecret, "sharetoken", "$2a$10$abcdefghij1234567890ab", sig) {
		t.Error("expected valid signature to verify")
	}
}

func TestVerifyWatchCookie_InvalidSignature(t *testing.T) {
	if verifyWatchCookie(testHMACSecret, "sharetoken", "$2a$10$abcdefghij1234567890ab", "invalidsig") {
		t.Error("expected invalid signature to fail verification")
	}
}

func TestVerifyWatchCookie_WrongSecret(t *testing.T) {
	sig := signWatchCookie(testHMACSecret, "sharetoken", "$2a$10$abcdefghij1234567890ab")
	if verifyWatchCookie("wrong-secret", "sharetoken", "$2a$10$abcdefghij1234567890ab", sig) {
		t.Error("expected verification to fail with wrong secret")
	}
}

func TestSetWatchCookie_SetsCookieOnResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	setWatchCookie(rec, "abcdefghijkl", "sigvalue", false)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != "wa_abcdefgh" {
		t.Errorf("expected cookie name wa_abcdefgh, got %s", c.Name)
	}
	if c.Value != "sigvalue" {
		t.Errorf("expected cookie value sigvalue, got %s", c.Value)
	}
	if !c.HttpOnly {
		t.Error("expected HttpOnly cookie")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("expected SameSiteStrict, got %v", c.SameSite)
	}
	if c.Secure {
		t.Error("expected non-secure cookie when secure=false")
	}
}

func TestSetWatchCookie_SecureFlag(t *testing.T) {
	rec := httptest.NewRecorder()
	setWatchCookie(rec, "abcdefghijkl", "sigvalue", true)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if !cookies[0].Secure {
		t.Error("expected secure cookie when secure=true")
	}
}

func TestHasValidWatchCookie_ValidCookie(t *testing.T) {
	passwordHash := "$2a$10$abcdefghij1234567890ab"
	sig := signWatchCookie(testHMACSecret, "sharetoken12", passwordHash)

	req := httptest.NewRequest(http.MethodGet, "/watch/sharetoken12", nil)
	req.AddCookie(&http.Cookie{
		Name:  watchCookieName("sharetoken12"),
		Value: sig,
	})

	if !hasValidWatchCookie(req, testHMACSecret, "sharetoken12", passwordHash) {
		t.Error("expected valid cookie to pass verification")
	}
}

func TestHasValidWatchCookie_NoCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/watch/sharetoken12", nil)
	if hasValidWatchCookie(req, testHMACSecret, "sharetoken12", "$2a$10$abcdefghij1234567890ab") {
		t.Error("expected no cookie to fail verification")
	}
}

func TestHasValidWatchCookie_InvalidCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/watch/sharetoken12", nil)
	req.AddCookie(&http.Cookie{
		Name:  watchCookieName("sharetoken12"),
		Value: "garbage",
	})

	if hasValidWatchCookie(req, testHMACSecret, "sharetoken12", "$2a$10$abcdefghij1234567890ab") {
		t.Error("expected invalid cookie to fail verification")
	}
}

// --- VerifyWatchPassword Handler Tests ---

func TestVerifyWatchPassword_CorrectPassword(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	passwordHash, _ := hashSharePassword("secret123")
	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "abc123defghi"

	mock.ExpectQuery(`SELECT share_password FROM videos WHERE share_token = \$1 AND status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows([]string{"share_password"}).AddRow(&passwordHash))

	body, _ := json.Marshal(map[string]string{"password": "secret123"})
	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/verify", handler.VerifyWatchPassword)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/verify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Name != watchCookieName(shareToken) {
		t.Errorf("expected cookie name %s, got %s", watchCookieName(shareToken), cookies[0].Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestVerifyWatchPassword_WrongPassword(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	passwordHash, _ := hashSharePassword("secret123")
	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "abc123defghi"

	mock.ExpectQuery(`SELECT share_password FROM videos WHERE share_token = \$1 AND status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows([]string{"share_password"}).AddRow(&passwordHash))

	body, _ := json.Marshal(map[string]string{"password": "wrongpassword"})
	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/verify", handler.VerifyWatchPassword)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/verify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestVerifyWatchPassword_NoPasswordSet(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "abc123defghi"

	mock.ExpectQuery(`SELECT share_password FROM videos WHERE share_token = \$1 AND status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows([]string{"share_password"}).AddRow((*string)(nil)))

	body, _ := json.Marshal(map[string]string{"password": "anything"})
	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/verify", handler.VerifyWatchPassword)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/verify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestVerifyWatchPassword_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "abc123defghi"

	mock.ExpectQuery(`SELECT share_password FROM videos WHERE share_token = \$1 AND status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnError(errors.New("no rows"))

	body, _ := json.Marshal(map[string]string{"password": "secret123"})
	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/verify", handler.VerifyWatchPassword)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/verify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- Password-Protected Watch Handler Tests ---

func TestWatch_PasswordProtected_NoCookie(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	passwordHash, _ := hashSharePassword("secret123")
	storage := &mockStorage{downloadURL: "https://s3.example.com/download"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "duration", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password", "transcript_key", "transcript_json", "transcript_status", "user_id", "email", "view_notification", "content_type", "ub_company_name", "ub_logo_key", "ub_color_background", "ub_color_surface", "ub_color_text", "ub_color_accent", "ub_footer_text", "ub_custom_css", "vb_company_name", "vb_logo_key", "vb_color_background", "vb_color_surface", "vb_color_text", "vb_color_accent", "vb_footer_text", "cta_text", "cta_url"}).
				AddRow("video-001", "Demo Recording", 180, "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, &shareExpiresAt, (*string)(nil), &passwordHash, (*string)(nil), (*string)(nil), "none", "owner-user-id", "owner@example.com", (*string)(nil), "video/webm", (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil)),
		)

	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.Watch)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestWatch_PasswordProtected_ValidCookie(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	passwordHash, _ := hashSharePassword("secret123")
	storage := &mockStorage{downloadURL: "https://s3.example.com/download"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)
	videoID := "video-001"

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "duration", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password", "transcript_key", "transcript_json", "transcript_status", "user_id", "email", "view_notification", "content_type", "ub_company_name", "ub_logo_key", "ub_color_background", "ub_color_surface", "ub_color_text", "ub_color_accent", "ub_footer_text", "ub_custom_css", "vb_company_name", "vb_logo_key", "vb_color_background", "vb_color_surface", "vb_color_text", "vb_color_accent", "vb_footer_text", "cta_text", "cta_url"}).
				AddRow(videoID, "Demo Recording", 180, "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, &shareExpiresAt, (*string)(nil), &passwordHash, (*string)(nil), (*string)(nil), "none", "owner-user-id", "owner@example.com", (*string)(nil), "video/webm", (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil)),
		)

	mock.ExpectExec(`INSERT INTO video_views`).
		WithArgs(videoID, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	sig := signWatchCookie(testHMACSecret, shareToken, passwordHash)

	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.Watch)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	req.AddCookie(&http.Cookie{
		Name:  watchCookieName(shareToken),
		Value: sig,
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	time.Sleep(100 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestWatchDownload_PasswordProtected_NoCookie(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	passwordHash, _ := hashSharePassword("secret123")
	storage := &mockStorage{downloadDispositionURL: "https://s3.example.com/download"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "abc123defghi"
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT title, file_key, share_expires_at, share_password, content_type, download_enabled FROM videos WHERE share_token = \$1 AND status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows([]string{"title", "file_key", "share_expires_at", "share_password", "content_type", "download_enabled"}).
			AddRow("Demo Recording", "recordings/user-1/abc.webm", &shareExpiresAt, &passwordHash, "video/webm", true))

	r := chi.NewRouter()
	r.Get("/api/watch/{shareToken}/download", handler.WatchDownload)

	req := httptest.NewRequest(http.MethodGet, "/api/watch/"+shareToken+"/download", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestEmailGateCookie(t *testing.T) {
	hmacSecret := "test-secret"
	shareToken := "token1234567"

	t.Run("cookie name", func(t *testing.T) {
		name := emailGateCookieName(shareToken)
		if name != "eg_token123" {
			t.Fatalf("expected eg_token123, got %s", name)
		}
	})

	t.Run("sign and verify", func(t *testing.T) {
		email := "alice@example.com"
		value := signEmailGateCookie(hmacSecret, shareToken, email)
		got, ok := verifyEmailGateCookie(hmacSecret, shareToken, value)
		if !ok {
			t.Fatal("expected valid cookie")
		}
		if got != email {
			t.Fatalf("expected email %s, got %s", email, got)
		}
	})

	t.Run("invalid signature rejected", func(t *testing.T) {
		_, ok := verifyEmailGateCookie(hmacSecret, shareToken, "bad@email.com|invalidsig")
		if ok {
			t.Fatal("expected invalid cookie")
		}
	})
}

func TestIdentifyViewer(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testHMACSecret, false)

	mock.ExpectQuery(`SELECT v.id FROM videos`).
		WithArgs("validtoken1").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("vid-1"))

	mock.ExpectExec(`INSERT INTO video_viewers`).
		WithArgs("vid-1", "alice@example.com", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := strings.NewReader(`{"email":"alice@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/watch/validtoken1/identify", body)
	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/identify", handler.IdentifyViewer)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "eg_validtok" {
			found = true
		}
	}
	if !found {
		t.Error("expected email gate cookie to be set")
	}

	time.Sleep(100 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_PasswordProtected_ShowsPasswordForm(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	passwordHash, _ := hashSharePassword("secret123")
	storage := &mockStorage{downloadURL: "https://s3.example.com/download"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password", "comment_mode", "transcript_key", "transcript_json", "transcript_status", "user_id", "email", "view_notification", "content_type", "ub_company_name", "ub_logo_key", "ub_color_background", "ub_color_surface", "ub_color_text", "ub_color_accent", "ub_footer_text", "ub_custom_css", "vb_company_name", "vb_logo_key", "vb_color_background", "vb_color_surface", "vb_color_text", "vb_color_accent", "vb_footer_text", "download_enabled", "cta_text", "cta_url", "email_gate_enabled"}).
				AddRow("vid-1", "Demo Recording", "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, &shareExpiresAt, (*string)(nil), &passwordHash, "disabled", (*string)(nil), (*string)(nil), "none", "owner-user-id", "owner@example.com", (*string)(nil), "video/webm", (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), true, (*string)(nil), (*string)(nil), false),
		)

	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.WatchPage)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "password protected") {
		t.Error("expected password page to contain 'password protected'")
	}
	if !strings.Contains(body, "password-form") {
		t.Error("expected password page to contain password form")
	}
	if strings.Contains(body, "<video") {
		t.Error("expected password page to NOT contain video player")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}
