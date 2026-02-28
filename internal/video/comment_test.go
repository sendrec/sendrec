package video

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
)

// --- SetCommentMode Tests ---

func TestSetCommentMode_ValidMode(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoID := "video-123"

	mock.ExpectExec(`UPDATE videos SET comment_mode = \$1 WHERE id = \$2 AND user_id = \$3`).
		WithArgs("anonymous", videoID, testUserID, (*string)(nil)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(setCommentModeRequest{CommentMode: "anonymous"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/comment-mode", handler.SetCommentMode)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/"+videoID+"/comment-mode", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestSetCommentMode_InvalidMode(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	body, _ := json.Marshal(setCommentModeRequest{CommentMode: "invalid_mode"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/comment-mode", handler.SetCommentMode)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/video-123/comment-mode", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestSetCommentMode_NotOwner(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoID := "video-123"

	mock.ExpectExec(`UPDATE videos SET comment_mode = \$1 WHERE id = \$2 AND user_id = \$3`).
		WithArgs("anonymous", videoID, testUserID, (*string)(nil)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	body, _ := json.Marshal(setCommentModeRequest{CommentMode: "anonymous"})

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Put("/api/videos/{id}/comment-mode", handler.SetCommentMode)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/"+videoID+"/comment-mode", body))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestSetCommentMode_AllValidModes(t *testing.T) {
	modes := []string{"disabled", "anonymous", "name_required", "name_email_required"}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()

			storage := &mockStorage{}
			handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

			mock.ExpectExec(`UPDATE videos SET comment_mode = \$1 WHERE id = \$2 AND user_id = \$3`).
				WithArgs(mode, "video-123", testUserID, (*string)(nil)).
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))

			body, _ := json.Marshal(setCommentModeRequest{CommentMode: mode})

			r := chi.NewRouter()
			r.With(newAuthMiddleware()).Put("/api/videos/{id}/comment-mode", handler.SetCommentMode)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPut, "/api/videos/video-123/comment-mode", body))

			if rec.Code != http.StatusNoContent {
				t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet pgxmock expectations: %v", err)
			}
		})
	}
}

// --- mockCommentNotifier ---

type mockCommentNotifier struct {
	called        bool
	toEmail       string
	videoTitle    string
	commentAuthor string
}

func (m *mockCommentNotifier) SendCommentNotification(_ context.Context, toEmail, toName, videoTitle, commentAuthor, commentBody, watchURL string) error {
	m.called = true
	m.toEmail = toEmail
	m.videoTitle = videoTitle
	m.commentAuthor = commentAuthor
	return nil
}

// --- PostWatchComment Tests ---

func commentVideoRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "user_id", "comment_mode", "share_expires_at", "share_password"})
}

