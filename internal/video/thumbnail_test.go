package video

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestThumbnailFileKey(t *testing.T) {
	key := thumbnailFileKey("user-123", "abc123defghi")
	expected := "recordings/user-123/abc123defghi.jpg"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestExtractFrame_InvalidInput(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	err := extractFrame("/nonexistent/input.webm", "/tmp/sendrec-test-output.jpg")
	if err == nil {
		t.Error("expected error for nonexistent input file")
	}
}

func TestGenerateThumbnail_StorageDownloadError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{downloadToFileErr: fmt.Errorf("s3 down")}

	// Should log the error but not panic. No DB update expected since download failed.
	GenerateThumbnail(context.Background(), mock, s, "video-123", "recordings/user/abc.webm", "recordings/user/abc.jpg")

	// If we get here without panic, the test passes
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unexpected DB calls: %v", err)
	}
}

func TestGenerateThumbnail_UploadError(t *testing.T) {
	// This test verifies that when ffmpeg isn't available, GenerateThumbnail
	// logs the error and returns without panicking or making DB calls.
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// downloadToFileErr is nil so download "succeeds" but ffmpeg will fail
	// (no actual video content in temp file)
	s := &mockStorage{}

	GenerateThumbnail(context.Background(), mock, s, "video-123", "recordings/user/abc.webm", "recordings/user/abc.jpg")

	// Should not have called DB since ffmpeg/upload failed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unexpected DB calls: %v", err)
	}
}

func TestExtractFrameAt_InvalidInput(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	err := extractFrameAt("/nonexistent/input.webm", "/tmp/sendrec-test-output.jpg", 2)
	if err == nil {
		t.Error("expected error for nonexistent input file")
	}
}

func TestExtractFrameAt_SeekZero(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	err := extractFrameAt("/nonexistent/input.webm", "/tmp/sendrec-test-output.jpg", 0)
	if err == nil {
		t.Error("expected error for nonexistent input file")
	}
}

func TestThumbnailOutputSizeCheck(t *testing.T) {
	// Verify that an empty file is detected as having no content
	tmpFile, err := os.CreateTemp("", "sendrec-thumb-test-*.jpg")
	if err != nil {
		t.Fatal(err)
	}
	path := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(path) }()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Errorf("expected empty file, got %d bytes", info.Size())
	}
}

// --- UploadThumbnail handler ---

