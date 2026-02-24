package video

import (
	"context"
	"fmt"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func TestTranscodeWebMAsync_DownloadError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{downloadToFileErr: fmt.Errorf("s3 down")}

	TranscodeWebMAsync(context.Background(), mock, s, "video-123", "recordings/user/video.webm")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTranscodeWebMAsync_FFmpegFails(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{}

	TranscodeWebMAsync(context.Background(), mock, s, "video-123", "recordings/user/video.webm")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTranscodeWebMAsync_UploadError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{uploadFileErr: fmt.Errorf("upload failed")}

	TranscodeWebMAsync(context.Background(), mock, s, "video-123", "recordings/user/video.webm")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
