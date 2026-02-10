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
	uploadURL              string
	webcamUploadURL        string
	uploadErr              error
	downloadURL            string
	downloadErr            error
	downloadDispositionURL string
	downloadDispositionErr error
	deleteErr              error
	deleteCalled           chan string
	deleteCallCount        int
	deleteFailUntil        int
	headSize               int64
	headType               string
	headErr                error
	downloadToFileErr      error
	uploadFileErr          error
}

func (m *mockStorage) GenerateUploadURL(_ context.Context, key string, _ string, _ int64, _ time.Duration) (string, error) {
	if m.webcamUploadURL != "" && strings.HasSuffix(key, "_webcam.webm") {
		return m.webcamUploadURL, m.uploadErr
	}
	return m.uploadURL, m.uploadErr
}

func (m *mockStorage) GenerateDownloadURL(_ context.Context, _ string, _ time.Duration) (string, error) {
	return m.downloadURL, m.downloadErr
}

func (m *mockStorage) GenerateDownloadURLWithDisposition(_ context.Context, _ string, _ string, _ time.Duration) (string, error) {
	if m.downloadDispositionURL != "" || m.downloadDispositionErr != nil {
		return m.downloadDispositionURL, m.downloadDispositionErr
	}
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

func (m *mockStorage) DownloadToFile(_ context.Context, _ string, _ string) error {
	return m.downloadToFileErr
}

func (m *mockStorage) UploadFile(_ context.Context, _ string, _ string, _ string) error {
	return m.uploadFileErr
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"My Recording",
			pgxmock.AnyArg(),
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"Untitled Recording",
			pgxmock.AnyArg(),
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"Test Video",
			pgxmock.AnyArg(),
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"Test Video",
			pgxmock.AnyArg(),
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

// --- Duration Limit Tests ---

func TestCreate_RejectsDurationExceedingLimit(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 300, testJWTSecret, false) // 5 min limit

	body, _ := json.Marshal(createRequest{
		Title:    "Long Video",
		Duration: 301,
		FileSize: 5000000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if !strings.Contains(errMsg, "5 minutes") {
		t.Errorf("expected error mentioning 5 minutes, got %q", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreate_AllowsDurationWithinLimit(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 300, testJWTSecret, false)

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"Short Video",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("video-ok"))

	body, _ := json.Marshal(createRequest{
		Title:    "Short Video",
		Duration: 300,
		FileSize: 5000000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreate_AllowsAnyDurationWhenLimitIsZero(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false) // no limit

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"Very Long Video",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("video-long"))

	body, _ := json.Marshal(createRequest{
		Title:    "Very Long Video",
		Duration: 3600,
		FileSize: 5000000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- Monthly Video Limit Tests ---

func TestCreate_RejectsWhenMonthlyLimitReached(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 25, 0, testJWTSecret, false) // 25/month limit

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM videos`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(25))

	body, _ := json.Marshal(createRequest{
		Title:    "One Too Many",
		Duration: 60,
		FileSize: 5000000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if !strings.Contains(errMsg, "25") {
		t.Errorf("expected error mentioning limit of 25, got %q", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreate_AllowsWhenBelowMonthlyLimit(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 25, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM videos`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(24))

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"Video 25",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("video-25"))

	body, _ := json.Marshal(createRequest{
		Title:    "Video 25",
		Duration: 60,
		FileSize: 5000000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreate_SkipsMonthlyCheckWhenLimitIsZero(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false) // no limit

	// No ExpectQuery for COUNT — should not query at all
	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"Unlimited Video",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("video-unlimited"))

	body, _ := json.Marshal(createRequest{
		Title:    "Unlimited Video",
		Duration: 60,
		FileSize: 5000000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreate_MonthlyLimitCountQueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 25, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM videos`).
		WithArgs(testUserID).
		WillReturnError(errors.New("db error"))

	body, _ := json.Marshal(createRequest{
		Title:    "Error Video",
		Duration: 60,
		FileSize: 5000000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", body))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- Create with Webcam Tests ---

func TestCreate_WithWebcam_ReturnsWebcamUploadURL(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{
		uploadURL:       "https://s3.example.com/upload?signed=screen",
		webcamUploadURL: "https://s3.example.com/upload?signed=webcam",
	}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"Webcam Recording",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("video-webcam"))

	body, _ := json.Marshal(map[string]interface{}{
		"title":          "Webcam Recording",
		"duration":       120,
		"fileSize":       5000000,
		"webcamFileSize": 1000000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["uploadUrl"] != "https://s3.example.com/upload?signed=screen" {
		t.Errorf("expected screen upload URL, got %v", resp["uploadUrl"])
	}
	if resp["webcamUploadUrl"] != "https://s3.example.com/upload?signed=webcam" {
		t.Errorf("expected webcam upload URL, got %v", resp["webcamUploadUrl"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestCreate_WithoutWebcam_OmitsWebcamUploadURL(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{uploadURL: "https://s3.example.com/upload?signed=screen"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"No Webcam",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("video-noweb"))

	body, _ := json.Marshal(map[string]interface{}{
		"title":    "No Webcam",
		"duration": 60,
		"fileSize": 5000000,
	})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos", handler.Create)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if _, exists := resp["webcamUploadUrl"]; exists {
		t.Errorf("expected webcamUploadUrl to be absent, got %v", resp["webcamUploadUrl"])
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"
	fileKey := "recordings/user/video.webm"
	fileSize := int64(100000)

	mock.ExpectQuery(`SELECT file_key, file_size, share_token, webcam_key FROM videos`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key", "file_size", "share_token", "webcam_key"}).AddRow(fileKey, fileSize, "abc123defghi", (*string)(nil)))

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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "nonexistent-id"

	mock.ExpectQuery(`SELECT file_key, file_size, share_token, webcam_key FROM videos`).
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	mock.ExpectQuery(`SELECT file_key, file_size, share_token, webcam_key FROM videos`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key", "file_size", "share_token", "webcam_key"}).AddRow("recordings/user/video.webm", int64(1000), "abc123defghi", (*string)(nil)))

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

// --- Update with Webcam Tests ---

func TestUpdate_WithWebcam_SetsProcessingStatus(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{headSize: 100000, headType: "video/webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-webcam"
	fileKey := "recordings/user/video.webm"
	fileSize := int64(100000)
	webcamKey := "recordings/user/video_webcam.webm"

	mock.ExpectQuery(`SELECT file_key, file_size, share_token, webcam_key FROM videos`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key", "file_size", "share_token", "webcam_key"}).
			AddRow(fileKey, fileSize, "abc123defghi", &webcamKey))

	mock.ExpectExec(`UPDATE videos SET status`).
		WithArgs("processing", videoID, testUserID).
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

func TestUpdate_WithoutWebcam_SetsReadyStatus(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{headSize: 100000, headType: "video/webm"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-noweb"
	fileKey := "recordings/user/video.webm"
	fileSize := int64(100000)

	mock.ExpectQuery(`SELECT file_key, file_size, share_token, webcam_key FROM videos`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key", "file_size", "share_token", "webcam_key"}).
			AddRow(fileKey, fileSize, "abc123defghi", (*string)(nil)))

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

// --- List Tests ---

func TestList_SuccessWithVideos(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	createdAt := time.Date(2026, 2, 5, 10, 30, 0, 0, time.UTC)
	shareExpiresAt := createdAt.Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.status, v.duration, v.share_token, v.created_at, v.share_expires_at`).
		WithArgs(testUserID, 50, 0).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "status", "duration", "share_token", "created_at", "share_expires_at", "view_count", "unique_view_count", "thumbnail_key", "share_password"}).
				AddRow("video-1", "First Video", "ready", 120, "abc123defghi", createdAt, shareExpiresAt, int64(0), int64(0), (*string)(nil), (*string)(nil)).
				AddRow("video-2", "Second Video", "uploading", 60, "xyz789uvwklm", createdAt.Add(-time.Hour), shareExpiresAt, int64(0), int64(0), (*string)(nil), (*string)(nil)),
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
	if items[0].ThumbnailURL != "" {
		t.Errorf("expected empty thumbnail URL for nil thumbnail_key, got %q", items[0].ThumbnailURL)
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 10, 30, 0, 0, time.UTC)
	shareExpiresAt := createdAt.Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.status, v.duration, v.share_token, v.created_at, v.share_expires_at`).
		WithArgs(testUserID, 50, 0).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "status", "duration", "share_token", "created_at", "share_expires_at", "view_count", "unique_view_count", "thumbnail_key", "share_password"}).
				AddRow("video-1", "My Video", "ready", 90, shareToken, createdAt, shareExpiresAt, int64(0), int64(0), (*string)(nil), (*string)(nil)),
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT v.id, v.title, v.status, v.duration, v.share_token, v.created_at, v.share_expires_at`).
		WithArgs(testUserID, 50, 0).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "status", "duration", "share_token", "created_at", "share_expires_at", "view_count", "unique_view_count", "thumbnail_key", "share_password"}),
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT v.id, v.title, v.status, v.duration, v.share_token, v.created_at, v.share_expires_at`).
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

func TestList_IncludesViewCounts(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	createdAt := time.Date(2026, 2, 5, 10, 30, 0, 0, time.UTC)
	shareExpiresAt := createdAt.Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.id, v.title, v.status, v.duration, v.share_token, v.created_at, v.share_expires_at`).
		WithArgs(testUserID, 50, 0).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "status", "duration", "share_token", "created_at", "share_expires_at", "view_count", "unique_view_count", "thumbnail_key", "share_password"}).
				AddRow("video-1", "First Video", "ready", 120, "abc123defghi", createdAt, shareExpiresAt, int64(15), int64(8), (*string)(nil), (*string)(nil)),
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

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ViewCount != 15 {
		t.Errorf("expected ViewCount 15, got %d", items[0].ViewCount)
	}
	if items[0].UniqueViewCount != 8 {
		t.Errorf("expected UniqueViewCount 8, got %d", items[0].UniqueViewCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestList_IncludesThumbnailURL(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	downloadURL := "https://s3.example.com/thumb?signed=abc"
	storage := &mockStorage{downloadURL: downloadURL}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	createdAt := time.Date(2026, 2, 5, 10, 30, 0, 0, time.UTC)
	shareExpiresAt := createdAt.Add(7 * 24 * time.Hour)
	thumbKey := "recordings/user-1/abc123defghi.jpg"

	mock.ExpectQuery(`SELECT v.id, v.title, v.status, v.duration, v.share_token, v.created_at, v.share_expires_at`).
		WithArgs(testUserID, 50, 0).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "status", "duration", "share_token", "created_at", "share_expires_at", "view_count", "unique_view_count", "thumbnail_key", "share_password"}).
				AddRow("video-1", "First Video", "ready", 120, "abc123defghi", createdAt, shareExpiresAt, int64(5), int64(3), &thumbKey, (*string)(nil)),
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

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ThumbnailURL != downloadURL {
		t.Errorf("expected ThumbnailURL %q, got %q", downloadURL, items[0].ThumbnailURL)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"
	fileKey := "recordings/user-1/abc.webm"

	mock.ExpectQuery(`UPDATE videos SET status = 'deleted'`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key", "thumbnail_key", "webcam_key"}).AddRow(fileKey, (*string)(nil), (*string)(nil)))

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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)

	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	videoID := "video-001"

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "duration", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password"}).
				AddRow(videoID, "Demo Recording", 180, "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt, (*string)(nil), (*string)(nil)),
		)

	mock.ExpectExec(`INSERT INTO video_views`).
		WithArgs(videoID, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

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
	if resp.ThumbnailURL != "" {
		t.Errorf("expected empty thumbnail URL for nil thumbnail_key, got %q", resp.ThumbnailURL)
	}

	// Give goroutine time to execute INSERT
	time.Sleep(100 * time.Millisecond)

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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "nonexistent12"

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`).
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)

	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	videoID := "video-001"

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "duration", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password"}).
				AddRow(videoID, "Demo Recording", 180, "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt, (*string)(nil), (*string)(nil)),
		)

	mock.ExpectExec(`INSERT INTO video_views`).
		WithArgs(videoID, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

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

	// Give goroutine time to execute INSERT
	time.Sleep(100 * time.Millisecond)

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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(-1 * time.Hour)

	videoID := "video-001"

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "duration", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password"}).
				AddRow(videoID, "Demo Recording", 180, "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt, (*string)(nil), (*string)(nil)),
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

	handler := NewHandler(mock, storage, baseURL, 0, 0, 0, testJWTSecret, false)

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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`INSERT INTO videos`).
		WithArgs(
			testUserID,
			"Test Video",
			pgxmock.AnyArg(),
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password"}).
				AddRow("Demo Recording", "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt, (*string)(nil), (*string)(nil)),
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "nonexistent12"

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password"}).
				AddRow("Demo Recording", "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt, (*string)(nil), (*string)(nil)),
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(-1 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password"}).
				AddRow("Demo Recording", "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt, (*string)(nil), (*string)(nil)),
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

func TestWatch_RecordsView(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	downloadURL := "https://s3.example.com/download?signed=xyz"
	storage := &mockStorage{downloadURL: downloadURL}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)
	videoID := "video-001"

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "duration", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password"}).
				AddRow(videoID, "Demo Recording", 180, "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt, (*string)(nil), (*string)(nil)),
		)

	mock.ExpectExec(`INSERT INTO video_views`).
		WithArgs(videoID, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	r := chi.NewRouter()
	r.Get("/watch/{shareToken}", handler.Watch)

	req := httptest.NewRequest(http.MethodGet, "/watch/"+shareToken, nil)
	req.Header.Set("User-Agent", "TestBrowser/1.0")
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	// Give goroutine time to execute INSERT
	time.Sleep(100 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWatch_IncludesThumbnailURL(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	downloadURL := "https://s3.example.com/download?signed=xyz"
	storage := &mockStorage{downloadURL: downloadURL}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)
	videoID := "video-001"
	thumbKey := "recordings/user-1/abc123defghi.jpg"

	mock.ExpectQuery(`SELECT v.id, v.title, v.duration, v.file_key, u.name, v.created_at, v.share_expires_at`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "title", "duration", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password"}).
				AddRow(videoID, "Demo Recording", 180, "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt, &thumbKey, (*string)(nil)),
		)

	mock.ExpectExec(`INSERT INTO video_views`).
		WithArgs(videoID, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

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

	if resp.ThumbnailURL != downloadURL {
		t.Errorf("expected ThumbnailURL %q, got %q", downloadURL, resp.ThumbnailURL)
	}

	// Give goroutine time to execute INSERT
	time.Sleep(100 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
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

// --- Trim Tests ---

func TestTrim_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	mock.ExpectQuery(`SELECT duration, file_key, share_token, status FROM videos WHERE id = \$1 AND user_id = \$2`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"duration", "file_key", "share_token", "status"}).
			AddRow(120, "recordings/user/video.webm", "abc123defghi", "ready"))

	mock.ExpectExec(`UPDATE videos SET status = 'processing'`).
		WithArgs(videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/trim", handler.Trim)

	body := []byte(`{"startSeconds": 5.0, "endSeconds": 30.0}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/trim", body))

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d: %s", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestTrim_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "nonexistent-id"

	mock.ExpectQuery(`SELECT duration, file_key, share_token, status FROM videos WHERE id = \$1 AND user_id = \$2`).
		WithArgs(videoID, testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/trim", handler.Trim)

	body := []byte(`{"startSeconds": 5.0, "endSeconds": 30.0}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/trim", body))

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestTrim_VideoNotReady(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	mock.ExpectQuery(`SELECT duration, file_key, share_token, status FROM videos WHERE id = \$1 AND user_id = \$2`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"duration", "file_key", "share_token", "status"}).
			AddRow(120, "recordings/user/video.webm", "abc123defghi", "processing"))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/trim", handler.Trim)

	body := []byte(`{"startSeconds": 5.0, "endSeconds": 30.0}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/trim", body))

	if rec.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestTrim_InvalidBody(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/trim", handler.Trim)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/video-123/trim", []byte(`{invalid`)))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestTrim_StartAfterEnd(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/trim", handler.Trim)

	body := []byte(`{"startSeconds": 30.0, "endSeconds": 10.0}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/video-123/trim", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestTrim_NegativeStart(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/trim", handler.Trim)

	body := []byte(`{"startSeconds": -5.0, "endSeconds": 30.0}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/video-123/trim", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestTrim_EndBeyondDuration(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	mock.ExpectQuery(`SELECT duration, file_key, share_token, status FROM videos WHERE id = \$1 AND user_id = \$2`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"duration", "file_key", "share_token", "status"}).
			AddRow(60, "recordings/user/video.webm", "abc123defghi", "ready"))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/trim", handler.Trim)

	body := []byte(`{"startSeconds": 5.0, "endSeconds": 90.0}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/trim", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestTrim_TrimTooShort(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	mock.ExpectQuery(`SELECT duration, file_key, share_token, status FROM videos WHERE id = \$1 AND user_id = \$2`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"duration", "file_key", "share_token", "status"}).
			AddRow(60, "recordings/user/video.webm", "abc123defghi", "ready"))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/trim", handler.Trim)

	body := []byte(`{"startSeconds": 10.0, "endSeconds": 10.5}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/trim", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestTrim_RaceCondition(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	mock.ExpectQuery(`SELECT duration, file_key, share_token, status FROM videos WHERE id = \$1 AND user_id = \$2`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"duration", "file_key", "share_token", "status"}).
			AddRow(120, "recordings/user/video.webm", "abc123defghi", "ready"))

	mock.ExpectExec(`UPDATE videos SET status = 'processing'`).
		WithArgs(videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/videos/{id}/trim", handler.Trim)

	body := []byte(`{"startSeconds": 5.0, "endSeconds": 30.0}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/videos/"+videoID+"/trim", body))

	if rec.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password"}).
				AddRow("Test Video", "recordings/user-1/abc.webm", "Tester", createdAt, shareExpiresAt, (*string)(nil), (*string)(nil)),
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password"}).
				AddRow("Test Video", "recordings/user-1/abc.webm", "Tester", createdAt, shareExpiresAt, (*string)(nil), (*string)(nil)),
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(-1 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password"}).
				AddRow("Test Video", "recordings/user-1/abc.webm", "Tester", createdAt, shareExpiresAt, (*string)(nil), (*string)(nil)),
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

func TestWatchPage_ContainsPosterAndOGImage(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	downloadURL := "https://s3.example.com/download?signed=xyz"
	storage := &mockStorage{downloadURL: downloadURL}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)
	thumbKey := "recordings/user-1/abc123defghi.jpg"

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password"}).
				AddRow("Demo Recording", "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt, &thumbKey, (*string)(nil)),
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
	if !strings.Contains(body, `poster="`) {
		t.Error("expected watch page to contain poster attribute on video tag")
	}
	if !strings.Contains(body, `og:image`) {
		t.Error("expected watch page to contain og:image meta tag")
	}
	if !strings.Contains(body, downloadURL) {
		t.Errorf("expected watch page to contain thumbnail URL %q", downloadURL)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestWatchPage_NoPosterWhenNoThumbnail(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	downloadURL := "https://s3.example.com/download?signed=xyz"
	storage := &mockStorage{downloadURL: downloadURL}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password"}).
				AddRow("Demo Recording", "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt, (*string)(nil), (*string)(nil)),
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
	if strings.Contains(body, `poster="`) {
		t.Error("expected watch page NOT to contain poster attribute when no thumbnail")
	}
	if strings.Contains(body, `og:image`) {
		t.Error("expected watch page NOT to contain og:image meta tag when no thumbnail")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestWatchPage_ContainsDownloadButton(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	downloadURL := "https://s3.example.com/download?signed=xyz"
	storage := &mockStorage{downloadURL: downloadURL}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	createdAt := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT v.title, v.file_key, u.name, v.created_at, v.share_expires_at, v.thumbnail_key`).
		WithArgs(shareToken).
		WillReturnRows(
			pgxmock.NewRows([]string{"title", "file_key", "name", "created_at", "share_expires_at", "thumbnail_key", "share_password"}).
				AddRow("Demo Recording", "recordings/user-1/abc.webm", "Alex Neamtu", createdAt, shareExpiresAt, (*string)(nil), (*string)(nil)),
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
	if !strings.Contains(body, "Download") {
		t.Error("expected watch page to contain Download button")
	}
	if !strings.Contains(body, "/api/watch/"+shareToken+"/download") {
		t.Errorf("expected watch page to contain download API URL for share token %s", shareToken)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- viewerHash Tests ---

func TestViewerHash_DeterministicOutput(t *testing.T) {
	hash1 := viewerHash("192.168.1.1", "Mozilla/5.0")
	hash2 := viewerHash("192.168.1.1", "Mozilla/5.0")
	if hash1 != hash2 {
		t.Errorf("expected identical hashes, got %q and %q", hash1, hash2)
	}
}

func TestViewerHash_DifferentIPProducesDifferentHash(t *testing.T) {
	hash1 := viewerHash("192.168.1.1", "Mozilla/5.0")
	hash2 := viewerHash("10.0.0.1", "Mozilla/5.0")
	if hash1 == hash2 {
		t.Error("expected different hashes for different IPs")
	}
}

func TestViewerHash_DifferentUAProducesDifferentHash(t *testing.T) {
	hash1 := viewerHash("192.168.1.1", "Mozilla/5.0")
	hash2 := viewerHash("192.168.1.1", "Chrome/120")
	if hash1 == hash2 {
		t.Error("expected different hashes for different user agents")
	}
}

func TestViewerHash_Returns16Characters(t *testing.T) {
	hash := viewerHash("192.168.1.1", "Mozilla/5.0")
	if len(hash) != 16 {
		t.Errorf("expected 16-character hash, got %d: %q", len(hash), hash)
	}
}

// --- clientIP Tests ---

func TestClientIP_UsesXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18")
	if ip := clientIP(req); ip != "203.0.113.50" {
		t.Errorf("expected %q, got %q", "203.0.113.50", ip)
	}
}

func TestClientIP_FallsBackToRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:54321"
	if ip := clientIP(req); ip != "192.168.1.100:54321" {
		t.Errorf("expected %q, got %q", "192.168.1.100:54321", ip)
	}
}

func TestClientIP_SingleXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	if ip := clientIP(req); ip != "203.0.113.50" {
		t.Errorf("expected %q, got %q", "203.0.113.50", ip)
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
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"
	fileKey := "recordings/user-1/abc.webm"

	mock.ExpectQuery(`UPDATE videos SET status = 'deleted'`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key", "thumbnail_key", "webcam_key"}).AddRow(fileKey, (*string)(nil), (*string)(nil)))

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

// --- Delete with Webcam Tests ---

func TestDelete_CleansUpWebcamFile(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	deleteCalled := make(chan string, 3)
	storage := &mockStorage{deleteCalled: deleteCalled}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-webcam-del"
	fileKey := "recordings/user-1/abc.webm"
	webcamKey := "recordings/user-1/abc_webcam.webm"

	mock.ExpectQuery(`UPDATE videos SET status = 'deleted'`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key", "thumbnail_key", "webcam_key"}).AddRow(fileKey, (*string)(nil), &webcamKey))

	mock.ExpectExec(`UPDATE videos SET file_purged_at`).
		WithArgs(fileKey).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/videos/{id}", handler.Delete)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/videos/"+videoID, nil))

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	// Collect deleted keys (video file + webcam file)
	deletedKeys := make(map[string]bool)
	for i := 0; i < 2; i++ {
		select {
		case key := <-deleteCalled:
			deletedKeys[key] = true
		case <-time.After(2 * time.Second):
			t.Fatal("expected 2 delete calls within 2 seconds")
		}
	}

	if !deletedKeys[fileKey] {
		t.Errorf("expected video file %q to be deleted", fileKey)
	}
	if !deletedKeys[webcamKey] {
		t.Errorf("expected webcam file %q to be deleted", webcamKey)
	}

	// Give goroutine time to execute the UPDATE
	time.Sleep(100 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- Limits Endpoint Tests ---

func TestLimits_ReturnsLimitsAndUsage(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 25, 300, testJWTSecret, false)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM videos`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(12))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos/limits", handler.Limits)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos/limits", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp struct {
		MaxVideosPerMonth       int `json:"maxVideosPerMonth"`
		MaxVideoDurationSeconds int `json:"maxVideoDurationSeconds"`
		VideosUsedThisMonth     int `json:"videosUsedThisMonth"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.MaxVideosPerMonth != 25 {
		t.Errorf("expected maxVideosPerMonth 25, got %d", resp.MaxVideosPerMonth)
	}
	if resp.MaxVideoDurationSeconds != 300 {
		t.Errorf("expected maxVideoDurationSeconds 300, got %d", resp.MaxVideoDurationSeconds)
	}
	if resp.VideosUsedThisMonth != 12 {
		t.Errorf("expected videosUsedThisMonth 12, got %d", resp.VideosUsedThisMonth)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestLimits_UnlimitedSkipsCountQuery(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	// No ExpectQuery — should not query COUNT when unlimited

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos/limits", handler.Limits)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos/limits", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp struct {
		MaxVideosPerMonth       int `json:"maxVideosPerMonth"`
		MaxVideoDurationSeconds int `json:"maxVideoDurationSeconds"`
		VideosUsedThisMonth     int `json:"videosUsedThisMonth"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.MaxVideosPerMonth != 0 {
		t.Errorf("expected maxVideosPerMonth 0, got %d", resp.MaxVideosPerMonth)
	}
	if resp.VideosUsedThisMonth != 0 {
		t.Errorf("expected videosUsedThisMonth 0, got %d", resp.VideosUsedThisMonth)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- Download Tests ---

func TestDownload_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	dispositionURL := "https://s3.example.com/download?signed=xyz&disposition=attachment"
	storage := &mockStorage{downloadDispositionURL: dispositionURL}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	mock.ExpectQuery(`SELECT title, file_key FROM videos WHERE id = \$1 AND user_id = \$2 AND status = 'ready'`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"title", "file_key"}).
			AddRow("Demo Recording", "recordings/user-1/abc.webm"))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos/{id}/download", handler.Download)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos/"+videoID+"/download", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp struct {
		DownloadURL string `json:"downloadUrl"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.DownloadURL != dispositionURL {
		t.Errorf("expected downloadUrl %q, got %q", dispositionURL, resp.DownloadURL)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestDownload_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "nonexistent-id"

	mock.ExpectQuery(`SELECT title, file_key FROM videos WHERE id = \$1 AND user_id = \$2 AND status = 'ready'`).
		WithArgs(videoID, testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos/{id}/download", handler.Download)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos/"+videoID+"/download", nil))

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

func TestDownload_StorageError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadDispositionErr: errors.New("s3 unreachable")}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	mock.ExpectQuery(`SELECT title, file_key FROM videos WHERE id = \$1 AND user_id = \$2 AND status = 'ready'`).
		WithArgs(videoID, testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"title", "file_key"}).
			AddRow("Demo Recording", "recordings/user-1/abc.webm"))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos/{id}/download", handler.Download)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos/"+videoID+"/download", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- WatchDownload Tests ---

func TestWatchDownload_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	dispositionURL := "https://s3.example.com/download?signed=xyz&disposition=attachment"
	storage := &mockStorage{downloadDispositionURL: dispositionURL}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	shareExpiresAt := time.Now().Add(7 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT title, file_key, share_expires_at, share_password FROM videos WHERE share_token = \$1 AND status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows([]string{"title", "file_key", "share_expires_at", "share_password"}).
			AddRow("Demo Recording", "recordings/user-1/abc.webm", shareExpiresAt, (*string)(nil)))

	r := chi.NewRouter()
	r.Get("/api/watch/{shareToken}/download", handler.WatchDownload)

	req := httptest.NewRequest(http.MethodGet, "/api/watch/"+shareToken+"/download", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp struct {
		DownloadURL string `json:"downloadUrl"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.DownloadURL != dispositionURL {
		t.Errorf("expected downloadUrl %q, got %q", dispositionURL, resp.DownloadURL)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestWatchDownload_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "nonexistent12"

	mock.ExpectQuery(`SELECT title, file_key, share_expires_at, share_password FROM videos WHERE share_token = \$1 AND status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.Get("/api/watch/{shareToken}/download", handler.WatchDownload)

	req := httptest.NewRequest(http.MethodGet, "/api/watch/"+shareToken+"/download", nil)
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

func TestWatchDownload_Expired(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadDispositionURL: "https://s3.example.com/download"}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	shareExpiresAt := time.Now().Add(-1 * time.Hour)

	mock.ExpectQuery(`SELECT title, file_key, share_expires_at, share_password FROM videos WHERE share_token = \$1 AND status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(pgxmock.NewRows([]string{"title", "file_key", "share_expires_at", "share_password"}).
			AddRow("Demo Recording", "recordings/user-1/abc.webm", shareExpiresAt, (*string)(nil)))

	r := chi.NewRouter()
	r.Get("/api/watch/{shareToken}/download", handler.WatchDownload)

	req := httptest.NewRequest(http.MethodGet, "/api/watch/"+shareToken+"/download", nil)
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

// --- SetPassword Tests ---

func TestSetPassword_SetNewPassword(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "550e8400-e29b-41d4-a716-446655440099"

	mock.ExpectExec(`UPDATE videos SET share_password = \$1, updated_at = now\(\) WHERE id = \$2 AND user_id = \$3 AND status != 'deleted'`).
		WithArgs(pgxmock.AnyArg(), videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body := `{"password":"secret123"}`
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/password", handler.SetPassword)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/"+videoID+"/password", []byte(body)))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestSetPassword_RemovePassword(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "550e8400-e29b-41d4-a716-446655440099"

	mock.ExpectExec(`UPDATE videos SET share_password = NULL, updated_at = now\(\) WHERE id = \$1 AND user_id = \$2 AND status != 'deleted'`).
		WithArgs(videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body := `{"password":""}`
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/password", handler.SetPassword)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/"+videoID+"/password", []byte(body)))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestSetPassword_VideoNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "550e8400-e29b-41d4-a716-446655440099"

	mock.ExpectExec(`UPDATE videos SET share_password = \$1, updated_at = now\(\) WHERE id = \$2 AND user_id = \$3 AND status != 'deleted'`).
		WithArgs(pgxmock.AnyArg(), videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	body := `{"password":"secret123"}`
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/password", handler.SetPassword)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/"+videoID+"/password", []byte(body)))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}
