package video

import (
	"context"
	"fmt"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func TestTrimVideoAsync_DownloadError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{downloadToFileErr: fmt.Errorf("s3 down")}

	mock.ExpectExec(`UPDATE videos SET status = 'ready', updated_at = now\(\) WHERE id =`).
		WithArgs("video-123").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	TrimVideoAsync(context.Background(), mock, s, "video-123",
		"recordings/user/video.webm", "recordings/user/video.jpg",
		"user-123", "sharetoken", 5.0, 30.0)

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

	mock.ExpectExec(`UPDATE videos SET status = 'ready', updated_at = now\(\) WHERE id =`).
		WithArgs("video-123").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	TrimVideoAsync(context.Background(), mock, s, "video-123",
		"recordings/user/video.webm", "recordings/user/video.jpg",
		"user-123", "sharetoken", 5.0, 30.0)

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

	mock.ExpectExec(`UPDATE videos SET status = 'ready', updated_at = now\(\) WHERE id =`).
		WithArgs("video-123").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	TrimVideoAsync(context.Background(), mock, s, "video-123",
		"recordings/user/video.webm", "recordings/user/video.jpg",
		"user-123", "sharetoken", 5.0, 30.0)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
