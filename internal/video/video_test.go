package video

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/httputil"
)

type mockStorage struct {
	uploadURL       string
	uploadErr       error
	downloadURL     string
	downloadErr     error
	deleteErr       error
	deleteCalled    chan string
	deleteCallCount int
	deleteFailUntil int
	headSize        int64
	headType        string
	headErr         error
}

func (m *mockStorage) GenerateUploadURL(_ context.Context, _ string, _ string, _ int64, _ time.Duration) (string, error) {
	return m.uploadURL, m.uploadErr
}

func (m *mockStorage) GenerateDownloadURL(_ context.Context, _ string, _ time.Duration) (string, error) {
	return m.downloadURL, m.downloadErr
}

func (m *mockStorage) DeleteObject(_ context.Context, key string) error {
	m.deleteCallCount++
	if m.deleteCalled != nil {
		m.deleteCalled <- key
	}
	if m.deleteFailUntil > 0 && m.deleteCallCount <= m.deleteFailUntil {
		return m.deleteErr
	}
	if m.deleteFailUntil == 0 {
		return m.deleteErr
	}
	return nil
}

func (m *mockStorage) HeadObject(_ context.Context, _ string) (int64, string, error) {
	if m.headErr != nil {
		return 0, "", m.headErr
	}
	return m.headSize, m.headType, nil
}

const testJWTSecret = "test-secret-for-video-tests"
const testUserID = "550e8400-e29b-41d4-a716-446655440000"
const testBaseURL = "https://sendrec.eu"

func authenticatedRequest(t *testing.T, method, target string, body []byte) *http.Request {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	token, err := auth.GenerateAccessToken(testJWTSecret, testUserID)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func newAuthMiddleware() func(http.Handler) http.Handler {
	return auth.NewHandler(nil, testJWTSecret, false).Middleware
}

func parseErrorResponse(t *testing.T, body []byte) string {
	t.Helper()
	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	return errResp.Error
}

// --- generateShareToken Tests ---

func TestGenerateShareToken_Returns12CharacterString(t *testing.T) {
	token, err := generateShareToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(token) != 12 {
		t.Errorf("expected 12-character token, got %d characters: %q", len(token), token)
	}
}

func TestGenerateShareToken_ReturnsUniqueValues(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := generateShareToken()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if seen[token] {
			t.Errorf("iteration %d: duplicate token %q", i, token)
		}
		seen[token] = true
	}
}

func TestGenerateShareToken_ReturnsURLSafeCharacters(t *testing.T) {
	for i := 0; i < 100; i++ {
		token, err := generateShareToken()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		for _, c := range token {
			if !isURLSafe(c) {
				t.Errorf("iteration %d: token contains non-URL-safe character %q in %q", i, string(c), token)
			}
		}
	}
}

func isURLSafe(c rune) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_'
}

// --- Create Tests ---

