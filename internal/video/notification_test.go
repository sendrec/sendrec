package video

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/email"
	"github.com/sendrec/sendrec/internal/webhook"
)

// --- GetNotificationPreferences Tests ---

func TestGetNotificationPreferences_ExistingRow(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT view_notification, slack_webhook_url, webhook_url, webhook_secret FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification", "slack_webhook_url", "webhook_url", "webhook_secret"}).
			AddRow("views_only", (*string)(nil), (*string)(nil), (*string)(nil)))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/settings/notifications", handler.GetNotificationPreferences)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/settings/notifications", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["notificationMode"] != "views_only" {
		t.Errorf("expected notificationMode=views_only, got %v", resp["notificationMode"])
	}
}

func TestGetNotificationPreferences_DefaultWhenNoRow(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT view_notification, slack_webhook_url, webhook_url, webhook_secret FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/settings/notifications", handler.GetNotificationPreferences)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/settings/notifications", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["notificationMode"] != "off" {
		t.Errorf("expected notificationMode=off, got %v", resp["notificationMode"])
	}
	if resp["slackWebhookUrl"] != nil {
		t.Errorf("expected slackWebhookUrl=nil, got %v", resp["slackWebhookUrl"])
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
		WithArgs(testUserID, "digest", (*string)(nil), (*string)(nil), "").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body, _ := json.Marshal(setNotificationPreferencesRequest{NotificationMode: "digest"})

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

	body, _ := json.Marshal(setNotificationPreferencesRequest{NotificationMode: "invalid"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/settings/notifications", handler.PutNotificationPreferences)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/settings/notifications", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetNotificationPreferences_IncludesSlackWebhookUrl(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	slackURL := "https://hooks.slack.com/services/T/B/X"
	mock.ExpectQuery(`SELECT view_notification, slack_webhook_url, webhook_url, webhook_secret FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification", "slack_webhook_url", "webhook_url", "webhook_secret"}).
			AddRow("views_only", &slackURL, (*string)(nil), (*string)(nil)))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/settings/notifications", handler.GetNotificationPreferences)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/settings/notifications", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["notificationMode"] != "views_only" {
		t.Errorf("expected notificationMode=views_only, got %v", resp["notificationMode"])
	}
	if resp["slackWebhookUrl"] != slackURL {
		t.Errorf("expected slackWebhookUrl=%s, got %v", slackURL, resp["slackWebhookUrl"])
	}
}

func TestGetNotificationPreferences_SlackWebhookUrlNull(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT view_notification, slack_webhook_url, webhook_url, webhook_secret FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification", "slack_webhook_url", "webhook_url", "webhook_secret"}).
			AddRow("off", (*string)(nil), (*string)(nil), (*string)(nil)))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/settings/notifications", handler.GetNotificationPreferences)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/settings/notifications", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["notificationMode"] != "off" {
		t.Errorf("expected notificationMode=off, got %v", resp["notificationMode"])
	}
	if resp["slackWebhookUrl"] != nil {
		t.Errorf("expected slackWebhookUrl=nil, got %v", resp["slackWebhookUrl"])
	}
}

func TestPutNotificationPreferences_SavesSlackWebhookUrl(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	slackURL := "https://hooks.slack.com/services/T/B/X"
	mock.ExpectExec(`INSERT INTO notification_preferences`).
		WithArgs(testUserID, "views_only", &slackURL, (*string)(nil), "").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := []byte(`{"notificationMode":"views_only","slackWebhookUrl":"https://hooks.slack.com/services/T/B/X"}`)

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

func TestPutNotificationPreferences_ClearsSlackWebhookUrl(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectExec(`INSERT INTO notification_preferences`).
		WithArgs(testUserID, "off", (*string)(nil), (*string)(nil), "").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := []byte(`{"notificationMode":"off","slackWebhookUrl":""}`)

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

func TestPutNotificationPreferences_RejectsInvalidWebhookUrl(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	body := []byte(`{"notificationMode":"views_only","slackWebhookUrl":"https://evil.com/webhook"}`)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/settings/notifications", handler.PutNotificationPreferences)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/settings/notifications", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPutNotificationPreferences_RejectsLongWebhookUrl(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	longURL := "https://hooks.slack.com/services/" + strings.Repeat("x", 500)
	body := []byte(`{"notificationMode":"views_only","slackWebhookUrl":"` + longURL + `"}`)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/settings/notifications", handler.PutNotificationPreferences)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/settings/notifications", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- TestSlackWebhook Tests ---

func TestTestSlackWebhook_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	fakeSlack := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer fakeSlack.Close()

	webhookURL := fakeSlack.URL
	mock.ExpectQuery(`SELECT slack_webhook_url FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"slack_webhook_url"}).AddRow(&webhookURL))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/settings/notifications/test-slack", handler.TestSlackWebhook)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/settings/notifications/test-slack", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTestSlackWebhook_NoUrlConfigured_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT slack_webhook_url FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/settings/notifications/test-slack", handler.TestSlackWebhook)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/settings/notifications/test-slack", nil))

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

	every := "every"
	mock.ExpectExec(`UPDATE videos SET view_notification = \$1`).
		WithArgs(&every, videoID, testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body := []byte(`{"viewNotification":"every"}`)

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

	body := []byte(`{"viewNotification":"first"}`)

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

// --- viewerUserIDFromRequest Tests ---

func TestViewerUserIDFromRequest_ValidRefreshToken(t *testing.T) {
	handler := NewHandler(nil, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	tokenID := "test-token-id"
	refreshToken, err := auth.GenerateRefreshToken(testJWTSecret, testUserID, tokenID)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/watch/abc", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshToken})

	viewerID := handler.viewerUserIDFromRequest(req)
	if viewerID != testUserID {
		t.Errorf("expected %q, got %q", testUserID, viewerID)
	}
}

func TestViewerUserIDFromRequest_NoCookie(t *testing.T) {
	handler := NewHandler(nil, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	req := httptest.NewRequest(http.MethodGet, "/watch/abc", nil)

	viewerID := handler.viewerUserIDFromRequest(req)
	if viewerID != "" {
		t.Errorf("expected empty, got %q", viewerID)
	}
}

func TestViewerUserIDFromRequest_InvalidToken(t *testing.T) {
	handler := NewHandler(nil, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	req := httptest.NewRequest(http.MethodGet, "/watch/abc", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "invalid-token"})

	viewerID := handler.viewerUserIDFromRequest(req)
	if viewerID != "" {
		t.Errorf("expected empty, got %q", viewerID)
	}
}

func TestViewerUserIDFromRequest_AccessTokenRejected(t *testing.T) {
	handler := NewHandler(nil, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	accessToken, err := auth.GenerateAccessToken(testJWTSecret, testUserID)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/watch/abc", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: accessToken})

	viewerID := handler.viewerUserIDFromRequest(req)
	if viewerID != "" {
		t.Errorf("expected empty for access token in cookie, got %q", viewerID)
	}
}

// --- resolveAndNotify owner-skip Tests ---

func TestResolveAndNotify_SkipsWhenViewerIsOwner(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	notifier := &mockViewNotifier{}
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)
	handler.SetViewNotifier(notifier)

	every := "every"
	handler.resolveAndNotify(context.Background(), "vid-1", testUserID, "owner@test.com", "Owner", "My Video", "token123", testUserID, &every)

	if notifier.called {
		t.Error("expected notification to be skipped when viewer is owner")
	}
}

func TestResolveAndNotify_SendsWhenViewerIsDifferent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	notifier := &mockViewNotifier{}
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)
	handler.SetViewNotifier(notifier)

	every := "every"
	handler.resolveAndNotify(context.Background(), "vid-1", testUserID, "owner@test.com", "Owner", "My Video", "token123", "different-user-id", &every)

	if !notifier.called {
		t.Error("expected notification to be sent when viewer is different from owner")
	}
}

func TestResolveAndNotify_SendsWhenViewerIsAnonymous(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	notifier := &mockViewNotifier{}
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)
	handler.SetViewNotifier(notifier)

	every := "every"
	handler.resolveAndNotify(context.Background(), "vid-1", testUserID, "owner@test.com", "Owner", "My Video", "token123", "", &every)

	if !notifier.called {
		t.Error("expected notification to be sent when viewer is anonymous")
	}
}

func TestResolveAndNotify_UsesAccountModeAndSkipsCommentsOnly(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	notifier := &mockViewNotifier{}
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)
	handler.SetViewNotifier(notifier)

	mock.ExpectQuery(`SELECT view_notification FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification"}).AddRow("comments_only"))

	handler.resolveAndNotify(context.Background(), "vid-1", testUserID, "owner@test.com", "Owner", "My Video", "token123", "", nil)

	if notifier.called {
		t.Error("expected no view notification when account mode is comments_only")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestResolveAndNotify_UsesAccountModeAndSendsViewsOnly(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	notifier := &mockViewNotifier{}
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)
	handler.SetViewNotifier(notifier)

	mock.ExpectQuery(`SELECT view_notification FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification"}).AddRow("views_only"))

	handler.resolveAndNotify(context.Background(), "vid-1", testUserID, "owner@test.com", "Owner", "My Video", "token123", "", nil)

	if !notifier.called {
		t.Error("expected view notification when account mode is views_only")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestResolveAndNotify_SlackFiresWhenModeIsOff(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	emailNotifier := &mockViewNotifier{}
	slackNotifier := &mockSlackNotifier{}
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)
	handler.SetViewNotifier(emailNotifier)
	handler.SetSlackNotifier(slackNotifier)

	mock.ExpectQuery(`SELECT view_notification FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification"}).AddRow("off"))

	handler.resolveAndNotify(context.Background(), "vid-1", testUserID, "owner@test.com", "Owner", "My Video", "token123", "", nil)

	if emailNotifier.called {
		t.Error("expected email notification to be skipped when mode is off")
	}
	if !slackNotifier.viewCalled {
		t.Error("expected Slack notification to fire regardless of notification mode")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestResolveAndNotify_SlackSkippedWhenViewerIsOwner(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	slackNotifier := &mockSlackNotifier{}
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)
	handler.SetSlackNotifier(slackNotifier)

	handler.resolveAndNotify(context.Background(), "vid-1", testUserID, "owner@test.com", "Owner", "My Video", "token123", testUserID, nil)

	if slackNotifier.viewCalled {
		t.Error("expected Slack notification to be skipped when viewer is owner")
	}
}

func TestResolveAndNotify_BothFireWhenModeIsViewsOnly(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	emailNotifier := &mockViewNotifier{}
	slackNotifier := &mockSlackNotifier{}
	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)
	handler.SetViewNotifier(emailNotifier)
	handler.SetSlackNotifier(slackNotifier)

	mock.ExpectQuery(`SELECT view_notification FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification"}).AddRow("views_only"))

	handler.resolveAndNotify(context.Background(), "vid-1", testUserID, "owner@test.com", "Owner", "My Video", "token123", "", nil)

	if !emailNotifier.called {
		t.Error("expected email notification when mode is views_only")
	}
	if !slackNotifier.viewCalled {
		t.Error("expected Slack notification when mode is views_only")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// --- Webhook Settings Tests ---

func TestGetNotificationPreferences_IncludesWebhook(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	webhookURL := "https://example.com/webhook"
	webhookSecret := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	mock.ExpectQuery(`SELECT view_notification, slack_webhook_url, webhook_url, webhook_secret FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification", "slack_webhook_url", "webhook_url", "webhook_secret"}).
			AddRow("off", (*string)(nil), &webhookURL, &webhookSecret))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/settings/notifications", handler.GetNotificationPreferences)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/settings/notifications", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["webhookUrl"] != webhookURL {
		t.Errorf("expected webhookUrl=%s, got %v", webhookURL, resp["webhookUrl"])
	}
	if resp["webhookSecret"] != webhookSecret {
		t.Errorf("expected webhookSecret=%s, got %v", webhookSecret, resp["webhookSecret"])
	}
}

func TestPutNotificationPreferences_SavesWebhookUrl(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	webhookURL := "https://example.com/webhook"
	mock.ExpectExec(`INSERT INTO notification_preferences`).
		WithArgs(testUserID, "views_only", (*string)(nil), &webhookURL, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := []byte(`{"notificationMode":"views_only","webhookUrl":"https://example.com/webhook"}`)

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

func TestPutNotificationPreferences_RejectsHttpWebhookUrl(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	body := []byte(`{"notificationMode":"views_only","webhookUrl":"http://example.com/webhook"}`)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/settings/notifications", handler.PutNotificationPreferences)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/settings/notifications", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPutNotificationPreferences_AllowsLocalhostHttp(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	localhostURL := "http://localhost:8080/webhook"
	mock.ExpectExec(`INSERT INTO notification_preferences`).
		WithArgs(testUserID, "off", (*string)(nil), &localhostURL, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := []byte(`{"notificationMode":"off","webhookUrl":"http://localhost:8080/webhook"}`)

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

func TestPutNotificationPreferences_ClearsWebhookWithEmpty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectExec(`INSERT INTO notification_preferences`).
		WithArgs(testUserID, "off", (*string)(nil), (*string)(nil), "").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := []byte(`{"notificationMode":"off","webhookUrl":""}`)

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

func TestRegenerateWebhookSecret(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectExec(`UPDATE notification_preferences SET webhook_secret = \$1, updated_at = now\(\) WHERE user_id = \$2 AND webhook_url IS NOT NULL`).
		WithArgs(pgxmock.AnyArg(), testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/settings/notifications/regenerate-webhook-secret", handler.RegenerateWebhookSecret)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/settings/notifications/regenerate-webhook-secret", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	secret, ok := resp["webhookSecret"].(string)
	if !ok || len(secret) != 64 {
		t.Errorf("expected 64-char hex secret, got %q", secret)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRegenerateWebhookSecret_NoWebhookConfigured(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectExec(`UPDATE notification_preferences SET webhook_secret = \$1, updated_at = now\(\) WHERE user_id = \$2 AND webhook_url IS NOT NULL`).
		WithArgs(pgxmock.AnyArg(), testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/settings/notifications/regenerate-webhook-secret", handler.RegenerateWebhookSecret)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/settings/notifications/regenerate-webhook-secret", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTestWebhook_NoUrl(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT webhook_url, webhook_secret FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnError(pgx.ErrNoRows)

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/settings/notifications/test-webhook", handler.TestWebhook)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/settings/notifications/test-webhook", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTestWebhook_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer fakeServer.Close()

	webhookURL := fakeServer.URL
	webhookSecret := "testsecret123"
	mock.ExpectQuery(`SELECT webhook_url, webhook_secret FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"webhook_url", "webhook_secret"}).AddRow(&webhookURL, &webhookSecret))

	// The webhook client will log the delivery attempt
	mock.ExpectExec(`INSERT INTO webhook_deliveries`).
		WithArgs(testUserID, "webhook.test", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), 1).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	handler.SetWebhookClient(webhook.New(mock))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Post("/api/settings/notifications/test-webhook", handler.TestWebhook)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPost, "/api/settings/notifications/test-webhook", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestListWebhookDeliveries(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	statusCode := 200
	createdAt := time.Date(2026, 2, 21, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT id, event, payload, status_code, response_body, attempt, created_at FROM webhook_deliveries WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "event", "payload", "status_code", "response_body", "attempt", "created_at"}).
			AddRow("del-1", "video.viewed", json.RawMessage(`{"videoId":"v1"}`), &statusCode, "ok", 1, createdAt))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/settings/notifications/webhook-deliveries", handler.ListWebhookDeliveries)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/settings/notifications/webhook-deliveries", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var deliveries []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &deliveries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}
	if deliveries[0]["event"] != "video.viewed" {
		t.Errorf("expected event=video.viewed, got %v", deliveries[0]["event"])
	}
	if deliveries[0]["id"] != "del-1" {
		t.Errorf("expected id=del-1, got %v", deliveries[0]["id"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestListWebhookDeliveries_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT id, event, payload, status_code, response_body, attempt, created_at FROM webhook_deliveries WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "event", "payload", "status_code", "response_body", "attempt", "created_at"}))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/settings/notifications/webhook-deliveries", handler.ListWebhookDeliveries)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/settings/notifications/webhook-deliveries", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var deliveries []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &deliveries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(deliveries) != 0 {
		t.Fatalf("expected 0 deliveries, got %d", len(deliveries))
	}
}

