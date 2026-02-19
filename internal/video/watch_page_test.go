package video

import (
	"context"
	"encoding/json"
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

var watchPageColumns = []string{
	"id", "title", "file_key", "name", "created_at", "share_expires_at",
	"thumbnail_key", "share_password", "comment_mode",
	"transcript_key", "transcript_json", "transcript_status",
	"user_id", "email", "view_notification", "content_type",
	"ub_company_name", "ub_logo_key", "ub_color_background", "ub_color_surface", "ub_color_text", "ub_color_accent", "ub_footer_text", "ub_custom_css",
	"vb_company_name", "vb_logo_key", "vb_color_background", "vb_color_surface", "vb_color_text", "vb_color_accent", "vb_footer_text",
	"download_enabled",
}

func watchPageRequest(shareToken string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	ctx := httputil.ContextWithNonce(req.Context(), "test-nonce")
	return req.WithContext(ctx)
}

func serveWatchPage(handler *Handler, req *http.Request) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.WatchPage)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestWatchPage_NotFound_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testHMACSecret, false)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs("nonexistent").
		WillReturnError(errors.New("no rows"))

	rec := serveWatchPage(handler, watchPageRequest("nonexistent"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Video not found") {
		t.Error("expected 'Video not found' in response")
	}
	if !strings.Contains(body, "text/html") {
		ct := rec.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			t.Errorf("expected text/html content type, got %s", ct)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_Expired_Returns410(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{downloadURL: "https://s3.example.com/video"}, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "expiredtoken1"
	createdAt := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	expiredAt := time.Now().Add(-24 * time.Hour) // expired yesterday

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Old Video", "recordings/u1/old.webm", "Alice", createdAt, &expiredAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

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

func TestWatchPage_Success_RendersVideoPlayer(t *testing.T) {
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
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "My Demo", "recordings/u1/abc.webm", "Bob Smith", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	checks := map[string]string{
		"video element":   "<video",
		"video source":    `src="https://s3.example.com/video.webm"`,
		"title":           "My Demo",
		"creator":         "Bob Smith",
		"date":            "Feb 5, 2026",
		"download button": `id="download-btn"`,
		"branding":        "SendRec",
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

func TestWatchPage_Success_RendersSpeedButtons(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "speedtoken12"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Speed Test", "recordings/u1/speed.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	speeds := []string{
		`data-speed="0.5"`,
		`data-speed="1"`,
		`data-speed="1.5"`,
		`data-speed="2"`,
	}
	for _, s := range speeds {
		if !strings.Contains(body, s) {
			t.Errorf("expected speed button %q in response", s)
		}
	}
	if !strings.Contains(body, "speed-controls") {
		t.Error("expected speed-controls container in response")
	}
	// 1x should be active by default
	if !strings.Contains(body, `class="speed-btn active" data-speed="1"`) {
		t.Error("expected 1x speed button to have active class")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_WithThumbnail_RendersPosterAndOGImage(t *testing.T) {
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
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Thumb Video", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			&thumbKey, (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "poster=") {
		t.Error("expected poster attribute on video element")
	}
	if !strings.Contains(body, `og:image`) {
		t.Error("expected og:image meta tag")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_WithoutThumbnail_NoPosterOrOGImage(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "nothumbtoken"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "No Thumb", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if strings.Contains(body, "poster=") {
		t.Error("expected no poster attribute without thumbnail")
	}
	if strings.Contains(body, `og:image`) {
		t.Error("expected no og:image meta tag without thumbnail")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_CommentsEnabled_RendersCommentForm(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "commtoken123"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Comments Video", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "anonymous",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	checks := []string{
		"comments-section",
		"comment-form",
		"comment-submit",
		"comment-body",
		"Post comment",
		"markers-bar",
		"emoji-trigger",
	}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("expected %q in response when comments enabled", check)
		}
	}
	if strings.Contains(body, "if (!markersBar || !videoDuration) return;") {
		t.Error("expected markers rendering to handle videos with unknown duration instead of hard-stopping")
	}
	if !strings.Contains(body, "body: JSON.stringify({authorName: '', authorEmail: '', body: emoji, isPrivate: false, videoTimestamp: timestamp})") {
		t.Error("expected reaction payload to use anonymous author name/email")
	}
	if !strings.Contains(body, `id="reaction-error"`) {
		t.Error("expected reaction error feedback element")
	}
	if !strings.Contains(body, `var reactionEmojis = ["👍","👎","❤️","😂","😮","🎉"];`) {
		t.Error("expected reaction emojis to be sourced from backend-generated JSON list")
	}
	if !strings.Contains(body, `class="reaction-btn"`) || !strings.Contains(body, `aria-label="React with`) {
		t.Error("expected reaction buttons to include accessibility labels")
	}
	if !strings.Contains(body, `dot.type = 'button';`) {
		t.Error("expected marker dots to be keyboard-accessible buttons")
	}
	if !strings.Contains(body, `type="button" class="comment emoji-reaction"`) {
		t.Error("expected emoji reactions to render as button elements for keyboard access")
	}
	if !strings.Contains(body, `reactionErrorEl.textContent =`) {
		t.Error("expected reaction errors to be surfaced to users")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_CommentsDisabled_NoCommentForm(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "nocommtoken1"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "No Comments", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if strings.Contains(body, `id="comments-section"`) {
		t.Error("expected no comments-section div when comments disabled")
	}
	if strings.Contains(body, `id="comment-form"`) {
		t.Error("expected no comment-form div when comments disabled")
	}
	if strings.Contains(body, `id="markers-bar"`) {
		t.Error("expected no markers-bar div when comments disabled")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_NameRequired_RendersNameField(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "namereqtoken"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Name Required", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "name_required",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, `id="comment-name"`) {
		t.Error("expected name input field")
	}
	if strings.Contains(body, `id="comment-email"`) {
		t.Error("expected no email input for name_required mode")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_NameEmailRequired_RendersBothFields(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "emailreqtokn"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Email Required", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "name_email_required",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, `id="comment-name"`) {
		t.Error("expected name input field")
	}
	if !strings.Contains(body, `id="comment-email"`) {
		t.Error("expected email input field")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_TranscriptReady_RendersSegments(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "transreadytk"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	transcriptKey := "transcripts/u1/abc.vtt"
	segments := []TranscriptSegment{
		{Start: 0, End: 5.5, Text: "Hello world"},
		{Start: 5.5, End: 12, Text: "This is a test"},
	}
	segJSON, _ := json.Marshal(segments)
	segStr := string(segJSON)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Transcribed", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			&transcriptKey, &segStr, "ready",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "transcript-panel") {
		t.Error("expected transcript-panel element")
	}
	if !strings.Contains(body, "Hello world") {
		t.Error("expected transcript segment text 'Hello world'")
	}
	if !strings.Contains(body, "This is a test") {
		t.Error("expected transcript segment text 'This is a test'")
	}
	if !strings.Contains(body, "transcript-segment") {
		t.Error("expected transcript-segment elements")
	}
	if !strings.Contains(body, "transcript-timestamp") {
		t.Error("expected transcript-timestamp elements")
	}
	// Verify VTT track element is rendered
	if !strings.Contains(body, `<track kind="subtitles"`) {
		t.Error("expected <track> element for subtitles")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_TranscriptPending_ShowsQueueMessage(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "transpendtok"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Pending Trans", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "pending",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "Transcription queued") {
		t.Error("expected 'Transcription queued' message")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_TranscriptProcessing_ShowsProgressMessage(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "transproctok"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Processing Trans", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "processing",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "Transcription in progress") {
		t.Error("expected 'Transcription in progress' message")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_TranscriptFailed_ShowsFailedMessage(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "transfailtok"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Failed Trans", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "failed",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "Transcription failed") {
		t.Error("expected 'Transcription failed' message")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_StorageError_Returns500(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadErr: errors.New("storage unavailable")}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "storageerrtk"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Storage Error", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_PasswordProtected_NoCookie_ShowsPasswordForm(t *testing.T) {
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
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Protected Video", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), &passwordHash, "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "password protected") {
		t.Error("expected password protected message")
	}
	if !strings.Contains(body, "password-form") {
		t.Error("expected password form")
	}
	if strings.Contains(body, "<video") {
		t.Error("expected no video player on password page")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_PasswordProtected_ValidCookie_ShowsPlayer(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	passwordHash, _ := hashSharePassword("secret123")
	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "pwdvalid1234"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Protected OK", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), &passwordHash, "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	sig := signWatchCookie(testHMACSecret, shareToken, passwordHash)
	req := watchPageRequest(shareToken)
	req.AddCookie(&http.Cookie{
		Name:  watchCookieName(shareToken),
		Value: sig,
	})

	rec := serveWatchPage(handler, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if strings.Contains(body, "password protected") {
		t.Error("expected no password page when valid cookie present")
	}
	if !strings.Contains(body, "<video") {
		t.Error("expected video player when valid cookie present")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_OGMetaTags(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "ogmetatoken1"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "OG Tags Test", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	checks := []string{
		`og:title`,
		`og:type`,
		`og:video`,
		`og:site_name`,
	}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("expected %q in OG meta tags", check)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_CrossOriginAttribute(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "corstoken123"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "CORS Test", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, `crossorigin="anonymous"`) {
		t.Error("expected crossorigin=anonymous on video element for CORS subtitle loading")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_PlaysInlineAttribute(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "inlinetoken1"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Inline Test", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "playsinline") {
		t.Error("expected playsinline attribute on video element for iOS Safari")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_CSPNonce(t *testing.T) {
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
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Nonce Test", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, `nonce="test-nonce"`) {
		t.Error("expected CSP nonce in style and script tags")
	}
	// Should have nonce on both style and script tags
	nonceCount := strings.Count(body, `nonce="test-nonce"`)
	if nonceCount < 2 {
		t.Errorf("expected nonce on multiple tags (style + scripts), found %d occurrences", nonceCount)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_TitleInHTMLTitle(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "titletoken12"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "My Special Recording", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "<title>My Special Recording") {
		t.Error("expected video title in HTML <title> tag")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_AutoplayScript(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "autoplaytkn1"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Autoplay", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	// Should have autoplay with muted fallback
	if !strings.Contains(body, "v.play()") {
		t.Error("expected autoplay script")
	}
	if !strings.Contains(body, "v.muted = true") {
		t.Error("expected muted fallback in autoplay script")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_BrandingLogoLinksToSendrec(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "brandingtkn1"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Branding Test", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, `class="logo"`) {
		t.Error("expected logo element on watch page")
	}
	if !strings.Contains(body, `href="`+testBaseURL+`"`) {
		t.Error("expected logo to link to app base URL")
	}
	if !strings.Contains(body, `src="/images/logo.png"`) {
		t.Error("expected logo to use /images/logo.png, not a data URI")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestFormatTimestamp_TemplateFuncMap(t *testing.T) {
	fn := watchFuncs["formatTimestamp"].(func(float64) string)

	tests := []struct {
		input float64
		want  string
	}{
		{0, "0:00"},
		{5, "0:05"},
		{65, "1:05"},
		{3600, "60:00"},
		{125.7, "2:05"},
	}
	for _, tt := range tests {
		got := fn(tt.input)
		if got != tt.want {
			t.Errorf("formatTimestamp(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWatchPage_RecordsView(t *testing.T) {
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
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "My Demo", "recordings/u1/abc.webm", "Bob Smith", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	mock.ExpectExec(`INSERT INTO video_views`).
		WithArgs("vid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Give the goroutine time to execute
	time.Sleep(50 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_NotFound_HasNonceInTemplate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testHMACSecret, false)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs("missing12345").
		WillReturnError(errors.New("no rows"))

	// Use a nonce-aware request
	req := httptest.NewRequest(http.MethodGet, "/watch/missing12345", nil)
	ctx := httputil.ContextWithNonce(context.Background(), "custom-nonce")
	req = req.WithContext(ctx)

	rec := serveWatchPage(handler, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `nonce="custom-nonce"`) {
		t.Error("expected nonce in not-found page template")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestInjectScriptNonce(t *testing.T) {
	tests := []struct {
		name   string
		script string
		nonce  string
		want   string
	}{
		{"empty script", "", "abc", ""},
		{"umami", `<script defer src="/script.js" data-website-id="xxx"></script>`, "n123",
			`<script nonce="n123" defer src="/script.js" data-website-id="xxx"></script>`},
		{"plausible", `<script defer data-domain="example.com" src="https://plausible.io/js/script.js"></script>`, "n456",
			`<script nonce="n456" defer data-domain="example.com" src="https://plausible.io/js/script.js"></script>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(injectScriptNonce(tt.script, tt.nonce))
			if got != tt.want {
				t.Errorf("injectScriptNonce(%q, %q) = %q, want %q", tt.script, tt.nonce, got, tt.want)
			}
		})
	}
}

func TestWatchPage_AnalyticsScriptRendered(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	handler.SetAnalyticsScript(`<script defer src="/script.js" data-website-id="test-id"></script>`)

	shareToken := "analytics123"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Analytics Test", "recordings/u1/abc.webm", "Bob", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	mock.ExpectExec(`INSERT INTO video_views`).
		WithArgs("vid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, `nonce="test-nonce"`) {
		t.Error("expected nonce in analytics script")
	}
	if !strings.Contains(body, `data-website-id="test-id"`) {
		t.Error("expected analytics script with data-website-id")
	}

	time.Sleep(50 * time.Millisecond)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_NoAnalyticsWhenEmpty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)

	shareToken := "noanalytics1"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "No Analytics", "recordings/u1/abc.webm", "Bob", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	mock.ExpectExec(`INSERT INTO video_views`).
		WithArgs("vid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if strings.Contains(body, "data-website-id") {
		t.Error("expected no analytics script when not configured")
	}

	time.Sleep(50 * time.Millisecond)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_ResponsiveCSS(t *testing.T) {
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
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Responsive", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "@media") {
		t.Error("expected responsive media query in watch page CSS")
	}
	if !strings.Contains(body, "max-width: 640px") {
		t.Error("expected mobile breakpoint at 640px")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_SafariWebMWarningElement(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "safariwebm1"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Safari Test", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "safari-webm-warning") {
		t.Error("expected Safari WebM warning element in watch page")
	}
	if !strings.Contains(body, "browser-warning") {
		t.Error("expected browser-warning CSS class")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_DownloadEnabled_ShowsButton(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "dlenabledtk1"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Download On", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, `id="download-btn"`) {
		t.Error("expected download button element in response when download is enabled")
	}
	if strings.Contains(body, `controlsList="nodownload"`) {
		t.Error("expected no controlsList=nodownload when download is enabled")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_DownloadDisabled_HidesButton(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/video.webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	shareToken := "dldisabledtk"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "Download Off", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			false,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if strings.Contains(body, `id="download-btn"`) {
		t.Error("expected no download button element in response when download is disabled")
	}
	if !strings.Contains(body, `controlsList="nodownload"`) {
		t.Error("expected controlsList=nodownload on video element when download is disabled")
	}
	if !strings.Contains(body, `oncontextmenu="return false;"`) {
		t.Error("expected context menu disabled on video element when download is disabled")
	}
	if strings.Contains(body, `og:video`) {
		t.Error("expected no og:video meta tag when download is disabled")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_CustomCSS_Injected(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/download"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)
	handler.brandingEnabled = true

	shareToken := "custom-css-token"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	customCSS := "body { font-family: 'Inter', sans-serif; }"

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "CSS Test", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), &customCSS,
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, customCSS) {
		t.Error("expected custom CSS to be injected in the page")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_NoCustomCSS(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/download"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testHMACSecret, false)

	shareToken := "no-css-token"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.file_key`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows(watchPageColumns).AddRow(
			"vid-1", "No CSS Test", "recordings/u1/abc.webm", "Alice", createdAt, &expiresAt,
			(*string)(nil), (*string)(nil), "disabled",
			(*string)(nil), (*string)(nil), "none",
			"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
			true,
		))

	rec := serveWatchPage(handler, watchPageRequest(shareToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if strings.Contains(body, "font-family: 'Inter'") {
		t.Error("expected no custom CSS when NULL")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatchPage_NeverExpires(t *testing.T) {
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
			pgxmock.NewRows(watchPageColumns).AddRow(
				"vid-1", "Never Expire Video", "recordings/u1/abc.webm", "Bob", createdAt, (*time.Time)(nil),
				(*string)(nil), (*string)(nil), "disabled",
				(*string)(nil), (*string)(nil), "none",
				"owner-user-id", "owner@example.com", (*string)(nil), "video/webm",
				(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
				(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
				true,
			),
		)

	mock.ExpectExec("INSERT INTO video_views").
		WithArgs("vid-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	req := watchPageRequest("token-never")
	rec := serveWatchPage(handler, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Never Expire Video") {
		t.Error("expected video title in response")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
