package video

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestRecordSegments_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT id FROM videos WHERE share_token = \$1 AND status IN`).
		WithArgs("abc123").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("vid-1"))

	mock.ExpectExec(`INSERT INTO segment_engagement`).
		WithArgs("vid-1", 0).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectExec(`INSERT INTO segment_engagement`).
		WithArgs("vid-1", 5).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectExec(`INSERT INTO segment_engagement`).
		WithArgs("vid-1", 49).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/segments", handler.RecordSegments)

	body := []byte(`{"segments":[0,5,49]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/watch/abc123/segments", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	time.Sleep(100 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestRecordSegments_EmptySegments(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/segments", handler.RecordSegments)

	body := []byte(`{"segments":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/watch/abc123/segments", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestRecordSegments_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT id FROM videos WHERE share_token = \$1 AND status IN`).
		WithArgs("notfound").
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/segments", handler.RecordSegments)

	body := []byte(`{"segments":[1,2,3]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/watch/notfound/segments", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if !strings.Contains(errMsg, "video not found") {
		t.Errorf("expected error to contain 'video not found', got %q", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestRecordSegments_InvalidBody(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/segments", handler.RecordSegments)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/abc123/segments", strings.NewReader("not-json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if !strings.Contains(errMsg, "invalid request body") {
		t.Errorf("expected error to contain 'invalid request body', got %q", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestAnalyticsExport_ReturnsCSV(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT id FROM videos WHERE id = \$1 AND user_id = \$2 AND status != 'deleted'`).
		WithArgs("vid-1", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("vid-1"))

	exportDay := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`date_trunc\('day', created_at\)::date AS day`).
		WithArgs("vid-1", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"day", "views", "unique_views"}).
			AddRow(exportDay, int64(25), int64(12)))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos/{id}/analytics/export", handler.AnalyticsExport)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos/vid-1/analytics/export?range=7d", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/csv" {
		t.Errorf("expected Content-Type 'text/csv', got %q", contentType)
	}

	disposition := rec.Header().Get("Content-Disposition")
	if disposition != "attachment; filename=analytics.csv" {
		t.Errorf("expected Content-Disposition 'attachment; filename=analytics.csv', got %q", disposition)
	}

	body := rec.Body.String()
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 CSV lines (header + data), got %d", len(lines))
	}
	if lines[0] != "Date,Views,Unique Views" {
		t.Errorf("expected CSV header 'Date,Views,Unique Views', got %q", lines[0])
	}
	if lines[1] != "2026-02-15,25,12" {
		t.Errorf("expected CSV row '2026-02-15,25,12', got %q", lines[1])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestAnalyticsExport_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT id FROM videos WHERE id = \$1 AND user_id = \$2 AND status != 'deleted'`).
		WithArgs("vid-nonexistent", testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos/{id}/analytics/export", handler.AnalyticsExport)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos/vid-nonexistent/analytics/export?range=7d", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if !strings.Contains(errMsg, "video not found") {
		t.Errorf("expected error to contain 'video not found', got %q", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}