// --- mock view notifier ---

type mockViewNotifier struct {
	called       bool
	toEmail      string
	toName       string
	viewCount    int
	digestCalled bool
	digestVideos []email.DigestVideoSummary
}

func (m *mockViewNotifier) SendViewNotification(_ context.Context, toEmail, toName, _ string, _ string, viewCount int) error {
	m.called = true
	m.toEmail = toEmail
	m.toName = toName
	m.viewCount = viewCount
	return nil
}

func (m *mockViewNotifier) SendDigestNotification(_ context.Context, toEmail, toName string, videos []email.DigestVideoSummary) error {
	m.digestCalled = true
	m.toEmail = toEmail
	m.toName = toName
	m.digestVideos = videos
	return nil
}

type countingDigestNotifier struct {
	callCount *int
}

func (m *countingDigestNotifier) SendViewNotification(context.Context, string, string, string, string, int) error {
	return nil
}

func (m *countingDigestNotifier) SendDigestNotification(_ context.Context, _, _ string, _ []email.DigestVideoSummary) error {
	*m.callCount++
	return nil
}

type mockSlackNotifier struct {
	viewCalled    bool
	commentCalled bool
}

func (m *mockSlackNotifier) SendViewNotification(_ context.Context, _, _, _, _ string, _ int) error {
	m.viewCalled = true
	return nil
}

func (m *mockSlackNotifier) SendCommentNotification(_ context.Context, _, _, _, _, _, _ string) error {
	m.commentCalled = true
	return nil
}