func TestPostWatchComment_AnonymousMode_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	notifier := &mockCommentNotifier{}
	handler.SetCommentNotifier(notifier)

	shareToken := "abc123defghi"
	videoID := "video-123"
	ownerID := "owner-user-1"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow(videoID, ownerID, "anonymous", expiresAt, (*string)(nil)))

	mock.ExpectQuery(`INSERT INTO video_comments`).
		WithArgs(videoID, (*string)(nil), "Someone", "", "Great video!", false, (*float64)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow("comment-1", time.Now()))

	mock.ExpectQuery(`SELECT view_notification FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(ownerID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification"}).AddRow("views_and_comments"))

	mock.ExpectQuery(`SELECT u\.email, u\.name, v\.title FROM users u JOIN videos v ON v\.user_id = u\.id WHERE v\.id = \$1`).
		WithArgs(videoID).
		WillReturnRows(pgxmock.NewRows([]string{"email", "name", "title"}).AddRow("owner@example.com", "Owner", "My Video"))

	body, _ := json.Marshal(postCommentRequest{
		AuthorName: "Someone",
		Body:       "Great video!",
	})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp commentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ID != "comment-1" {
		t.Errorf("expected comment ID comment-1, got %s", resp.ID)
	}
	if resp.AuthorName != "Someone" {
		t.Errorf("expected author Someone, got %s", resp.AuthorName)
	}

	// Wait briefly for async notification goroutine
	time.Sleep(50 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestPostWatchComment_DisabledMode_Returns403(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow("video-123", "owner-1", "disabled", expiresAt, (*string)(nil)))

	body, _ := json.Marshal(postCommentRequest{Body: "Hello"})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestPostWatchComment_NameRequired_MissingName_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow("video-123", "owner-1", "name_required", expiresAt, (*string)(nil)))

	body, _ := json.Marshal(postCommentRequest{Body: "Hello"})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestPostWatchComment_NameEmailRequired_MissingEmail_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow("video-123", "owner-1", "name_email_required", expiresAt, (*string)(nil)))

	body, _ := json.Marshal(postCommentRequest{AuthorName: "Alex", Body: "Hello"})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestPostWatchComment_NameRequired_ReactionAllowsAnonymous(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	videoID := "video-123"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow(videoID, "owner-1", "name_required", expiresAt, (*string)(nil)))

	mock.ExpectQuery(`INSERT INTO video_comments`).
		WithArgs(videoID, (*string)(nil), "", "", "👍", false, (*float64)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow("comment-1", time.Now()))

	body, _ := json.Marshal(postCommentRequest{Body: "👍"})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}
}

func TestPostWatchComment_NameEmailRequired_ReactionAllowsAnonymous(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	videoID := "video-123"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow(videoID, "owner-1", "name_email_required", expiresAt, (*string)(nil)))

	mock.ExpectQuery(`INSERT INTO video_comments`).
		WithArgs(videoID, (*string)(nil), "", "", "🎉", false, (*float64)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow("comment-1", time.Now()))

	body, _ := json.Marshal(postCommentRequest{Body: "🎉"})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}
}

func TestPostWatchComment_NameRequired_NonQuickReactionEmojiStillRequiresName(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow("video-123", "owner-1", "name_required", expiresAt, (*string)(nil)))

	body, _ := json.Marshal(postCommentRequest{Body: "🔥"})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestPostWatchComment_QuickReaction_DoesNotSendCommentNotification(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	notifier := &mockCommentNotifier{}
	handler.SetCommentNotifier(notifier)

	shareToken := "abc123defghi"
	videoID := "video-123"
	ownerID := "owner-user-1"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow(videoID, ownerID, "anonymous", expiresAt, (*string)(nil)))

	mock.ExpectQuery(`INSERT INTO video_comments`).
		WithArgs(videoID, (*string)(nil), "", "", "🎉", false, (*float64)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow("comment-1", time.Now()))

	body, _ := json.Marshal(postCommentRequest{Body: "🎉"})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	// Notification is sent asynchronously for non-reaction comments.
	time.Sleep(50 * time.Millisecond)
	if notifier.called {
		t.Fatal("expected no comment notification for quick reactions")
	}
}

func TestPostWatchComment_NotificationModeViewsOnly_DoesNotSendCommentNotification(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	notifier := &mockCommentNotifier{}
	handler.SetCommentNotifier(notifier)

	shareToken := "abc123defghi"
	videoID := "video-123"
	ownerID := "owner-user-1"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow(videoID, ownerID, "anonymous", expiresAt, (*string)(nil)))

	mock.ExpectQuery(`INSERT INTO video_comments`).
		WithArgs(videoID, (*string)(nil), "Someone", "", "Great video!", false, (*float64)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow("comment-1", time.Now()))

	mock.ExpectQuery(`SELECT view_notification FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(ownerID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification"}).AddRow("views_only"))

	body, _ := json.Marshal(postCommentRequest{
		AuthorName: "Someone",
		Body:       "Great video!",
	})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	time.Sleep(50 * time.Millisecond)
	if notifier.called {
		t.Fatal("expected no comment notification when account mode is views_only")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestPostWatchComment_NotificationModeCommentsOnly_SendsCommentNotification(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	notifier := &mockCommentNotifier{}
	handler.SetCommentNotifier(notifier)

	shareToken := "abc123defghi"
	videoID := "video-123"
	ownerID := "owner-user-1"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow(videoID, ownerID, "anonymous", expiresAt, (*string)(nil)))

	mock.ExpectQuery(`INSERT INTO video_comments`).
		WithArgs(videoID, (*string)(nil), "Someone", "", "Great video!", false, (*float64)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow("comment-1", time.Now()))

	mock.ExpectQuery(`SELECT view_notification FROM notification_preferences WHERE user_id = \$1`).
		WithArgs(ownerID).
		WillReturnRows(pgxmock.NewRows([]string{"view_notification"}).AddRow("comments_only"))

	mock.ExpectQuery(`SELECT u\.email, u\.name, v\.title FROM users u JOIN videos v ON v\.user_id = u\.id WHERE v\.id = \$1`).
		WithArgs(videoID).
		WillReturnRows(pgxmock.NewRows([]string{"email", "name", "title"}).AddRow("owner@example.com", "Owner", "My Video"))

	body, _ := json.Marshal(postCommentRequest{
		AuthorName: "Someone",
		Body:       "Great video!",
	})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	time.Sleep(50 * time.Millisecond)
	if !notifier.called {
		t.Fatal("expected comment notification when account mode is comments_only")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestPostWatchComment_EmptyBody_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow("video-123", "owner-1", "anonymous", expiresAt, (*string)(nil)))

	body, _ := json.Marshal(postCommentRequest{Body: ""})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestPostWatchComment_NameTooLong_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow("video-123", "owner-1", "name_required", expiresAt, (*string)(nil)))

	body, _ := json.Marshal(postCommentRequest{
		AuthorName: strings.Repeat("a", 201),
		Body:       "Hello",
	})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "name is too long" {
		t.Errorf("expected error %q, got %q", "name is too long", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestPostWatchComment_EmailTooLong_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow("video-123", "owner-1", "name_email_required", expiresAt, (*string)(nil)))

	body, _ := json.Marshal(postCommentRequest{
		AuthorName:  "Alex",
		AuthorEmail: strings.Repeat("a", 321),
		Body:        "Hello",
	})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "email is too long" {
		t.Errorf("expected error %q, got %q", "email is too long", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestPostWatchComment_PrivateWithoutJWT_Returns401(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow("video-123", "owner-1", "anonymous", expiresAt, (*string)(nil)))

	body, _ := json.Marshal(postCommentRequest{Body: "Private note", IsPrivate: true})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}

func TestPostWatchComment_ExpiredVideo_Returns410(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(-1 * time.Hour) // expired

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow("video-123", "owner-1", "anonymous", expiresAt, (*string)(nil)))

	body, _ := json.Marshal(postCommentRequest{Body: "Hello"})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("expected status %d, got %d: %s", http.StatusGone, rec.Code, rec.Body.String())
	}
}

func TestPostWatchComment_VideoNotFound_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "nonexistent12"

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows())

	body, _ := json.Marshal(postCommentRequest{Body: "Hello"})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}
}

