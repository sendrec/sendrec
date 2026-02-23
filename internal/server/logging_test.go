package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestSlogMiddleware_LogsRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })

	r := chi.NewRouter()
	r.Use(slogMiddleware)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	output := buf.String()
	if output == "" {
		t.Fatal("expected log output, got empty string")
	}

	expectedFields := []string{
		"method=GET",
		"path=/test",
		"status=200",
		"remote_addr=",
		"duration_ms=",
	}
	for _, field := range expectedFields {
		if !bytes.Contains([]byte(output), []byte(field)) {
			t.Errorf("expected log to contain %q, got: %s", field, output)
		}
	}
}

func TestSlogMiddleware_SkipsHealthCheck(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })

	r := chi.NewRouter()
	r.Use(slogMiddleware)
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	output := buf.String()
	if output != "" {
		t.Errorf("expected no log output for /api/health, got: %s", output)
	}
}

func TestSlogMiddleware_LogsNon200Status(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })

	r := chi.NewRouter()
	r.Use(slogMiddleware)
	r.Get("/missing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	output := buf.String()
	if output == "" {
		t.Fatal("expected log output, got empty string")
	}

	if !bytes.Contains([]byte(output), []byte("status=404")) {
		t.Errorf("expected log to contain status=404, got: %s", output)
	}
}
