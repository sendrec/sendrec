package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/server"
)

// --- Mock types ---

type mockPinger struct{ err error }

func (m *mockPinger) Ping(ctx context.Context) error { return m.err }

type mockStorage struct{}

func (m *mockStorage) GenerateUploadURL(ctx context.Context, key string, contentType string, contentLength int64, expiry time.Duration) (string, error) {
	return "https://example.com/upload", nil
}

func (m *mockStorage) GenerateDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return "https://example.com/download", nil
}

func (m *mockStorage) DeleteObject(ctx context.Context, key string) error {
	return nil
}

func (m *mockStorage) HeadObject(ctx context.Context, key string) (int64, string, error) {
	return 1024, "video/webm", nil
}

// --- Helpers ---

func newServerWithoutDB() *server.Server {
	return server.New(server.Config{})
}

func newServerWithSPA(webFS fstest.MapFS) *server.Server {
	return server.New(server.Config{WebFS: webFS})
}

func newServerWithDB(t *testing.T) (*server.Server, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	t.Cleanup(func() { mock.Close() })

	srv := server.New(server.Config{
		DB:              mock,
		Pinger:          &mockPinger{err: nil},
		Storage:         &mockStorage{},
		JWTSecret:       "test-secret",
		BaseURL:         "https://localhost:8080",
		S3PublicEndpoint: "https://storage.example.com",
	})
	return srv, mock
}

func testWebFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("console.log('app')")},
		"assets/app.css": {Data: []byte("body{}")},
	}
}

func executeRequest(srv *server.Server, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func executeRequestWithBody(srv *server.Server, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

// --- Health Endpoint (no DB) ---

func TestHealthEndpointReturnsOK(t *testing.T) {
	srv := newServerWithoutDB()
	rec := executeRequest(srv, http.MethodGet, "/api/health")

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	expected := `{"status":"ok"}`
	if rec.Body.String() != expected {
		t.Errorf("expected body %q, got %q", expected, rec.Body.String())
	}
}

func TestHealthEndpointContentType(t *testing.T) {
	srv := newServerWithoutDB()
	rec := executeRequest(srv, http.MethodGet, "/api/health")

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type %q, got %q", "application/json", contentType)
	}
}

// --- Health Endpoint (with DB) ---

func TestHealthEndpointWithPingSuccess(t *testing.T) {
	srv := server.New(server.Config{
		Pinger: &mockPinger{err: nil},
	})
	rec := executeRequest(srv, http.MethodGet, "/api/health")

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	expected := `{"status":"ok"}`
	if rec.Body.String() != expected {
		t.Errorf("expected body %q, got %q", expected, rec.Body.String())
	}
}

func TestHealthEndpointWithPingFailure(t *testing.T) {
	srv := server.New(server.Config{
		Pinger: &mockPinger{err: errors.New("connection refused")},
	})
	rec := executeRequest(srv, http.MethodGet, "/api/health")

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}

	expected := `{"status":"unhealthy","error":"database unreachable"}`
	if rec.Body.String() != expected {
		t.Errorf("expected body %q, got %q", expected, rec.Body.String())
	}
}

// --- Server with nil DB ---

func TestNilDBStillRegistersHealthEndpoint(t *testing.T) {
	srv := newServerWithoutDB()
	rec := executeRequest(srv, http.MethodGet, "/api/health")

	if rec.Code != http.StatusOK {
		t.Errorf("health endpoint should be accessible without DB, got status %d", rec.Code)
	}
}

func TestNilDBAuthRoutesNotRegistered(t *testing.T) {
	srv := newServerWithoutDB()

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/auth/register"},
		{http.MethodPost, "/api/auth/login"},
		{http.MethodPost, "/api/auth/refresh"},
		{http.MethodPost, "/api/auth/logout"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			rec := executeRequest(srv, route.method, route.path)
			if rec.Code != http.StatusNotFound {
				t.Errorf("expected 404 for %s %s without DB, got %d", route.method, route.path, rec.Code)
			}
		})
	}
}