func TestPostWatchComment_BodyTooLong_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow("video-123", "owner-1", "anonymous", expiresAt, (*string)(nil)))

	longBody := make([]byte, 5001)
	for i := range longBody {
		longBody[i] = 'a'
	}
	body, _ := json.Marshal(postCommentRequest{Body: string(longBody)})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

// --- VideoTimestamp Tests ---

func TestPostWatchComment_WithValidTimestamp_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	videoID := "video-123"
	ownerID := "owner-user-1"
	expiresAt := time.Now().Add(24 * time.Hour)
	timestamp := 83.5

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow(videoID, ownerID, "anonymous", expiresAt, (*string)(nil)))

	mock.ExpectQuery(`INSERT INTO video_comments`).
		WithArgs(videoID, (*string)(nil), "Someone", "", "Great at 83.5s!", false, &timestamp).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow("comment-ts", time.Now()))

	body, _ := json.Marshal(postCommentRequest{
		AuthorName:     "Someone",
		Body:           "Great at 83.5s!",
		VideoTimestamp: &timestamp,
	})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp commentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.VideoTimestamp == nil {
		t.Fatal("expected videoTimestamp to be set, got nil")
	}
	if *resp.VideoTimestamp != 83.5 {
		t.Errorf("expected videoTimestamp 83.5, got %f", *resp.VideoTimestamp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestPostWatchComment_NegativeTimestamp_Returns400(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(24 * time.Hour)
	negativeTimestamp := -5.0

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow("video-123", "owner-1", "anonymous", expiresAt, (*string)(nil)))

	body, _ := json.Marshal(postCommentRequest{
		Body:           "Bad timestamp",
		VideoTimestamp: &negativeTimestamp,
	})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	errMsg := parseErrorResponse(t, rec.Body.Bytes())
	if errMsg != "invalid timestamp" {
		t.Errorf("expected error %q, got %q", "invalid timestamp", errMsg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestPostWatchComment_NilTimestamp_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	videoID := "video-123"
	ownerID := "owner-user-1"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow(videoID, ownerID, "anonymous", expiresAt, (*string)(nil)))

	mock.ExpectQuery(`INSERT INTO video_comments`).
		WithArgs(videoID, (*string)(nil), "", "", "No timestamp here", false, (*float64)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow("comment-no-ts", time.Now()))

	body, _ := json.Marshal(postCommentRequest{
		Body: "No timestamp here",
	})

	r := chi.NewRouter()
	r.Post("/api/watch/{shareToken}/comments", handler.PostWatchComment)

	req := httptest.NewRequest(http.MethodPost, "/api/watch/"+shareToken+"/comments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp commentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.VideoTimestamp != nil {
		t.Errorf("expected videoTimestamp to be nil, got %f", *resp.VideoTimestamp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestListWatchComments_IncludesTimestamp(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	videoID := "video-123"
	ownerID := "owner-user-1"
	expiresAt := time.Now().Add(24 * time.Hour)
	now := time.Now()
	timestamp := 42.7

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow(videoID, ownerID, "anonymous", expiresAt, (*string)(nil)))

	mock.ExpectQuery(`SELECT c\.id, c\.user_id, c\.author_name, c\.body, c\.is_private, c\.created_at, c\.video_timestamp_seconds FROM video_comments c WHERE c\.video_id = \$1 AND c\.is_private = false ORDER BY c\.created_at ASC`).
		WithArgs(videoID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "author_name", "body", "is_private", "created_at", "video_timestamp_seconds"}).
			AddRow("c1", (*string)(nil), "Alex", "At 42.7s", false, now, &timestamp).
			AddRow("c2", (*string)(nil), "Bob", "General comment", false, now, (*float64)(nil)))

	r := chi.NewRouter()
	r.Get("/api/watch/{shareToken}/comments", handler.ListWatchComments)

	req := httptest.NewRequest(http.MethodGet, "/api/watch/"+shareToken+"/comments", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp listCommentsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(resp.Comments))
	}

	if resp.Comments[0].VideoTimestamp == nil {
		t.Fatal("expected first comment to have videoTimestamp, got nil")
	}
	if *resp.Comments[0].VideoTimestamp != 42.7 {
		t.Errorf("expected first comment videoTimestamp 42.7, got %f", *resp.Comments[0].VideoTimestamp)
	}

	if resp.Comments[1].VideoTimestamp != nil {
		t.Errorf("expected second comment to have nil videoTimestamp, got %f", *resp.Comments[1].VideoTimestamp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

// --- ListWatchComments Tests ---

type listCommentsResponse struct {
	Comments    []commentResponse `json:"comments"`
	CommentMode string            `json:"commentMode"`
}

func TestListWatchComments_PublicOnly(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	videoID := "video-123"
	ownerID := "owner-user-1"
	expiresAt := time.Now().Add(24 * time.Hour)
	now := time.Now()

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow(videoID, ownerID, "anonymous", expiresAt, (*string)(nil)))

	mock.ExpectQuery(`SELECT c\.id, c\.user_id, c\.author_name, c\.body, c\.is_private, c\.created_at, c\.video_timestamp_seconds FROM video_comments c WHERE c\.video_id = \$1 AND c\.is_private = false ORDER BY c\.created_at ASC`).
		WithArgs(videoID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "author_name", "body", "is_private", "created_at", "video_timestamp_seconds"}).
			AddRow("c1", (*string)(nil), "Alex", "Nice!", false, now, (*float64)(nil)).
			AddRow("c2", &ownerID, "Owner", "Thanks!", false, now, (*float64)(nil)))

	r := chi.NewRouter()
	r.Get("/api/watch/{shareToken}/comments", handler.ListWatchComments)

	req := httptest.NewRequest(http.MethodGet, "/api/watch/"+shareToken+"/comments", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp listCommentsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(resp.Comments))
	}
	if resp.CommentMode != "anonymous" {
		t.Errorf("expected commentMode anonymous, got %s", resp.CommentMode)
	}
	if !resp.Comments[1].IsOwner {
		t.Error("expected second comment to be marked as owner")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestListWatchComments_DisabledMode_ReturnsEmptyArray(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow("video-123", "owner-1", "disabled", expiresAt, (*string)(nil)))

	r := chi.NewRouter()
	r.Get("/api/watch/{shareToken}/comments", handler.ListWatchComments)

	req := httptest.NewRequest(http.MethodGet, "/api/watch/"+shareToken+"/comments", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp listCommentsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Comments) != 0 {
		t.Errorf("expected 0 comments, got %d", len(resp.Comments))
	}
	if resp.CommentMode != "disabled" {
		t.Errorf("expected commentMode disabled, got %s", resp.CommentMode)
	}
}

func TestListWatchComments_WithJWT_NonOwner_ExcludesPrivate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	videoID := "video-123"
	ownerID := "owner-user-1"
	expiresAt := time.Now().Add(24 * time.Hour)
	now := time.Now()

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow(videoID, ownerID, "anonymous", expiresAt, (*string)(nil)))

	mock.ExpectQuery(`SELECT c\.id, c\.user_id, c\.author_name, c\.body, c\.is_private, c\.created_at, c\.video_timestamp_seconds FROM video_comments c WHERE c\.video_id = \$1 AND c\.is_private = false ORDER BY c\.created_at ASC`).
		WithArgs(videoID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "author_name", "body", "is_private", "created_at", "video_timestamp_seconds"}).
			AddRow("c1", (*string)(nil), "Alex", "Public", false, now, (*float64)(nil)))

	r := chi.NewRouter()
	r.Get("/api/watch/{shareToken}/comments", handler.ListWatchComments)

	req := authenticatedRequest(t, http.MethodGet, "/api/watch/"+shareToken+"/comments", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp listCommentsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Comments) != 1 {
		t.Fatalf("expected 1 public comment (private excluded for non-owner), got %d", len(resp.Comments))
	}
}

