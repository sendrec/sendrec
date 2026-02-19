package video

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/httputil"
)

var embedPageColumns = []string{
	"id", "title", "file_key", "name", "created_at", "share_expires_at",
	"thumbnail_key", "share_password", "content_type",
	"user_id", "email", "view_notification",
	"cta_text", "cta_url",
}

func embedPageRequest(shareToken string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/embed/"+shareToken, nil)
	ctx := httputil.ContextWithNonce(req.Context(), "test-nonce")
	return req.WithContext(ctx)
}

func serveEmbedPage(handler *Handler, req *http.Request) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Get("/embed/{shareToken}", handler.EmbedPage)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestEmbedPage_NotFound_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testHMACSecret, false)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs("nonexistent").
		WillReturnError(errors.New("no rows"))

	rec := serveEmbedPage(handler, embedPageRequest("nonexistent"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Video not found") {
		t.Error("expected 'Video not found' in response")
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content type, got %s", ct)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEmbedPage_Expired_Returns410(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{downloadURL: "https://s3.example.com/video"}, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "expiredtoken1"
	createdAt := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	expiredAt := time.Now().Add(-24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(embedPageColumns).AddRow(
			"vid-1", "Old Video", "recordings/u1/old.webm", "Alice", createdAt, &expiredAt,
			(*string)(nil), (*string)(nil), "video/webm",
			"owner-user-id", "owner@example.com", (*string)(nil),
			(*string)(nil), (*string)(nil),
		))

	rec := serveEmbedPage(handler, embedPageRequest(shareToken))

	if rec.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "This link has expired") {
		t.Error("expected expired message in response")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEmbedPage_Success_RendersVideoPlayer(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "validtoken12"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(embedPageColumns).AddRow(
			"vid-1", "My Demo", "recordings/u1/abc.webm", "Bob Smith", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "video/webm",
			"owner-user-id", "owner@example.com", (*string)(nil),
			(*string)(nil), (*string)(nil),
		))

	rec := serveEmbedPage(handler, embedPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	checks := map[string]string{
		"video element": "<video",
		"controls":      "controls",
		"playsinline":   "playsinline",
		"video source":  `src="https://s3.example.com/video.webm"`,
		"watch link":    `/watch/` + shareToken,
		"watch on text": "Watch on",
	}
	for name, want := range checks {
		if !strings.Contains(body, want) {
			t.Errorf("expected %s (%q) in response", name, want)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEmbedPage_WithThumbnail_RendersPoster(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "thumbtoken12"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	thumbKey := "recordings/u1/thumb.jpg"

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(embedPageColumns).AddRow(
			"vid-1", "Thumb Video", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			&thumbKey, (*string)(nil), "video/webm",
			"owner-user-id", "owner@example.com", (*string)(nil),
			(*string)(nil), (*string)(nil),
		))

	rec := serveEmbedPage(handler, embedPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "poster=") {
		t.Error("expected poster attribute on video element")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEmbedPage_PasswordProtected_NoCookie_ShowsPasswordForm(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	passwordHash, _ := hashSharePassword("secret123")
	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "pwdtoken1234"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(embedPageColumns).AddRow(
			"vid-1", "Protected Video", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), &passwordHash, "video/webm",
			"owner-user-id", "owner@example.com", (*string)(nil),
			(*string)(nil), (*string)(nil),
		))

	rec := serveEmbedPage(handler, embedPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "password") {
		t.Error("expected password form in response")
	}
	if strings.Contains(body, "<video") {
		t.Error("expected no video player on password page")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEmbedPage_RecordsView(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "viewtoken123"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(embedPageColumns).AddRow(
			"vid-1", "My Demo", "recordings/u1/abc.webm", "Bob Smith", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "video/webm",
			"owner-user-id", "owner@example.com", (*string)(nil),
			(*string)(nil), (*string)(nil),
		))

	mock.ExpectExec(`INSERT INTO video_views`).
		WithArgs("vid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	rec := serveEmbedPage(handler, embedPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Give the goroutine time to execute
	time.Sleep(100 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEmbedPage_CSPNonce(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "noncetoken12"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(embedPageColumns).AddRow(
			"vid-1", "Nonce Test", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "video/webm",
			"owner-user-id", "owner@example.com", (*string)(nil),
			(*string)(nil), (*string)(nil),
		))

	rec := serveEmbedPage(handler, embedPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, `nonce="test-nonce"`) {
		t.Error("expected CSP nonce in style and script tags")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEmbedPage_NeverExpires(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)

	createdAt := time.Now().Add(-30 * 24 * time.Hour)

	mock.ExpectQuery("SELECT v.id").
		WithArgs("token-never").
		WillReturnRows(
			pgxmock.NewRows(embedPageColumns).AddRow(
				"vid-1", "Never Expire Video", "recordings/u1/abc.webm", "Bob", createdAt, (*time.Time)(nil),
				(*string)(nil), (*string)(nil), "video/webm",
				"owner-user-id", "owner@example.com", (*string)(nil),
				(*string)(nil), (*string)(nil),
			),
		)

	mock.ExpectExec("INSERT INTO video_views").
		WithArgs("vid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	req := embedPageRequest("token-never")
	rec := serveEmbedPage(handler, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Never Expire Video") {
		t.Error("expected video title in response")
	}

	time.Sleep(100 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEmbedPage_ResponsiveLayout(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "responsive01"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(embedPageColumns).AddRow(
			"vid-1", "Responsive", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "video/webm",
			"owner-user-id", "owner@example.com", (*string)(nil),
			(*string)(nil), (*string)(nil),
		))

	rec := serveEmbedPage(handler, embedPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "width: 100%") {
		t.Error("expected width: 100% for responsive embed layout")
	}
	if !strings.Contains(body, "height: 100%") {
		t.Error("expected height: 100% for responsive embed layout")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEmbedPage_RendersCtaOverlay(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)

	ctaText := "Get started"
	ctaUrl := "https://example.com/start"
	shareToken := "ctatoken1234"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(embedPageColumns).AddRow(
			"vid-1", "CTA Video", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "video/webm",
			"owner-user-id", "owner@example.com", (*string)(nil),
			&ctaText, &ctaUrl,
		))

	mock.ExpectExec(`INSERT INTO video_views`).
		WithArgs("vid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	rec := serveEmbedPage(handler, embedPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, `id="cta-overlay"`) {
		t.Error("expected CTA overlay element")
	}
	if !strings.Contains(body, "Get started") {
		t.Error("expected CTA text")
	}
	if !strings.Contains(body, "https://example.com/start") {
		t.Error("expected CTA URL")
	}

	time.Sleep(100 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEmbedPage_MilestoneTrackingScript(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)

	shareToken := "milestone-test"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(embedPageColumns).AddRow(
			"vid-1", "Test Video", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "video/webm",
			"owner-user-id", "owner@example.com", (*string)(nil),
			(*string)(nil), (*string)(nil),
		))

	mock.ExpectExec(`INSERT INTO video_views`).
		WithArgs("vid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	rec := serveEmbedPage(handler, embedPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "/milestone") {
		t.Error("expected milestone tracking endpoint in response")
	}
	if !strings.Contains(body, "timeupdate") {
		t.Error("expected timeupdate event listener in response")
	}

	time.Sleep(100 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
