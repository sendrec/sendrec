package storage_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sendrec/sendrec/internal/storage"
)

func newTestStorage(t *testing.T, cfg storage.Config) *storage.Storage {
	t.Helper()
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:1"
	}
	if cfg.Bucket == "" {
		cfg.Bucket = "test-bucket"
	}
	if cfg.AccessKey == "" {
		cfg.AccessKey = "testkey"
	}
	if cfg.SecretKey == "" {
		cfg.SecretKey = "testsecret"
	}
	store, err := storage.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	return store
}

func TestNewWithDefaultRegion(t *testing.T) {
	store, err := storage.New(context.Background(), storage.Config{
		Endpoint:  "http://localhost:1",
		Bucket:    "test-bucket",
		AccessKey: "testkey",
		SecretKey: "testsecret",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil storage")
	}
}

func TestNewWithCustomRegion(t *testing.T) {
	store, err := storage.New(context.Background(), storage.Config{
		Endpoint:  "http://localhost:1",
		Bucket:    "test-bucket",
		AccessKey: "testkey",
		SecretKey: "testsecret",
		Region:    "eu-central-1",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil storage")
	}
}

func TestNewWithValidConfig(t *testing.T) {
	store, err := storage.New(context.Background(), storage.Config{
		Endpoint:  "http://localhost:1",
		Bucket:    "my-bucket",
		AccessKey: "access",
		SecretKey: "secret",
		Region:    "us-west-2",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil storage")
	}
}

func TestConfigDefaultRegionAppliedWhenEmpty(t *testing.T) {
	cfg := storage.Config{
		Endpoint:  "http://localhost:1",
		Bucket:    "test-bucket",
		AccessKey: "testkey",
		SecretKey: "testsecret",
		Region:    "",
	}

	store, err := storage.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected no error with empty region, got: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil storage when region is empty")
	}

	// Verify presigning works, which confirms the region was set internally.
	url, err := store.GenerateDownloadURL(context.Background(), "test-key", 15*time.Minute)
	if err != nil {
		t.Fatalf("presigning failed with default region: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty presigned URL with default region")
	}
}

func TestGenerateUploadURLReturnsNonEmptyURL(t *testing.T) {
	store := newTestStorage(t, storage.Config{})

	url, err := store.GenerateUploadURL(context.Background(), "videos/abc.mp4", "video/mp4", 1024, 15*time.Minute)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty upload URL")
	}
}

func TestGenerateUploadURLContainsBucket(t *testing.T) {
	bucket := "my-upload-bucket"
	store := newTestStorage(t, storage.Config{Bucket: bucket})

	url, err := store.GenerateUploadURL(context.Background(), "file.txt", "text/plain", 0, 10*time.Minute)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(url, bucket) {
		t.Fatalf("expected URL to contain bucket %q, got: %s", bucket, url)
	}
}

func TestGenerateUploadURLContainsKey(t *testing.T) {
	store := newTestStorage(t, storage.Config{})
	key := "recordings/session-123/video.webm"

	url, err := store.GenerateUploadURL(context.Background(), key, "video/webm", 2048, 15*time.Minute)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(url, "recordings") || !strings.Contains(url, "video.webm") {
		t.Fatalf("expected URL to contain key path segments, got: %s", url)
	}
}

func TestGenerateUploadURLWithZeroContentLength(t *testing.T) {
	store := newTestStorage(t, storage.Config{})

	url, err := store.GenerateUploadURL(context.Background(), "file.bin", "application/octet-stream", 0, 5*time.Minute)
	if err != nil {
		t.Fatalf("expected no error with zero content length, got: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty URL with zero content length")
	}
}

func TestGenerateDownloadURLReturnsNonEmptyURL(t *testing.T) {
	store := newTestStorage(t, storage.Config{})

	url, err := store.GenerateDownloadURL(context.Background(), "videos/abc.mp4", 15*time.Minute)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty download URL")
	}
}

func TestGenerateDownloadURLContainsBucket(t *testing.T) {
	bucket := "my-download-bucket"
	store := newTestStorage(t, storage.Config{Bucket: bucket})

	url, err := store.GenerateDownloadURL(context.Background(), "file.txt", 10*time.Minute)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(url, bucket) {
		t.Fatalf("expected URL to contain bucket %q, got: %s", bucket, url)
	}
}