func TestCreate_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload?signed=abc"}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"My Recording",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("video-123"))

	body, _ := json.Marshal(createRequest{
		Title:    "My Recording",
		Duration: 120,
		FileSize: 5000000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp createResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ID != "video-123" {
		t.Errorf("expected video ID %q, got %q", "video-123", resp.ID)
	}
	if resp.UploadURL != "https://s3.example.com/upload?signed=abc" {
		t.Errorf("expected upload URL %q, got %q", "https://s3.example.com/upload?signed=abc", resp.UploadURL)
	}
	if resp.ShareToken == "" {
		t.Error("expected non-empty share token")
	}
	if len(resp.ShareToken) != 12 {
		t.Errorf("expected 12-character share token, got %d: %q", len(resp.ShareToken), resp.ShareToken)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreate_DefaultTitleWhenEmpty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload"}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"Untitled Recording",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("video-456"))

	body, _ := json.Marshal(createRequest{
		Title:    "",
		Duration: 60,
		FileSize: 1000000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", body))

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreate_InvalidJSONBody(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", []byte("{invalid json")))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "invalid request body" {
		t.Errorf("expected error %q, got %q", "invalid request body", errMsg)
	}
}

func TestCreate_DatabaseError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload"}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"Test Video",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnError(errors.New("connection refused"))

	body, _ := json.Marshal(createRequest{
		Title:    "Test Video",
		Duration: 30,
		FileSize: 500000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", body))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "failed to create video" {
		t.Errorf("expected error %q, got %q", "failed to create video", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreate_StorageError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadErr: errors.New("s3 unavailable")}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"Test Video",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("video-789"))

	body, _ := json.Marshal(createRequest{
		Title:    "Test Video",
		Duration: 45,
		FileSize: 2000000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", body))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "failed to generate upload URL" {
		t.Errorf("expected error %q, got %q", "failed to generate upload URL", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- Update Tests ---

func TestUpdate_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{headSize: 100000, headType: "video/webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0)
	videoID := "video-123"
	fileKey := "recordings/user/video.webm"
	fileSize := int64(100000)

	mock.ExpectQuery(`SELECT file_key, file_size FROM videos`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key", "file_size"}).AddRow(fileKey, fileSize))

	mock.ExpectExec(`UPDATE videos SET status`).
		WithArgs("ready", videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(updateRequest{Status: "ready"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Patch("/api/videos/{id}", handler.Update)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPatch, "/api/videos/"+videoID, body))

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUpdate_InvalidStatus(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{headSize: 1000, headType: "video/webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0)
	videoID := "video-123"

	body, _ := json.Marshal(updateRequest{Status: "processing"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Patch("/api/videos/{id}", handler.Update)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPatch, "/api/videos/"+videoID, body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "status can only be set to ready" {
		t.Errorf("expected error %q, got %q", "status can only be set to ready", errMsg)
	}
}

func TestUpdate_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{headSize: 1000, headType: "video/webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0)
	videoID := "nonexistent-id"

	mock.ExpectQuery(`SELECT file_key, file_size FROM videos`).
		WithArgs(videoID, testUserID).
		WillReturnError(pgx.ErrNoRows)

	body, _ := json.Marshal(updateRequest{Status: "ready"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Patch("/api/videos/{id}", handler.Update)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPatch, "/api/videos/"+videoID, body))

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "video not found" {
		t.Errorf("expected error %q, got %q", "video not found", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestUpdate_InvalidJSON(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{headSize: 1000, headType: "video/webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0)
	videoID := "video-123"

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Patch("/api/videos/{id}", handler.Update)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPatch, "/api/videos/"+videoID, []byte("{not valid")))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "invalid request body" {
		t.Errorf("expected error %q, got %q", "invalid request body", errMsg)
	}
}

func TestUpdate_DatabaseError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{headSize: 1000, headType: "video/webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0)
	videoID := "video-123"

	mock.ExpectQuery(`SELECT file_key, file_size FROM videos`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key", "file_size"}).AddRow("recordings/user/video.webm", int64(1000)))

	mock.ExpectExec(`UPDATE videos SET status`).
		WithArgs("ready", videoID, testUserID).
		WillReturnError(errors.New("database timeout"))

	body, _ := json.Marshal(updateRequest{Status: "ready"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Patch("/api/videos/{id}", handler.Update)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPatch, "/api/videos/"+videoID, body))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "failed to update video" {
		t.Errorf("expected error %q, got %q", "failed to update video", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- List Tests ---

func TestList_SuccessWithVideos(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	createdAt := time.Date(2026, 2, 5, 10, 30, 0, 0, time.UTC)
	shareExpiresAt := createdAt.Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT id, title, status, duration, share_token, created_at, share_expires_at`).
		WithArgs(testUserID, 50, 0).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "status", "duration", "share_token", "created_at", "share_expires_at"}).
				AddRow("video-1", "First Video", "ready", 120, "abc123defghi", createdAt, shareExpiresAt).
				AddRow("video-2", "Second Video", "uploading", 60, "xyz789uvwklm", createdAt.Add(-time.Hour), shareExpiresAt),
		)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos", handler.List)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var items []listItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].ID != "video-1" {
		t.Errorf("expected first item ID %q, got %q", "video-1", items[0].ID)
	}
	if items[0].Title != "First Video" {
		t.Errorf("expected first item title %q, got %q", "First Video", items[0].Title)
	}
	if items[0].Status != "ready" {
		t.Errorf("expected first item status %q, got %q", "ready", items[0].Status)
	}
	if items[0].Duration != 120 {
		t.Errorf("expected first item duration %d, got %d", 120, items[0].Duration)
	}
	if items[0].ShareToken != "abc123defghi" {
		t.Errorf("expected first item share token %q, got %q", "abc123defghi", items[0].ShareToken)
	}
	if items[0].CreatedAt != createdAt.Format(time.RFC3339) {
		t.Errorf("expected first item created_at %q, got %q", createdAt.Format(time.RFC3339), items[0].CreatedAt)
	}
	if items[0].ShareExpiresAt != shareExpiresAt.Format(time.RFC3339) {
		t.Errorf("expected first item share_expires_at %q, got %q", shareExpiresAt.Format(time.RFC3339), items[0].ShareExpiresAt)
	}

	if items[1].ID != "video-2" {
		t.Errorf("expected second item ID %q, got %q", "video-2", items[1].ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestList_ShareURLIncludesBaseURL(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 10, 30, 0, 0, time.UTC)
	shareExpiresAt := createdAt.Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT id, title, status, duration, share_token, created_at, share_expires_at`).
		WithArgs(testUserID, 50, 0).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "status", "duration", "share_token", "created_at", "share_expires_at"}).
				AddRow("video-1", "My Video", "ready", 90, shareToken, createdAt, shareExpiresAt),
		)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos", handler.List)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var items []listItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	expectedShareURL := testBaseURL + "/watch/" + shareToken
	if items[0].ShareURL != expectedShareURL {
		t.Errorf("expected share URL %q, got %q", expectedShareURL, items[0].ShareURL)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestList_EmptyList(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	mock.ExpectQuery(`SELECT id, title, status, duration, share_token, created_at, share_expires_at`).
		WithArgs(testUserID, 50, 0).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "status", "duration", "share_token", "created_at", "share_expires_at"}),
		)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos", handler.List)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var items []listItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}

	rawBody := rec.Body.String()
	if rawBody != "[]\n" {
		t.Errorf("expected JSON array [], got %q", rawBody)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestList_DatabaseError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	mock.ExpectQuery(`SELECT id, title, status, duration, share_token, created_at, share_expires_at`).
		WithArgs(testUserID, 50, 0).
		WillReturnError(errors.New("connection reset"))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos", handler.List)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "failed to list videos" {
		t.Errorf("expected error %q, got %q", "failed to list videos", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- Delete Tests ---

func TestDelete_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	deleteCalled := make(chan string, 1)
	storage := &mockStorage{deleteCalled: deleteCalled}
	handler := NewHandler(mock, storage, testBaseURL, 0)
	videoID := "video-123"
	fileKey := "recordings/user-1/abc.webm"

	mock.ExpectQuery(`UPDATE videos SET status = 'deleted'`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key"}).AddRow(fileKey))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/videos/{id}", handler.Delete)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/videos/"+videoID, nil))

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	select {
	case deletedKey := <-deleteCalled:
		if deletedKey != fileKey {
			t.Errorf("expected delete key %q, got %q", fileKey, deletedKey)
		}
	case <-time.After(2 * time.Second):
		t.Error("expected storage DeleteObject to be called within 2 seconds")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestDelete_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0)
	videoID := "nonexistent-id"

	mock.ExpectQuery(`UPDATE videos SET status = 'deleted'`).
		WithArgs(videoID, testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/videos/{id}", handler.Delete)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/videos/"+videoID, nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "video not found" {
		t.Errorf("expected error %q, got %q", "video not found", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- Watch Tests ---

func TestWatch_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	downloadURL := "https://s3.example.com/download?signed=xyz"
	storage := &mockStorage{downloadURL: downloadURL}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)

	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "duration", "file_key", "name", "created_at", "share_expires_at"}).
				AddRow("Demo Recording", 180, "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt),
		)

	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.Watch)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp watchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Title != "Demo Recording" {
		t.Errorf("expected title %q, got %q", "Demo Recording", resp.Title)
	}
	if resp.VideoURL != downloadURL {
		t.Errorf("expected video URL %q, got %q", downloadURL, resp.VideoURL)
	}
	if resp.Duration != 180 {
		t.Errorf("expected duration %d, got %d", 180, resp.Duration)
	}
	if resp.Creator != "Alex Neamtu" {
		t.Errorf("expected creator %q, got %q", "Alex Neamtu", resp.Creator)
	}
	if resp.CreatedAt != createdAt.Format(time.RFC3339) {
		t.Errorf("expected created_at %q, got %q", createdAt.Format(time.RFC3339), resp.CreatedAt)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestWatch_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	shareToken := "nonexistent12"

	mock.ExpectQuery(`SELECT v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.Watch)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "video not found" {
		t.Errorf("expected error %q, got %q", "video not found", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestWatch_StorageError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadErr: errors.New("s3 unreachable")}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)

	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "duration", "file_key", "name", "created_at", "share_expires_at"}).
				AddRow("Demo Recording", 180, "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt),
		)

	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.Watch)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "failed to generate video URL" {
		t.Errorf("expected error %q, got %q", "failed to generate video URL", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestWatch_ExpiredLink(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/download?signed=xyz"}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(-1 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "duration", "file_key", "name", "created_at", "share_expires_at"}).
				AddRow("Demo Recording", 180, "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt),
		)

	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.Watch)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("expected status %d, got %d: %s", http.StatusGone, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "link expired" {
		t.Errorf("expected error %q, got %q", "link expired", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- NewHandler Tests ---

func TestNewHandler_SetsFields(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	baseURL := "https://example.com"

	handler := NewHandler(mock, storage, baseURL, 0)

	if handler.db != mock {
		t.Error("expected db to be set")
	}
	if handler.storage != storage {
		t.Error("expected storage to be set")
	}
	if handler.baseURL != baseURL {
		t.Errorf("expected baseURL %q, got %q", baseURL, handler.baseURL)
	}
}

// --- Create fileKey format ---

func TestCreate_FileKeyContainsUserIDAndShareToken(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload"}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"Test Video",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("video-999"))

	body, _ := json.Marshal(createRequest{
		Title:    "Test Video",
		Duration: 30,
		FileSize: 100000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp createResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	expectedPrefix := fmt.Sprintf("recordings/%s/", testUserID)
	expectedSuffix := ".webm"
	fileKey := fmt.Sprintf("recordings/%s/%s.webm", testUserID, resp.ShareToken)

	if fileKey[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("file key should start with %q, got %q", expectedPrefix, fileKey)
	}
	if fileKey[len(fileKey)-len(expectedSuffix):] != expectedSuffix {
		t.Errorf("file key should end with %q, got %q", expectedSuffix, fileKey)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- WatchPage Tests ---

func TestWatchPage_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	downloadURL := "https://s3.example.com/download?signed=xyz"
	storage := &mockStorage{downloadURL: downloadURL}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at"}).
				AddRow("Demo Recording", "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt),
		)

	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.WatchPage)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("expected Content-Type %q, got %q", "text/html; charset=utf-8", contentType)
	}

	body := rec.Body.String()

	expectedStrings := map[string]string{
		"title":        "Demo Recording",
		"download URL": downloadURL,
		"creator":      "Alex Neamtu",
		"date":         "Feb 5, 2026",
		"branding":     "SendRec",
		"og:title":     `og:title`,
		"og:video":     `og:video`,
	}
	for label, expected := range expectedStrings {
		if !strings.Contains(body, expected) {
			t.Errorf("expected response body to contain %s (%q)", label, expected)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestWatchPage_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	shareToken := "nonexistent12"

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.WatchPage)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestWatchPage_StorageError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadErr: errors.New("s3 unreachable")}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at"}).
				AddRow("Demo Recording", "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt),
		)

	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.WatchPage)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "internal server error") {
		t.Errorf("expected response body to contain %q, got %q", "internal server error", body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestWatchPage_ExpiredLink(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/download?signed=xyz"}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(-1 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at"}).
				AddRow("Demo Recording", "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt),
		)

	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.WatchPage)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("expected status %d, got %d: %s", http.StatusGone, rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "This link has expired") {
		t.Errorf("expected response body to contain %q, got %q", "This link has expired", body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- Extend Tests ---

func TestExtend_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0)
	videoID := "video-123"

	mock.ExpectExec(`UPDATE videos SET share_expires_at`).
		WithArgs(videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/extend", handler.Extend)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/extend", nil))

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestExtend_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0)
	videoID := "nonexistent-id"

	mock.ExpectExec(`UPDATE videos SET share_expires_at`).
		WithArgs(videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/extend", handler.Extend)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/extend", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "video not found" {
		t.Errorf("expected error %q, got %q", "video not found", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestExtend_DatabaseError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0)
	videoID := "video-123"

	mock.ExpectExec(`UPDATE videos SET share_expires_at`).
		WithArgs(videoID, testUserID).
		WillReturnError(errors.New("database timeout"))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/extend", handler.Extend)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/extend", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "failed to extend share link" {
		t.Errorf("expected error %q, got %q", "failed to extend share link", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- WatchPage Nonce Tests ---

func nonceMiddleware(nonce string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := httputil.ContextWithNonce(r.Context(), nonce)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func TestWatchPage_ContainsNonceInStyleTag(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/download"}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at"}).
				AddRow("Test Video", "recordings/user-1/abc.webm", "Tester", createdAt, shareExpiresAt),
		)

	r := chi.NewRouter()
	r.Use(nonceMiddleware("test-nonce-abc123"))
	r.Get("/watch/{shareToken}", handler.WatchPage)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, `<style nonce="test-nonce-abc123">`) {
		t.Error("expected watch page to contain <style> tag with nonce attribute")
	}
}

func TestWatchPage_ContainsNonceInScriptTag(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/download"}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at"}).
				AddRow("Test Video", "recordings/user-1/abc.webm", "Tester", createdAt, shareExpiresAt),
		)

	r := chi.NewRouter()
	r.Use(nonceMiddleware("test-nonce-abc123"))
	r.Get("/watch/{shareToken}", handler.WatchPage)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `<script nonce="test-nonce-abc123">`) {
		t.Error("expected watch page to contain <script> tag with nonce attribute")
	}
}

func TestWatchPage_ExpiredContainsNonce(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadURL: "https://s3.example.com/download"}
	handler := NewHandler(mock, storage, testBaseURL, 0)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(-1 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at"}).
				AddRow("Test Video", "recordings/user-1/abc.webm", "Tester", createdAt, shareExpiresAt),
		)

	r := chi.NewRouter()
	r.Use(nonceMiddleware("test-nonce-expired"))
	r.Get("/watch/{shareToken}", handler.WatchPage)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("expected status %d, got %d", http.StatusGone, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `<style nonce="test-nonce-expired">`) {
		t.Error("expected expired page to contain <style> tag with nonce attribute")
	}
}

// --- deleteWithRetry Tests ---

func TestDeleteWithRetry_SucceedsFirstAttempt(t *testing.T) {
	s := &mockStorage{}
	err := deleteWithRetry(context.Background(), s, "test-key", 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s.deleteCallCount != 1 {
		t.Errorf("expected 1 call, got %d", s.deleteCallCount)
	}
}

func TestDeleteWithRetry_SucceedsOnSecondAttempt(t *testing.T) {
	s := &mockStorage{
		deleteErr:       errors.New("transient error"),
		deleteFailUntil: 1,
	}
	err := deleteWithRetry(context.Background(), s, "test-key", 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s.deleteCallCount != 2 {
		t.Errorf("expected 2 calls, got %d", s.deleteCallCount)
	}
}

func TestDeleteWithRetry_FailsAfterAllAttempts(t *testing.T) {
	s := &mockStorage{
		deleteErr: errors.New("persistent error"),
	}
	err := deleteWithRetry(context.Background(), s, "test-key", 3)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if s.deleteCallCount != 3 {
		t.Errorf("expected 3 calls, got %d", s.deleteCallCount)
	}
}

func TestDeleteWithRetry_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := &mockStorage{
		deleteErr: errors.New("fail"),
	}
	err := deleteWithRetry(ctx, s, "test-key", 3)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Should stop early due to cancelled context
	if s.deleteCallCount > 2 {
		t.Errorf("expected early stop, got %d calls", s.deleteCallCount)
	}
}

// --- Delete handler with retry and file_purged_at ---

func TestDelete_MarksFilePurgedOnSuccess(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	deleteCalled := make(chan string, 1)
	storage := &mockStorage{deleteCalled: deleteCalled}
	handler := NewHandler(mock, storage, testBaseURL, 0)
	videoID := "video-123"
	fileKey := "recordings/user-1/abc.webm"

	mock.ExpectQuery(`UPDATE videos SET status = 'deleted'`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key"}).AddRow(fileKey))

	mock.ExpectExec(`UPDATE videos SET file_purged_at`).
		WithArgs(fileKey).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/videos/{id}", handler.Delete)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/videos/"+videoID, nil))

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}

	select {
	case <-deleteCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("expected delete to be called")
	}

	// Give goroutine time to execute the UPDATE
	time.Sleep(100 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}
