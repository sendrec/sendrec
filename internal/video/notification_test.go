package video

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/auth"
	"github.com/sendrec/sendrec/internal/email"
)

// --- GetNotificationPreferences Tests ---

func TestGetNotificationPreferences_ExistingRow(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	handler := NewHandler(mock, &mockStorage{}, testBaseURL, 0, 0, 0, testJWTSecret, false)

	mock.ExpectQuery(`SELECT view_notification, slack_webhook_url FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification", "slack_webhook_url"}).AddRow("views_only", (*string)(nil)))

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

	mock.ExpectQuery(`SELECT view_notification, slack_webhook_url FROM notification_preferences WHERE user_id = \$1`).
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
		WithArgs(testUserID, "digest", (*string)(nil)).
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
	mock.ExpectQuery(`SELECT view_notification, slack_webhook_url FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification", "slack_webhook_url"}).AddRow("views_only", &slackURL))

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

	mock.ExpectQuery(`SELECT view_notification, slack_webhook_url FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification", "slack_webhook_url"}).AddRow("off", (*string)(nil)))

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
		WithArgs(testUserID, "views_only", &slackURL).
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
		WithArgs(testUserID, "off", (*string)(nil)).
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