func TestGenerateDownloadURLContainsKey(t *testing.T) {
	store := newTestStorage(t, storage.Config{})
	key := "recordings/session-456/video.mp4"

	url, err := store.GenerateDownloadURL(context.Background(), key, 15*time.Minute)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(url, "recordings") || !strings.Contains(url, "video.mp4") {
		t.Fatalf("expected URL to contain key path segments, got: %s", url)
	}
}

// s3ErrorResponse builds a minimal S3 XML error body that the AWS SDK can parse.
func s3ErrorResponse(code, message string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>%s</Message></Error>`, code, message)
}

// newFakeS3Server creates an httptest server with the given handler and a
// storage.Storage client pointed at it. The caller must call ts.Close()
// when done.
func newFakeS3Server(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *storage.Storage) {
	t.Helper()
	ts := httptest.NewServer(handler)
	store, err := storage.New(context.Background(), storage.Config{
		Endpoint:  ts.URL,
		Bucket:    "test-bucket",
		AccessKey: "testkey",
		SecretKey: "testsecret",
	})
	if err != nil {
		ts.Close()
		t.Fatalf("failed to create storage with fake server: %v", err)
	}
	return ts, store
}

// ---------------------------------------------------------------------------
// DeleteObject
// ---------------------------------------------------------------------------

func TestDeleteObjectSuccess(t *testing.T) {
	var mu sync.Mutex
	var calledPath string
	var calledMethod string

	ts, store := newFakeS3Server(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calledPath = r.URL.Path
		calledMethod = r.Method
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	defer ts.Close()

	err := store.DeleteObject(context.Background(), "videos/abc.mp4")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if calledMethod != http.MethodDelete {
		t.Fatalf("expected DELETE method, got: %s", calledMethod)
	}
	if !strings.Contains(calledPath, "test-bucket") || !strings.Contains(calledPath, "videos/abc.mp4") {
		t.Fatalf("expected path to contain bucket and key, got: %s", calledPath)
	}
}

func TestDeleteObjectError(t *testing.T) {
	ts, store := newFakeS3Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, s3ErrorResponse("AccessDenied", "Access Denied"))
	})
	defer ts.Close()

	err := store.DeleteObject(context.Background(), "videos/abc.mp4")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "delete object") {
		t.Fatalf("expected error to contain 'delete object', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// EnsureBucket
// ---------------------------------------------------------------------------

func TestEnsureBucketAlreadyExists(t *testing.T) {
	var mu sync.Mutex
	var methods []string

	ts, store := newFakeS3Server(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		methods = append(methods, r.Method)
		mu.Unlock()

		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Any other call is unexpected in this test.
		w.WriteHeader(http.StatusOK)
	})
	defer ts.Close()

	err := store.EnsureBucket(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, m := range methods {
		if m == http.MethodPut {
			t.Fatal("CreateBucket (PUT) should not be called when bucket already exists")
		}
	}
}

func TestEnsureBucketCreatesWhenMissing(t *testing.T) {
	var mu sync.Mutex
	var createdBucket bool

	ts, store := newFakeS3Server(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, s3ErrorResponse("NotFound", "Not Found"))
			return
		}
		if r.Method == http.MethodPut {
			createdBucket = true
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer ts.Close()

	err := store.EnsureBucket(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !createdBucket {
		t.Fatal("expected CreateBucket to be called when bucket is missing")
	}
}

func TestEnsureBucketCreateFails(t *testing.T) {
	ts, store := newFakeS3Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, s3ErrorResponse("NotFound", "Not Found"))
			return
		}
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, s3ErrorResponse("InternalError", "Internal Server Error"))
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer ts.Close()

	err := store.EnsureBucket(context.Background())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "create bucket") {
		t.Fatalf("expected error to contain 'create bucket', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GenerateUploadURL — upload limit & nil receiver
// ---------------------------------------------------------------------------

func TestGenerateUploadURL_ExceedsMaxBytes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	store, err := storage.New(context.Background(), storage.Config{
		Endpoint:       ts.URL,
		Bucket:         "test-bucket",
		AccessKey:      "testkey",
		SecretKey:      "testsecret",
		MaxUploadBytes: 1024,
	})
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	_, err = store.GenerateUploadURL(context.Background(), "big.bin", "application/octet-stream", 2048, 15*time.Minute)
	if err == nil {
		t.Fatal("expected error for oversized upload, got nil")
	}
	if !strings.Contains(err.Error(), "file too large") {
		t.Fatalf("expected error to contain 'file too large', got: %v", err)
	}
}

func TestGenerateUploadURL_NilStorage(t *testing.T) {
	var s *storage.Storage

	_, err := s.GenerateUploadURL(context.Background(), "key.bin", "application/octet-stream", 100, 5*time.Minute)
	if err == nil {
		t.Fatal("expected error from nil storage, got nil")
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("expected error to contain 'not initialized', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GenerateDownloadURLWithDisposition
// ---------------------------------------------------------------------------

func TestGenerateDownloadURLWithDisposition_Success(t *testing.T) {
	bucket := "test-bucket"
	store := newTestStorage(t, storage.Config{Bucket: bucket})

	url, err := store.GenerateDownloadURLWithDisposition(context.Background(), "videos/demo.mp4", "demo.mp4", 15*time.Minute)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty URL")
	}
	if !strings.Contains(url, bucket) {
		t.Fatalf("expected URL to contain bucket %q, got: %s", bucket, url)
	}
}

func TestGenerateDownloadURLWithDisposition_SpecialChars(t *testing.T) {
	store := newTestStorage(t, storage.Config{})

	// Filename with quotes, backslashes, and a control character (tab).
	filename := "file\"with\\special\tchars.mp4"
	url, err := store.GenerateDownloadURLWithDisposition(context.Background(), "videos/special.mp4", filename, 15*time.Minute)
	if err != nil {
		t.Fatalf("expected no error for special-char filename, got: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty URL for special-char filename")
	}
}

// ---------------------------------------------------------------------------
// HeadObject
// ---------------------------------------------------------------------------

func TestHeadObject_Success(t *testing.T) {
	ts, store := newFakeS3Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "12345")
			w.Header().Set("Content-Type", "video/mp4")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer ts.Close()

	size, contentType, err := store.HeadObject(context.Background(), "videos/clip.mp4")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if size != 12345 {
		t.Fatalf("expected content length 12345, got: %d", size)
	}
	if contentType != "video/mp4" {
		t.Fatalf("expected content type video/mp4, got: %s", contentType)
	}
}

func TestHeadObject_NotFound(t *testing.T) {
	ts, store := newFakeS3Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer ts.Close()

	_, _, err := store.HeadObject(context.Background(), "missing/key.mp4")
	if err == nil {
		t.Fatal("expected error for missing object, got nil")
	}
	if !strings.Contains(err.Error(), "head object") {
		t.Fatalf("expected error to contain 'head object', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// DownloadToFile
// ---------------------------------------------------------------------------

func TestDownloadToFile_Success(t *testing.T) {
	expectedBody := "fake video content"

	ts, store := newFakeS3Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, expectedBody)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer ts.Close()

	destPath := t.TempDir() + "/downloaded.bin"

	err := store.DownloadToFile(context.Background(), "videos/clip.mp4", destPath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(data) != expectedBody {
		t.Fatalf("expected file content %q, got %q", expectedBody, string(data))
	}
}

// ---------------------------------------------------------------------------
// UploadFile
// ---------------------------------------------------------------------------

func TestUploadFile_Success(t *testing.T) {
	expectedContent := "uploaded file content"

	var mu sync.Mutex
	var receivedBody string
	var receivedMethod string

	ts, store := newFakeS3Server(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if r.Method == http.MethodPut {
			receivedMethod = r.Method
			body, _ := io.ReadAll(r.Body)
			receivedBody = string(body)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer ts.Close()

	srcPath := t.TempDir() + "/upload.txt"
	if err := os.WriteFile(srcPath, []byte(expectedContent), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	err := store.UploadFile(context.Background(), "uploads/file.txt", srcPath, "text/plain")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if receivedMethod != http.MethodPut {
		t.Fatalf("expected PUT method, got: %s", receivedMethod)
	}
	if receivedBody != expectedContent {
		t.Fatalf("expected body %q, got %q", expectedContent, receivedBody)
	}
}