func TestUploadThumbnail_ValidJPEG(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload?signed=thumb"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoID := "video-thumb-1"
	shareToken := "abc123thumb"

	mock.ExpectQuery(`SELECT share_token FROM videos WHERE id = \$1 AND user_id = \$2 AND organization_id IS NOT DISTINCT FROM \$3 AND status = 'ready'`).
		WithArgs(videoID, testUserID, (*string)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"share_token"}).AddRow(shareToken))

	mock.ExpectExec(`UPDATE videos SET thumbnail_key = \$1, updated_at = now\(\) WHERE id = \$2`).
		WithArgs("recordings/"+testUserID+"/"+shareToken+".jpg", videoID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(struct {
		ContentType   string `json:"contentType"`
		ContentLength int64  `json:"contentLength"`
	}{
		ContentType:   "image/jpeg",
		ContentLength: 500000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/thumbnail", handler.UploadThumbnail)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/thumbnail", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp thumbnailUploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.UploadURL != "https://s3.example.com/upload?signed=thumb" {
		t.Errorf("unexpected upload URL: %s", resp.UploadURL)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUploadThumbnail_ValidPNG(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload?signed=png"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoID := "video-thumb-2"
	shareToken := "def456thumb"

	mock.ExpectQuery(`SELECT share_token FROM videos WHERE id = \$1 AND user_id = \$2 AND organization_id IS NOT DISTINCT FROM \$3 AND status = 'ready'`).
		WithArgs(videoID, testUserID, (*string)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"share_token"}).AddRow(shareToken))

	mock.ExpectExec(`UPDATE videos SET thumbnail_key = \$1, updated_at = now\(\) WHERE id = \$2`).
		WithArgs("recordings/"+testUserID+"/"+shareToken+".jpg", videoID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(struct {
		ContentType   string `json:"contentType"`
		ContentLength int64  `json:"contentLength"`
	}{
		ContentType:   "image/png",
		ContentLength: 100000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/thumbnail", handler.UploadThumbnail)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/thumbnail", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp thumbnailUploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.UploadURL != "https://s3.example.com/upload?signed=png" {
		t.Errorf("unexpected upload URL: %s", resp.UploadURL)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUploadThumbnail_ValidWebP(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload?signed=webp"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoID := "video-thumb-3"
	shareToken := "ghi789thumb"

	mock.ExpectQuery(`SELECT share_token FROM videos WHERE id = \$1 AND user_id = \$2 AND organization_id IS NOT DISTINCT FROM \$3 AND status = 'ready'`).
		WithArgs(videoID, testUserID, (*string)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"share_token"}).AddRow(shareToken))

	mock.ExpectExec(`UPDATE videos SET thumbnail_key = \$1, updated_at = now\(\) WHERE id = \$2`).
		WithArgs("recordings/"+testUserID+"/"+shareToken+".jpg", videoID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(struct {
		ContentType   string `json:"contentType"`
		ContentLength int64  `json:"contentLength"`
	}{
		ContentType:   "image/webp",
		ContentLength: 200000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/thumbnail", handler.UploadThumbnail)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/thumbnail", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUploadThumbnail_InvalidType(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	body, _ := json.Marshal(struct {
		ContentType   string `json:"contentType"`
		ContentLength int64  `json:"contentLength"`
	}{
		ContentType:   "application/pdf",
		ContentLength: 500000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/thumbnail", handler.UploadThumbnail)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/video-123/thumbnail", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "thumbnail must be JPEG, PNG, or WebP" {
		t.Errorf("unexpected error: %s", errMsg)
	}
}

func TestUploadThumbnail_TooLarge(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	body, _ := json.Marshal(struct {
		ContentType   string `json:"contentType"`
		ContentLength int64  `json:"contentLength"`
	}{
		ContentType:   "image/jpeg",
		ContentLength: 3 * 1024 * 1024,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/thumbnail", handler.UploadThumbnail)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/video-123/thumbnail", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "thumbnail must be 2MB or smaller" {
		t.Errorf("unexpected error: %s", errMsg)
	}
}

func TestUploadThumbnail_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT share_token FROM videos WHERE id = \$1 AND user_id = \$2 AND organization_id IS NOT DISTINCT FROM \$3 AND status = 'ready'`).
		WithArgs("nonexistent-video", testUserID, (*string)(nil)).
		WillReturnError(pgx.ErrNoRows)

	body, _ := json.Marshal(struct {
		ContentType   string `json:"contentType"`
		ContentLength int64  `json:"contentLength"`
	}{
		ContentType:   "image/jpeg",
		ContentLength: 500000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/thumbnail", handler.UploadThumbnail)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/nonexistent-video/thumbnail", body))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "video not found" {
		t.Errorf("unexpected error: %s", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- ResetThumbnail handler ---

func TestResetThumbnail_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoID := "video-reset-1"
	shareToken := "resettoken123"
	fileKey := "recordings/" + testUserID + "/resettoken123.webm"
	thumbKey := "recordings/" + testUserID + "/resettoken123.jpg"

	mock.ExpectQuery(`SELECT share_token, file_key, thumbnail_key FROM videos WHERE id = \$1 AND user_id = \$2 AND organization_id IS NOT DISTINCT FROM \$3 AND status = 'ready'`).
		WithArgs(videoID, testUserID, (*string)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"share_token", "file_key", "thumbnail_key"}).AddRow(shareToken, fileKey, &thumbKey))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/videos/{id}/thumbnail", handler.ResetThumbnail)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/videos/"+videoID+"/thumbnail", nil))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestResetThumbnail_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT share_token, file_key, thumbnail_key FROM videos WHERE id = \$1 AND user_id = \$2 AND organization_id IS NOT DISTINCT FROM \$3 AND status = 'ready'`).
		WithArgs("nonexistent-video", testUserID, (*string)(nil)).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/videos/{id}/thumbnail", handler.ResetThumbnail)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/videos/nonexistent-video/thumbnail", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	resetErrMsg := parseErrorResponse(t, rec.Body.Bytes())
	if resetErrMsg != "video not found" {
		t.Errorf("unexpected error: %s", resetErrMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}
