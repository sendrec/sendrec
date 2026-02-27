package video

import (
	"context"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
)

func TestEnqueueJob_UnknownType(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)
	// Should not panic
	handler.EnqueueJob(context.Background(), "unknown", "video-1", nil)
}

func TestEnqueueJob_AllTypesNoPanic(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.MatchExpectationsInOrder(false)

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	types := []struct {
		jobType JobType
		payload map[string]any
	}{
		{JobTypeThumbnail, map[string]any{"thumbnailKey": "thumb.jpg", "fileKey": "video.mp4"}},
		{JobTypeTranscode, map[string]any{"fileKey": "video.webm", "audioFilter": ""}},
		{JobTypeNormalize, map[string]any{"fileKey": "video.mov", "audioFilter": ""}},
		{JobTypeProbe, map[string]any{"fileKey": "video.mp4"}},
		{JobTypeComposite, map[string]any{"fileKey": "video.webm", "webcamKey": "webcam.webm", "thumbnailKey": "thumb.jpg", "contentType": "video/webm"}},
	}

	for _, tt := range types {
		handler.EnqueueJob(context.Background(), tt.jobType, "video-1", tt.payload)
	}

	// Give goroutines a moment to start (they'll fail on storage/ffmpeg but shouldn't panic)
	time.Sleep(200 * time.Millisecond)
}

func TestEnqueueJob_TranscribeType(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	handler := NewHandler(mock, storage, testBaseURL, 0, 0, 0, 0, testJWTSecret, false)

	// EnqueueTranscription does: UPDATE videos SET transcript_status = 'pending', updated_at = now()
	// WHERE id = $1 AND status != 'deleted'
	mock.ExpectExec(`UPDATE videos SET transcript_status = 'pending', updated_at = now\(\)`).
		WithArgs("video-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	handler.EnqueueJob(context.Background(), JobTypeTranscribe, "video-1", nil)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