func TestListWatchComments_WithJWT_Owner_IncludesPrivate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	videoID := "video-123"
	ownerID := string(testUserID)
	expiresAt := time.Now().Add(24 * time.Hour)
	now := time.Now()
	commenterID := "commenter-1"

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow(videoID, ownerID, "anonymous", expiresAt, (*string)(nil)))

	mock.ExpectQuery(`SELECT c\.id, c\.user_id, c\.author_name, c\.body, c\.is_private, c\.created_at, c\.video_timestamp_seconds FROM video_comments c WHERE c\.video_id = \$1 ORDER BY c\.created_at ASC`).
		WithArgs(videoID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "author_name", "body", "is_private", "created_at", "video_timestamp_seconds"}).
			AddRow("c1", (*string)(nil), "Alex", "Public", false, now, (*float64)(nil)).
			AddRow("c2", &commenterID, "Viewer", "Private note", true, now, (*float64)(nil)))

	r := chi.NewRouter()
	r.Get("/api/watch/{shareToken}/comments", handler.ListWatchComments)

	req := authenticatedRequest(t, http.MethodGet, "/api/watch/"+shareToken+"/comments", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp listCommentsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Comments) != 2 {
		t.Fatalf("expected 2 comments (public + private for owner), got %d", len(resp.Comments))
	}
	if !resp.Comments[1].IsPrivate {
		t.Error("expected second comment to be private")
	}
}

