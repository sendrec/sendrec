package video

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v6"
	"github.com/sendrec/sendrec/internal/webhook"
)

// webhookMock backs the webhook client with a database of its own, so a test
// can see which event was dispatched in the delivery log.
func webhookMock(t *testing.T, h *Handler) pgxmock.PgxPoolIface {
	t.Helper()
	wm, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(wm.Close)
	h.SetWebhookClient(webhook.New(wm))
	return wm
}

// A video with a webcam is still Processing at finalize: the watch page shows
// the processing overlay until the composite is done. video.ready must not go
// out yet. BG-13.
func TestUpdate_WithWebcam_HoldsVideoReady(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{headSize: 100000, headType: "video/webm", downloadToFileErr: fmt.Errorf("s3 down")}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	wm := webhookMock(t, handler)
	webcamKey := "recordings/user/video_webcam.webm"

	mock.ExpectQuery(`SELECT file_key, file_size, share_token, webcam_key, content_type, duration FROM videos`).
		WithArgs("video-webcam", testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"file_key", "file_size", "share_token", "webcam_key", "content_type", "duration"}).
			AddRow("recordings/user/video.webm", int64(100000), "abc123defghi", &webcamKey, "video/webm", 120))
	mock.ExpectExec(`UPDATE videos SET status`).
		WithArgs("processing", "video-webcam", testUserID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	// Any dispatch starts with this lookup; it must not happen.
	wm.ExpectQuery(`SELECT webhook_url, webhook_secret`).WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"webhook_url", "webhook_secret"}))

	body, _ := json.Marshal(updateRequest{Status: "ready"})
	r := chi.NewRouter()
	r.With(newAuthMiddleware()).Patch("/api/videos/{id}", handler.Update)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authenticatedRequest(t, http.MethodPatch, "/api/videos/video-webcam", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}
	time.Sleep(300 * time.Millisecond)
	if err := wm.ExpectationsWereMet(); err == nil {
		t.Error("video.ready went out while the video was still processing")
	}
}

// The composite job is what makes a webcam video watchable, overlay or not.
// video.ready goes out when it finishes, here through the fallback. BG-13.
func TestCompositeJob_DispatchesVideoReadyWhenDone(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadToFileErr: fmt.Errorf("s3 down")}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	wm := webhookMock(t, handler)

	mock.ExpectExec(`UPDATE videos SET status = 'ready'`).
		WithArgs("video-123", webcamDroppedWarning, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	expectWebcamCleared(mock)
	mock.ExpectQuery(`SELECT user_id, share_token, duration FROM videos WHERE id = \$1 AND status = 'ready'`).
		WithArgs("video-123").
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "share_token", "duration"}).AddRow(testUserID, "abc123defghi", 120))
	hook := "https://hooks.example.com/x"
	secret := "s"
	wm.ExpectQuery(`SELECT webhook_url, webhook_secret`).WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows([]string{"webhook_url", "webhook_secret"}).AddRow(&hook, &secret))
	wm.ExpectExec(`INSERT INTO webhook_deliveries`).
		WithArgs(testUserID, "video.ready", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), 1).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	handler.EnqueueJob(context.Background(), JobTypeComposite, "video-123", map[string]any{
		"fileKey":      "recordings/user/video.webm",
		"webcamKey":    "recordings/user/video_webcam.webm",
		"thumbnailKey": "recordings/user/video.jpg",
		"contentType":  "video/webm",
		"duration":     120,
	})

	if err := waitExpectations(wm, 15*time.Second); err != nil {
		t.Errorf("want video.ready dispatched after the composite: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
