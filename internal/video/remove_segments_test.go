package video

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestRemoveSegments_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	mock.ExpectQuery(`SELECT duration, file_key, share_token, status, content_type FROM videos WHERE id = \$1 AND user_id = \$2`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"duration", "file_key", "share_token", "status", "content_type"}).
			AddRow(120, "recordings/user/video.webm", "abc123defghi", "ready", "video/webm"))

	mock.ExpectExec(`UPDATE videos SET status = 'processing'`).
		WithArgs(videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/remove-segments", handler.RemoveSegments)

	body := []byte(`{"segments": [{"start": 5.0, "end": 10.0}, {"start": 30.0, "end": 40.0}]}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/remove-segments", body))

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d: %s", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestRemoveSegments_EmptySegments(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/remove-segments", handler.RemoveSegments)

	body := []byte(`{"segments": []}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/video-123/remove-segments", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestRemoveSegments_OverlappingSegments(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/remove-segments", handler.RemoveSegments)

	body := []byte(`{"segments": [{"start": 5.0, "end": 15.0}, {"start": 10.0, "end": 20.0}]}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/video-123/remove-segments", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestRemoveSegments_SegmentsExceedDuration(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	mock.ExpectQuery(`SELECT duration, file_key, share_token, status, content_type FROM videos WHERE id = \$1 AND user_id = \$2`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"duration", "file_key", "share_token", "status", "content_type"}).
			AddRow(60, "recordings/user/video.webm", "abc123defghi", "ready", "video/webm"))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/remove-segments", handler.RemoveSegments)

	body := []byte(`{"segments": [{"start": 5.0, "end": 90.0}]}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/remove-segments", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestRemoveSegments_TooManySegments(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/remove-segments", handler.RemoveSegments)

	// Build a JSON body with 201 segments
	segments := `[`
	for i := range 201 {
		if i > 0 {
			segments += ","
		}
		start := float64(i) * 2.0
		end := start + 1.0
		segments += fmt.Sprintf(`{"start": %.1f, "end": %.1f}`, start, end)
	}
	segments += `]`
	body := []byte(`{"segments": ` + segments + `}`)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/video-123/remove-segments", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestRemoveSegments_ResultDurationTooShort(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	mock.ExpectQuery(`SELECT duration, file_key, share_token, status, content_type FROM videos WHERE id = \$1 AND user_id = \$2`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"duration", "file_key", "share_token", "status", "content_type"}).
			AddRow(10, "recordings/user/video.webm", "abc123defghi", "ready", "video/webm"))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/remove-segments", handler.RemoveSegments)

	body := []byte(`{"segments": [{"start": 0.0, "end": 9.5}]}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/remove-segments", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestRemoveSegments_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	videoID := "nonexistent-id"

	mock.ExpectQuery(`SELECT duration, file_key, share_token, status, content_type FROM videos WHERE id = \$1 AND user_id = \$2`).
		WithArgs(videoID, testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/remove-segments", handler.RemoveSegments)

	body := []byte(`{"segments": [{"start": 5.0, "end": 10.0}]}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/remove-segments", body))

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestRemoveSegments_VideoNotReady(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	mock.ExpectQuery(`SELECT duration, file_key, share_token, status, content_type FROM videos WHERE id = \$1 AND user_id = \$2`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"duration", "file_key", "share_token", "status", "content_type"}).
			AddRow(120, "recordings/user/video.webm", "abc123defghi", "processing", "video/webm"))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/remove-segments", handler.RemoveSegments)

	body := []byte(`{"segments": [{"start": 5.0, "end": 10.0}]}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/remove-segments", body))

	if rec.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestRemoveSegments_ConcurrentProcessing(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	mock.ExpectQuery(`SELECT duration, file_key, share_token, status, content_type FROM videos WHERE id = \$1 AND user_id = \$2`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"duration", "file_key", "share_token", "status", "content_type"}).
			AddRow(120, "recordings/user/video.webm", "abc123defghi", "ready", "video/webm"))

	mock.ExpectExec(`UPDATE videos SET status = 'processing'`).
		WithArgs(videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/remove-segments", handler.RemoveSegments)

	body := []byte(`{"segments": [{"start": 5.0, "end": 10.0}]}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/remove-segments", body))

	if rec.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestRemoveSegments_UnsortedSegments(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/remove-segments", handler.RemoveSegments)

	body := []byte(`{"segments": [{"start": 30.0, "end": 40.0}, {"start": 5.0, "end": 10.0}]}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/video-123/remove-segments", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestRemoveSegments_NegativeStart(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/remove-segments", handler.RemoveSegments)

	body := []byte(`{"segments": [{"start": -5.0, "end": 10.0}]}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/video-123/remove-segments", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestRemoveSegments_EndLessThanOrEqualStart(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/remove-segments", handler.RemoveSegments)

	body := []byte(`{"segments": [{"start": 10.0, "end": 10.0}]}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/video-123/remove-segments", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}
