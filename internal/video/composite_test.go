package video

import (
	"context"
	"fmt"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func TestWebcamFileKey(t *testing.T) {
	key := webcamFileKey("user-123", "abc123defghi")
	expected := "recordings/user-123/abc123defghi_webcam.webm"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestCompositeWithWebcam_ScreenDownloadError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{downloadToFileErr: fmt.Errorf("s3 down")}

	// On error, should still set status to "ready" (fallback to screen-only)
	mock.ExpectExec(`UPDATE videos SET status = 'ready', webcam_key = NULL`).
		WithArgs("video-123").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	CompositeWithWebcam(context.Background(), mock, s, "video-123",
		"recordings/user/video.webm", "recordings/user/video_webcam.webm", "recordings/user/video.jpg",
		"user-123", "sharetoken")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestCompositeWithWebcam_FFmpegFailsFallsBackToReady(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Download succeeds but ffmpeg will fail (no actual video content)
	s := &mockStorage{}

	// On error, should still set status to "ready"
	mock.ExpectExec(`UPDATE videos SET status = 'ready', webcam_key = NULL`).
		WithArgs("video-123").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	CompositeWithWebcam(context.Background(), mock, s, "video-123",
		"recordings/user/video.webm", "recordings/user/video_webcam.webm", "recordings/user/video.jpg",
		"user-123", "sharetoken")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
