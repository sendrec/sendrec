package video

import (
	"context"
	"fmt"
	"os/exec"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func TestThumbnailFileKey(t *testing.T) {
	key := thumbnailFileKey("user-123", "abc123defghi")
	expected := "recordings/user-123/abc123defghi.jpg"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestExtractFrame_InvalidInput(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	err := extractFrame("/nonexistent/input.webm", "/tmp/sendrec-test-output.jpg")
	if err == nil {
		t.Error("expected error for nonexistent input file")
	}
}

func TestGenerateThumbnail_StorageDownloadError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{downloadToFileErr: fmt.Errorf("s3 down")}

	// Should log the error but not panic. No DB update expected since download failed.
	GenerateThumbnail(context.Background(), mock, s, "video-123", "recordings/user/abc.webm", "recordings/user/abc.jpg")

	// If we get here without panic, the test passes
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unexpected DB calls: %v", err)
	}
}

func TestGenerateThumbnail_UploadError(t *testing.T) {
	// This test verifies that when ffmpeg isn't available, GenerateThumbnail
	// logs the error and returns without panicking or making DB calls.
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// downloadToFileErr is nil so download "succeeds" but ffmpeg will fail
	// (no actual video content in temp file)
	s := &mockStorage{}

	GenerateThumbnail(context.Background(), mock, s, "video-123", "recordings/user/abc.webm", "recordings/user/abc.jpg")

	// Should not have called DB since ffmpeg/upload failed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unexpected DB calls: %v", err)
	}
}
