package video

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
)

// SR-02 / SR-08: the share-password and email-gate checks were enforced only on
// the server-rendered watch pages. Every JSON/redirect surface handed out the
// content — presigned video URL, download URL, thumbnail, oEmbed metadata — to
// anyone holding the share link. These tests pin the gate to the data APIs.

var watchAPIColumns = []string{
	"id", "title", "duration", "file_key", "name", "created_at", "share_expires_at",
	"thumbnail_key", "share_password", "transcript_key", "transcript_json", "transcript_status",
	"user_id", "email", "view_notification", "content_type",
	"ub_company_name", "ub_logo_key", "ub_color_background", "ub_color_surface", "ub_color_text", "ub_color_accent", "ub_footer_text", "ub_custom_css",
	"ob_company_name", "ob_logo_key", "ob_color_background", "ob_color_surface", "ob_color_text", "ob_color_accent", "ob_footer_text", "ob_custom_css",
	"vb_company_name", "vb_logo_key", "vb_color_background", "vb_color_surface", "vb_color_text", "vb_color_accent", "vb_footer_text",
	"cta_text", "cta_url", "summary", "chapters", "summary_status",
	"document", "document_status", "organization_id", "email_gate_enabled",
}

// watchAPIRow builds a Watch row with the gate flags under test.
func watchAPIRow(videoID string, sharePassword *string, emailGateEnabled bool, shareExpiresAt *time.Time) *pgxmock.Rows {
	return pgxmock.NewRows(watchAPIColumns).AddRow(
		videoID, "Demo Recording", 180, "recordings/user-1/abc.webm", "Alex Neamtu",
		time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC), shareExpiresAt,
		(*string)(nil), sharePassword, (*string)(nil), (*string)(nil), "none",
		"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
		(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
		(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
		(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
		(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), "none",
		(*string)(nil), "none", (*string)(nil), emailGateEnabled,
	)
}

func serveWatchAPI(handler *Handler, req *http.Request) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Get("/api/watch/{shareToken}", handler.Watch)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func gateCookie(t *testing.T, shareToken, email string) *http.Cookie {
	t.Helper()
	return &http.Cookie{
		Name:  emailGateCookieName(shareToken),
		Value: signEmailGateCookie(testHMACSecret, shareToken, email),
	}
}

func TestWatchAPI_EmailGateEnabled_NoCookie_IsDenied(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{downloadURL: "https://s3.example.com/video"}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(watchAPIRow("video-001", nil, true, &expiresAt))

	rec := serveWatchAPI(handler, httptest.NewRequest(http.MethodGet, "/api/watch/"+shareToken, nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for gated video without an email cookie, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); strings.Contains(body, "s3.example.com") {
		t.Errorf("presigned video URL leaked past the email gate: %s", body)
	}
}

func TestWatchAPI_EmailGateEnabled_ValidCookie_IsAllowed(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{downloadURL: "https://s3.example.com/video"}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(watchAPIRow("video-001", nil, true, &expiresAt))
	mock.ExpectExec(`INSERT INTO video_views`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	req := httptest.NewRequest(http.MethodGet, "/api/watch/"+shareToken, nil)
	req.AddCookie(gateCookie(t, shareToken, "viewer@example.com"))
	rec := serveWatchAPI(handler, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for identified viewer, got %d: %s", rec.Code, rec.Body.String())
	}
	time.Sleep(50 * time.Millisecond)
}

func TestWatchAPI_EmailGateDisabled_IsAllowed(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{downloadURL: "https://s3.example.com/video"}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(watchAPIRow("video-001", nil, false, &expiresAt))
	mock.ExpectExec(`INSERT INTO video_views`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	rec := serveWatchAPI(handler, httptest.NewRequest(http.MethodGet, "/api/watch/"+shareToken, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("ungated video must still play, got %d: %s", rec.Code, rec.Body.String())
	}
	time.Sleep(50 * time.Millisecond)
}

func TestWatchDownload_EmailGateEnabled_NoCookie_IsDenied(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{downloadDispositionURL: "https://s3.example.com/dl"}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT title, file_key, share_expires_at, share_password, content_type, download_enabled`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows([]string{"title", "file_key", "share_expires_at", "share_password", "content_type", "download_enabled", "email_gate_enabled"}).
			AddRow("Demo Recording", "recordings/user-1/abc.webm", &expiresAt, (*string)(nil), "video/webm", true, true))

	r := chi.NewRouter()
	r.Get("/api/watch/{shareToken}/download", handler.WatchDownload)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/watch/"+shareToken+"/download", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for gated download, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "s3.example.com") {
		t.Errorf("download URL leaked past the email gate: %s", rec.Body.String())
	}
}

func TestWatchThumbnail_PasswordProtected_NoCookie_IsDenied(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{downloadURL: "https://s3.example.com/thumb.jpg"}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "validtoken12"
	thumbKey := "recordings/u1/thumb.jpg"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	hashed := "$2a$10$abcdefghijklmnopqrstuv"

	mock.ExpectQuery(`SELECT v.thumbnail_key, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows([]string{"thumbnail_key", "share_expires_at", "share_password", "email_gate_enabled"}).
			AddRow(&thumbKey, &expiresAt, &hashed, false))

	rec := serveWatchThumbnail(handler, httptest.NewRequest(http.MethodGet, "/api/watch/"+shareToken+"/thumbnail", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a password-protected thumbnail, got %d (Location=%q)", rec.Code, rec.Header().Get("Location"))
	}
}

func TestWatchThumbnail_EmailGateEnabled_NoCookie_IsDenied(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{downloadURL: "https://s3.example.com/thumb.jpg"}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "validtoken12"
	thumbKey := "recordings/u1/thumb.jpg"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.thumbnail_key, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows([]string{"thumbnail_key", "share_expires_at", "share_password", "email_gate_enabled"}).
			AddRow(&thumbKey, &expiresAt, (*string)(nil), true))

	rec := serveWatchThumbnail(handler, httptest.NewRequest(http.MethodGet, "/api/watch/"+shareToken+"/thumbnail", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for an email-gated thumbnail, got %d (Location=%q)", rec.Code, rec.Header().Get("Location"))
	}
}

func TestOEmbed_PasswordProtected_IsDenied(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{downloadURL: "https://s3.example.com/thumb.jpg"}, testBaseURL, 0, 0, 0, 0, testHMACSecret, false)
	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 10, 14, 30, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	hashed := "$2a$10$abcdefghijklmnopqrstuv"

	mock.ExpectQuery(`SELECT v.title, v.duration, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows([]string{"title", "duration", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password", "email_gate_enabled"}).
			AddRow("Secret Recording", 180, "Alex Neamtu", createdAt, &expiresAt, stringPtr("thumbnails/user-1/abc.jpg"), &hashed, false))

	r := chi.NewRouter()
	r.Get("/api/videos/{shareToken}/oembed", handler.OEmbed)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/videos/"+shareToken+"/oembed", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 from oEmbed for a password-protected video, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Secret Recording") {
		t.Errorf("oEmbed leaked the title of a password-protected video: %s", rec.Body.String())
	}
}