func TestNilDBVideoRoutesNotRegistered(t *testing.T) {
	srv := newServerWithoutDB()

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/videos/"},
		{http.MethodGet, "/api/videos/"},
		{http.MethodPatch, "/api/videos/some-id"},
		{http.MethodDelete, "/api/videos/some-id"},
		{http.MethodGet, "/api/watch/some-token"},
		{http.MethodGet, "/watch/some-token"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			rec := executeRequest(srv, route.method, route.path)
			if rec.Code != http.StatusNotFound {
				t.Errorf("expected 404 for %s %s without DB, got %d", route.method, route.path, rec.Code)
			}
		})
	}
}

// --- Server with DB: auth routes registered ---

func TestAuthRoutesRegisteredWithDB(t *testing.T) {
	srv, _ := newServerWithDB(t)

	rec := executeRequestWithBody(srv, http.MethodPost, "/api/auth/register", "{}")
	if rec.Code == http.StatusNotFound {
		t.Errorf("expected auth/register to be registered (not 404), got %d", rec.Code)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty register body, got %d", rec.Code)
	}
}

func TestLoginRouteRegisteredWithDB(t *testing.T) {
	srv, _ := newServerWithDB(t)

	rec := executeRequestWithBody(srv, http.MethodPost, "/api/auth/login", "{}")
	if rec.Code == http.StatusNotFound {
		t.Errorf("expected auth/login to be registered (not 404), got %d", rec.Code)
	}
}

func TestRefreshRouteRegisteredWithDB(t *testing.T) {
	srv, _ := newServerWithDB(t)

	rec := executeRequest(srv, http.MethodPost, "/api/auth/refresh")
	if rec.Code == http.StatusNotFound {
		t.Errorf("expected auth/refresh to be registered (not 404), got %d", rec.Code)
	}
}

func TestLogoutRouteRegisteredWithDB(t *testing.T) {
	srv, _ := newServerWithDB(t)

	rec := executeRequest(srv, http.MethodPost, "/api/auth/logout")
	if rec.Code == http.StatusNotFound {
		t.Errorf("expected auth/logout to be registered (not 404), got %d", rec.Code)
	}
}

// --- Server with DB: video routes registered ---

func TestVideoCreateRouteRequiresAuth(t *testing.T) {
	srv, _ := newServerWithDB(t)

	rec := executeRequestWithBody(srv, http.MethodPost, "/api/videos/", "{}")
	if rec.Code == http.StatusNotFound {
		t.Errorf("expected /api/videos/ to be registered (not 404), got %d", rec.Code)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated video create, got %d", rec.Code)
	}
}

func TestVideoRoutesRateLimited(t *testing.T) {
	srv, _ := newServerWithDB(t)

	var lastCode int
	for i := 0; i < 30; i++ {
		rec := executeRequestWithBody(srv, http.MethodPost, "/api/videos/", "{}")
		lastCode = rec.Code
		if lastCode == http.StatusTooManyRequests {
			return
		}
	}

	t.Errorf("expected 429 after bursts, last status %d", lastCode)
}

func TestVideoListRouteRequiresAuth(t *testing.T) {
	srv, _ := newServerWithDB(t)

	rec := executeRequest(srv, http.MethodGet, "/api/videos/")
	if rec.Code == http.StatusNotFound {
		t.Errorf("expected GET /api/videos/ to be registered (not 404), got %d", rec.Code)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated video list, got %d", rec.Code)
	}
}

func TestWatchRouteRegisteredWithDB(t *testing.T) {
	srv, mock := newServerWithDB(t)

	mock.ExpectQuery("SELECT v.title, v.duration, v.file_key").
		WithArgs("some-token").
		WillReturnError(errors.New("no rows"))

	rec := executeRequest(srv, http.MethodGet, "/api/watch/some-token")

	// The route is registered if the handler hit the DB mock and returned
	// its own "video not found" error (not the router's default 404).
	body := rec.Body.String()
	if !strings.Contains(body, "video not found") {
		t.Errorf("expected handler response with 'video not found', got %d %q", rec.Code, body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("route /api/watch/{shareToken} not registered: mock expectation unmet: %v", err)
	}
}

func TestWatchPageRouteRegisteredWithDB(t *testing.T) {
	srv, mock := newServerWithDB(t)

	mock.ExpectQuery("SELECT v.title, v.file_key").
		WithArgs("some-token").
		WillReturnError(errors.New("no rows"))

	executeRequest(srv, http.MethodGet, "/watch/some-token")

	// The route is registered if the handler hit the DB mock expectation.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("route /watch/{shareToken} not registered: mock expectation unmet: %v", err)
	}
}

