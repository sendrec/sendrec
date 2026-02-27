package video

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func expectDashboardQueries(t *testing.T, mock pgxmock.PgxPoolIface, totalViews, uniqueViews, totalVideos, watchTime int64, avgCompletion float64, dailyRows *pgxmock.Rows, topVideoRows *pgxmock.Rows) {
	t.Helper()

	mock.ExpectQuery(`SELECT COUNT\(\*\) AS views, COUNT\(DISTINCT viewer_hash\)`).
		WithArgs(testUserID, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"views", "unique_views"}).AddRow(totalViews, uniqueViews))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM videos WHERE user_id`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(totalVideos))

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(v.duration\), 0\)`).
		WithArgs(testUserID, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"total_watch_time"}).AddRow(watchTime))

	mock.ExpectQuery(`SELECT COALESCE\(AVG`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"avg_completion"}).AddRow(avgCompletion))

	mock.ExpectQuery(`date_trunc\('day', vv.created_at\)::date AS day`).
		WithArgs(testUserID, pgxmock.AnyArg()).
		WillReturnRows(dailyRows)

	mock.ExpectQuery(`v\.share_token`).
		WithArgs(testUserID, pgxmock.AnyArg()).
		WillReturnRows(topVideoRows)
}

func TestAnalyticsDashboard_Returns7DayStats(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	now := time.Now().UTC().Truncate(24 * time.Hour)
	dailyRows := pgxmock.NewRows([]string{"day", "views", "unique_views"})
	for i := 6; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		dailyRows.AddRow(d, int64(10+i), int64(5+i))
	}

	topVideoRows := pgxmock.NewRows([]string{"id", "title", "views", "unique_views", "share_token", "has_thumbnail", "completion"})

	expectDashboardQueries(t, mock, 100, 50, 5, 3600, 75.0, dailyRows, topVideoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/analytics/dashboard", handler.AnalyticsDashboard)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/analytics/dashboard?range=7d", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp dashboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Summary.TotalViews != 100 {
		t.Errorf("expected TotalViews 100, got %d", resp.Summary.TotalViews)
	}
	if resp.Summary.UniqueViews != 50 {
		t.Errorf("expected UniqueViews 50, got %d", resp.Summary.UniqueViews)
	}
	if resp.Summary.TotalVideos != 5 {
		t.Errorf("expected TotalVideos 5, got %d", resp.Summary.TotalVideos)
	}
	if resp.Summary.TotalWatchTimeSeconds != 3600 {
		t.Errorf("expected TotalWatchTimeSeconds 3600, got %d", resp.Summary.TotalWatchTimeSeconds)
	}
	if resp.Summary.AvgCompletion != 75.0 {
		t.Errorf("expected AvgCompletion 75.0, got %f", resp.Summary.AvgCompletion)
	}
	if len(resp.Daily) != 7 {
		t.Errorf("expected 7 daily entries, got %d", len(resp.Daily))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestAnalyticsDashboard_DefaultsTo7d(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	dailyRows := pgxmock.NewRows([]string{"day", "views", "unique_views"})
	topVideoRows := pgxmock.NewRows([]string{"id", "title", "views", "unique_views", "share_token", "has_thumbnail", "completion"})

	expectDashboardQueries(t, mock, 0, 0, 0, 0, 0.0, dailyRows, topVideoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/analytics/dashboard", handler.AnalyticsDashboard)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/analytics/dashboard", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp dashboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Daily) != 7 {
		t.Errorf("expected 7 daily entries (default range), got %d", len(resp.Daily))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestAnalyticsDashboard_InvalidRange(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/analytics/dashboard", handler.AnalyticsDashboard)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/analytics/dashboard?range=invalid", nil))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if !strings.Contains(errMsg, "invalid range") {
		t.Errorf("expected error to contain 'invalid range', got %q", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestAnalyticsDashboard_WithTopVideos(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	dailyRows := pgxmock.NewRows([]string{"day", "views", "unique_views"})
	topVideoRows := pgxmock.NewRows([]string{"id", "title", "views", "unique_views", "share_token", "has_thumbnail", "completion"}).
		AddRow("vid-1", "My Video", int64(42), int64(20), "abc123token", true, 85).
		AddRow("vid-2", "No Thumb", int64(10), int64(5), "def456token", false, 50)

	expectDashboardQueries(t, mock, 52, 25, 2, 1200, 67.5, dailyRows, topVideoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/analytics/dashboard", handler.AnalyticsDashboard)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/analytics/dashboard?range=7d", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp dashboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.TopVideos) != 2 {
		t.Fatalf("expected 2 top videos, got %d", len(resp.TopVideos))
	}

	first := resp.TopVideos[0]
	if first.ID != "vid-1" {
		t.Errorf("expected first video ID 'vid-1', got %q", first.ID)
	}
	if first.Title != "My Video" {
		t.Errorf("expected first video title 'My Video', got %q", first.Title)
	}
	if first.Views != 42 {
		t.Errorf("expected first video views 42, got %d", first.Views)
	}
	if first.UniqueViews != 20 {
		t.Errorf("expected first video unique views 20, got %d", first.UniqueViews)
	}
	if first.ThumbnailURL != "/api/watch/abc123token/thumbnail" {
		t.Errorf("expected thumbnailURL '/api/watch/abc123token/thumbnail', got %q", first.ThumbnailURL)
	}
	if first.Completion != 85 {
		t.Errorf("expected first video completion 85, got %d", first.Completion)
	}

	second := resp.TopVideos[1]
	if second.ThumbnailURL != "" {
		t.Errorf("expected empty thumbnailURL for video without thumbnail, got %q", second.ThumbnailURL)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestAnalyticsDashboard_30DayRange(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	dailyRows := pgxmock.NewRows([]string{"day", "views", "unique_views"})
	topVideoRows := pgxmock.NewRows([]string{"id", "title", "views", "unique_views", "share_token", "has_thumbnail", "completion"})

	expectDashboardQueries(t, mock, 0, 0, 0, 0, 0.0, dailyRows, topVideoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/analytics/dashboard", handler.AnalyticsDashboard)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/analytics/dashboard?range=30d", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp dashboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Daily) != 30 {
		t.Errorf("expected 30 daily entries, got %d", len(resp.Daily))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestDashboardExport_ReturnsCSV(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	exportDay := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`date_trunc\('day', vv.created_at\)::date AS day`).
		WithArgs(testUserID, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"day", "views", "unique_views"}).
			AddRow(exportDay, int64(25), int64(12)))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/analytics/dashboard/export", handler.DashboardExport)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/analytics/dashboard/export?range=7d", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/csv" {
		t.Errorf("expected Content-Type 'text/csv', got %q", contentType)
	}

	disposition := rec.Header().Get("Content-Disposition")
	if disposition != "attachment; filename=analytics-dashboard.csv" {
		t.Errorf("expected Content-Disposition 'attachment; filename=analytics-dashboard.csv', got %q", disposition)
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

func TestDashboardExport_InvalidRange(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/analytics/dashboard/export", handler.DashboardExport)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/analytics/dashboard/export?range=bad", nil))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if !strings.Contains(errMsg, "invalid range") {
		t.Errorf("expected error to contain 'invalid range', got %q", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestSortDashboardDaily(t *testing.T) {
	daily := []dashboardDaily{
		{Date: "2026-02-15", Views: 10, UniqueViews: 5},
		{Date: "2026-02-10", Views: 20, UniqueViews: 8},
		{Date: "2026-02-20", Views: 5, UniqueViews: 2},
		{Date: "2026-02-12", Views: 15, UniqueViews: 7},
	}

	sortDashboardDaily(daily)

	expected := []string{"2026-02-10", "2026-02-12", "2026-02-15", "2026-02-20"}
	for i, want := range expected {
		if daily[i].Date != want {
			t.Errorf("daily[%d].Date = %q, want %q", i, daily[i].Date, want)
		}
	}
}
