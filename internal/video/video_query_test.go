package video

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestGetTranscript_ReturnsSegments(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	segmentsJSON := `[{"start":0.0,"end":2.5,"text":"Hello world"},{"start":2.5,"end":5.0,"text":"Second segment"}]`
	mock.ExpectQuery(`SELECT transcript_status, transcript_json FROM videos WHERE id = \$1 AND user_id = \$2 AND organization_id IS NULL AND status != 'deleted'`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"transcript_status", "transcript_json"}).
			AddRow("ready", &segmentsJSON))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos/{id}/transcript", handler.GetTranscript)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos/"+videoID+"/transcript", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp struct {
		Status   string              `json:"status"`
		Segments []TranscriptSegment `json:"segments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "ready" {
		t.Errorf("expected status 'ready', got %q", resp.Status)
	}
	if len(resp.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(resp.Segments))
	}
	if resp.Segments[0].Text != "Hello world" {
		t.Errorf("expected first segment text 'Hello world', got %q", resp.Segments[0].Text)
	}
	if resp.Segments[1].Start != 2.5 {
		t.Errorf("expected second segment start 2.5, got %f", resp.Segments[1].Start)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestGetTranscript_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	videoID := "nonexistent-id"

	mock.ExpectQuery(`SELECT transcript_status, transcript_json FROM videos WHERE id = \$1 AND user_id = \$2 AND organization_id IS NULL AND status != 'deleted'`).
		WithArgs(videoID, testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos/{id}/transcript", handler.GetTranscript)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos/"+videoID+"/transcript", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestGetTranscript_NullSegments(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	videoID := "video-456"

	mock.ExpectQuery(`SELECT transcript_status, transcript_json FROM videos WHERE id = \$1 AND user_id = \$2 AND organization_id IS NULL AND status != 'deleted'`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"transcript_status", "transcript_json"}).
			AddRow("pending", (*string)(nil)))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos/{id}/transcript", handler.GetTranscript)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos/"+videoID+"/transcript", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp struct {
		Status   string              `json:"status"`
		Segments []TranscriptSegment `json:"segments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "pending" {
		t.Errorf("expected status 'pending', got %q", resp.Status)
	}
	if len(resp.Segments) != 0 {
		t.Errorf("expected 0 segments, got %d", len(resp.Segments))
	}
	if resp.Segments == nil {
		t.Error("expected empty array, got null")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestList_SearchBySpeaker(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	createdAt := time.Date(2026, 2, 5, 10, 30, 0, 0, time.UTC)
	shareExpiresAt := createdAt.Add(7 * 24 * time.Hour)

	// Title deliberately does NOT contain the search term "Alice": this row
	// can only come back if the WHERE clause actually matches on
	// seg->>'speaker', not on title or seg->>'text'. The query regexp also
	// pins the speaker predicate itself so the test fails if it's removed.
	mock.ExpectQuery(`SELECT v\.id, v\.title.*seg->>'speaker' ILIKE \$2`).
		WithArgs(testUserID, "%Alice%", 50, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "title", "status", "duration", "share_token", "created_at", "share_expires_at", "view_count", "unique_view_count", "thumbnail_key", "share_password", "comment_mode", "comment_count", "transcript_status", "view_notification", "download_enabled", "cta_text", "cta_url", "email_gate_enabled", "summary_status", "document_status", "suggested_title", "folder_id", "transcription_language", "noise_reduction", "pinned", "tags_json", "playlists_json"}).
			AddRow("video-1", "Q3 Planning", "ready", 300, "tok123", createdAt, &shareExpiresAt, int64(5), int64(3), (*string)(nil), (*string)(nil), "disabled", int64(0), "ready", (*string)(nil), true, (*string)(nil), (*string)(nil), false, "none", "none", (*string)(nil), (*string)(nil), (*string)(nil), false, false, "[]", "[]"),
		)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos", handler.List)

	req := authenticatedRequest(t, http.MethodGet, "/api/videos?q=Alice", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