func TestListWatchComments_ExpiredVideo_Returns410(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	shareToken := "abc123defghi"
	expiresAt := time.Now().Add(-1 * time.Hour)

	mock.ExpectQuery(`SELECT v\.id, v\.user_id, v\.comment_mode, v\.share_expires_at, v\.share_password FROM videos v WHERE v\.share_token = \$1 AND v\.status IN \('ready', 'processing'\)`).
		WithArgs(shareToken).
		WillReturnRows(commentVideoRows().AddRow("video-123", "owner-1", "anonymous", expiresAt, (*string)(nil)))

	r := chi.NewRouter()
	r.Get("/api/watch/{shareToken}/comments", handler.ListWatchComments)

	req := httptest.NewRequest(http.MethodGet, "/api/watch/"+shareToken+"/comments", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("expected status %d, got %d: %s", http.StatusGone, rec.Code, rec.Body.String())
	}
}

// --- ListOwnerComments Tests ---

func TestListOwnerComments_ReturnsAllComments(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoID := "video-123"
	now := time.Now()
	commenterID := "commenter-1"

	mock.ExpectQuery(`SELECT v\.user_id, v\.comment_mode FROM videos WHERE id = \$1 AND user_id = \$2`).
		WithArgs(videoID, testUserID, (*string)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "comment_mode"}).AddRow(testUserID, "anonymous"))

	mock.ExpectQuery(`SELECT c\.id, c\.user_id, c\.author_name, c\.body, c\.is_private, c\.created_at, c\.video_timestamp_seconds FROM video_comments c WHERE c\.video_id = \$1 ORDER BY c\.created_at ASC`).
		WithArgs(videoID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "author_name", "body", "is_private", "created_at", "video_timestamp_seconds"}).
			AddRow("c1", (*string)(nil), "Alex", "Public comment", false, now, (*float64)(nil)).
			AddRow("c2", &commenterID, "Viewer", "Private note", true, now, (*float64)(nil)))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos/{id}/comments", handler.ListOwnerComments)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos/"+videoID+"/comments", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp listCommentsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(resp.Comments))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestListOwnerComments_NotOwner_Returns404(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoID := "video-123"

	mock.ExpectQuery(`SELECT v\.user_id, v\.comment_mode FROM videos WHERE id = \$1 AND user_id = \$2`).
		WithArgs(videoID, testUserID, (*string)(nil)).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "comment_mode"}))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Get("/api/videos/{id}/comments", handler.ListOwnerComments)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodGet, "/api/videos/"+videoID+"/comments", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}
}

// --- DeleteComment Tests ---

func TestDeleteComment_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoID := "video-123"
	commentID := "comment-1"

	mock.ExpectExec(`DELETE FROM video_comments c USING videos v WHERE c\.id = \$1 AND c\.video_id = \$2 AND v\.id = c\.video_id AND v\.user_id = \$3`).
		WithArgs(commentID, videoID, testUserID, (*string)(nil)).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/videos/{id}/comments/{commentId}", handler.DeleteComment)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/videos/"+videoID+"/comments/"+commentID, nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}

func TestDeleteComment_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	videoID := "video-123"
	commentID := "nonexistent"

	mock.ExpectExec(`DELETE FROM video_comments c USING videos v WHERE c\.id = \$1 AND c\.video_id = \$2 AND v\.id = c\.video_id AND v\.user_id = \$3`).
		WithArgs(commentID, videoID, testUserID, (*string)(nil)).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Delete("/api/videos/{id}/comments/{commentId}", handler.DeleteComment)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodDelete, "/api/videos/"+videoID+"/comments/"+commentID, nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet pgxmock expectations: %v", err)
	}
}
