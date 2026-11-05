package video

import (
	"context"
	"fmt"
	"testing"

	"github.com/pashagolub/pgxmock/v6"
)

func TestTrimVideoAsync_DownloadError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{downloadToFileErr: fmt.Errorf("s3 down")}

	// A failed edit leaves the video as it was, and now says so. Audit 3.3.
	mock.ExpectExec(`UPDATE videos SET status = 'ready', processing_started_at = NULL, processing_error = \$2`).
		WithArgs("video-123", editFailedMessage, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	TrimVideoAsync(context.Background(), mock, s, "video-123",
		"recordings/user/video.webm", "recordings/user/video.jpg",
		"video/webm", 5.0, 30.0)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTrimVideoAsync_FFmpegFailsFallsBackToReady(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{}

	// A failed edit leaves the video as it was, and now says so. Audit 3.3.
	mock.ExpectExec(`UPDATE videos SET status = 'ready', processing_started_at = NULL, processing_error = \$2`).
		WithArgs("video-123", editFailedMessage, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	TrimVideoAsync(context.Background(), mock, s, "video-123",
		"recordings/user/video.webm", "recordings/user/video.jpg",
		"video/webm", 5.0, 30.0)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTrimVideoAsync_UploadError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{uploadFileErr: fmt.Errorf("upload failed")}

	// A failed edit leaves the video as it was, and now says so. Audit 3.3.
	mock.ExpectExec(`UPDATE videos SET status = 'ready', processing_started_at = NULL, processing_error = \$2`).
		WithArgs("video-123", editFailedMessage, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	TrimVideoAsync(context.Background(), mock, s, "video-123",
		"recordings/user/video.webm", "recordings/user/video.jpg",
		"video/webm", 5.0, 30.0)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
