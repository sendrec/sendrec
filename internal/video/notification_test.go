package video

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

// --- GetNotificationPreferences Tests ---

func TestGetNotificationPreferences_ExistingRow(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT view_notification FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification"}).AddRow("every"))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/settings/notifications", handler.GetNotificationPreferences)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/settings/notifications", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["viewNotification"] != "every" {
		t.Errorf("expected viewNotification=every, got %s", resp["viewNotification"])
	}
}

func TestGetNotificationPreferences_DefaultWhenNoRow(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT view_notification FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/settings/notifications", handler.GetNotificationPreferences)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/settings/notifications", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["viewNotification"] != "off" {
		t.Errorf("expected viewNotification=off, got %s", resp["viewNotification"])
	}
}

// --- PutNotificationPreferences Tests ---

func TestPutNotificationPreferences_ValidMode(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectExec(`INSERT INTO notification_preferences`).
		WithArgs(testUserID, "digest").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body, _ := json.Marshal(setNotificationPreferencesRequest{ViewNotification: "digest"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/settings/notifications", handler.PutNotificationPreferences)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/settings/notifications", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPutNotificationPreferences_InvalidMode(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	body, _ := json.Marshal(setNotificationPreferencesRequest{ViewNotification: "invalid"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/settings/notifications", handler.PutNotificationPreferences)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/settings/notifications", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- SetVideoNotification Tests ---

func TestSetVideoNotification_SetMode(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-123"

	first := "first"
	mock.ExpectExec(`UPDATE videos SET view_notification = \$1`).
		WithArgs(&first, videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body := []byte(`{"viewNotification":"first"}`)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/notifications", handler.SetVideoNotification)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/"+videoID+"/notifications", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSetVideoNotification_ClearOverride(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)
	videoID := "video-456"

	mock.ExpectExec(`UPDATE videos SET view_notification = \$1`).
		WithArgs((*string)(nil), videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body := []byte(`{"viewNotification":null}`)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/notifications", handler.SetVideoNotification)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/"+videoID+"/notifications", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSetVideoNotification_InvalidMode(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	body := []byte(`{"viewNotification":"invalid"}`)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/notifications", handler.SetVideoNotification)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/video-123/notifications", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSetVideoNotification_NotOwner(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	every := "every"
	mock.ExpectExec(`UPDATE videos SET view_notification = \$1`).
		WithArgs(&every, "video-999", testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	body := []byte(`{"viewNotification":"every"}`)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/notifications", handler.SetVideoNotification)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/video-999/notifications", body))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- mock view notifier ---

type mockViewNotifier struct {
	called    bool
	toEmail   string
	toName    string
	viewCount int
}

func (m *mockViewNotifier) SendViewNotification(_ context.Context, toEmail, toName, _ string, _ string, viewCount int, _ bool) error {
	m.called = true
	m.toEmail = toEmail
	m.toName = toName
	m.viewCount = viewCount
	return nil
}