// --- Auth routes have rate limiting ---

func TestAuthRoutesRateLimited(t *testing.T) {
	srv, _ := newServerWithDB(t)

	var lastCode int
	for i := 0; i < 20; i++ {
		rec := executeRequestWithBody(srv, http.MethodPost, "/api/auth/register", "{}")
		lastCode = rec.Code
		if lastCode == http.StatusTooManyRequests {
			return
		}
	}
	t.Errorf("expected 429 after many rapid requests, last status was %d", lastCode)
}

// --- SPA File Server ---

func TestSPAServesExistingFiles(t *testing.T) {
	srv := newServerWithSPA(testWebFS())
	rec := executeRequest(srv, http.MethodGet, "/assets/app.js")

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for existing file, got %d", rec.Code)
	}

	expected := "console.log('app')"
	if rec.Body.String() != expected {
		t.Errorf("expected body %q, got %q", expected, rec.Body.String())
	}
}

func TestSPAFallbackToIndexForUnknownPaths(t *testing.T) {
	srv := newServerWithSPA(testWebFS())
	rec := executeRequest(srv, http.MethodGet, "/dashboard/settings")

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for SPA fallback, got %d", rec.Code)
	}

	expected := "<html>app</html>"
	body := rec.Body.String()
	if body != expected {
		t.Errorf("expected index.html content %q, got %q", expected, body)
	}
}

func TestSPAServesIndexForRootPath(t *testing.T) {
	srv := newServerWithSPA(testWebFS())
	rec := executeRequest(srv, http.MethodGet, "/")

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for root path, got %d", rec.Code)
	}

	expected := "<html>app</html>"
	body := rec.Body.String()
	if body != expected {
		t.Errorf("expected index.html content %q, got %q", expected, body)
	}
}

func TestSPAServesCorrectContentTypeForJS(t *testing.T) {
	srv := newServerWithSPA(testWebFS())
	rec := executeRequest(srv, http.MethodGet, "/assets/app.js")

	contentType := rec.Header().Get("Content-Type")
	expected := "text/javascript; charset=utf-8"
	if contentType != expected {
		t.Errorf("expected Content-Type %q for JS file, got %q", expected, contentType)
	}
}

func TestSPAServesCorrectContentTypeForCSS(t *testing.T) {
	srv := newServerWithSPA(testWebFS())
	rec := executeRequest(srv, http.MethodGet, "/assets/app.css")

	contentType := rec.Header().Get("Content-Type")
	expected := "text/css; charset=utf-8"
	if contentType != expected {
		t.Errorf("expected Content-Type %q for CSS file, got %q", expected, contentType)
	}
}

func TestSPAFallbackForDeeplyNestedPaths(t *testing.T) {
	srv := newServerWithSPA(testWebFS())
	rec := executeRequest(srv, http.MethodGet, "/some/deeply/nested/route")

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for SPA fallback on nested path, got %d", rec.Code)
	}

	expected := "<html>app</html>"
	body := rec.Body.String()
	if body != expected {
		t.Errorf("expected index.html content for nested path, got %q", body)
	}
}

// --- Route Registration (no SPA FS) ---

func TestUnknownRouteReturns404WithoutSPA(t *testing.T) {
	srv := newServerWithoutDB()
	rec := executeRequest(srv, http.MethodGet, "/unknown")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown route without SPA, got %d", rec.Code)
	}
}

func TestHealthEndpointWrongMethodReturnsMethodNotAllowed(t *testing.T) {
	srv := newServerWithoutDB()
	rec := executeRequest(srv, http.MethodPost, "/api/health")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST /api/health, got %d", rec.Code)
	}
}

// --- SPA does not intercept API routes ---

func TestSPADoesNotInterceptHealthEndpoint(t *testing.T) {
	srv := newServerWithSPA(testWebFS())
	rec := executeRequest(srv, http.MethodGet, "/api/health")

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for health endpoint with SPA, got %d", rec.Code)
	}

	expected := `{"status":"ok"}`
	if rec.Body.String() != expected {
		t.Errorf("expected health JSON, got %q", rec.Body.String())
	}
}
