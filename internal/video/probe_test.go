package video

import (
	"context"
	"errors"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func TestProbeDuration_DownloadFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{downloadToFileErr: errors.New("download failed")}
	probeDuration(context.Background(), mock, storage, "video-1", "videos/test.mp4")

	// No DB update should be attempted
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProbeDuration_FfprobeFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// download succeeds (creates empty file), ffprobe will fail on empty file
	storage := &mockStorage{}
	probeDuration(context.Background(), mock, storage, "video-1", "videos/test.mp4")

	// No DB update should be attempted
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
