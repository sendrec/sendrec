package video

import (
	"context"
	"fmt"
	"slices"
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

	TranscodeWebMAsync(context.Background(), mock, s, "video-123", "recordings/user/video.webm", "")

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

	TranscodeWebMAsync(context.Background(), mock, s, "video-123", "recordings/user/video.webm", "")

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

	TranscodeWebMAsync(context.Background(), mock, s, "video-123", "recordings/user/video.webm", "")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestIsPermanentFFmpegError(t *testing.T) {
	if isPermanentFFmpegError(nil) {
		t.Error("expected nil error to be non-permanent")
	}
	// The failure seen in production: browser upload stored a webm without a
	// readable header, so every retry produced the same result.
	corrupt := fmt.Errorf("ffmpeg transcode: exit status 183: [matroska,webm] EBML header parsing failed\nError opening input: Invalid data found when processing input")
	if !isPermanentFFmpegError(corrupt) {
		t.Error("expected corrupt webm error to be permanent")
	}
	if isPermanentFFmpegError(fmt.Errorf("ffmpeg transcode: exit status 1: No space left on device")) {
		t.Error("expected transient error to be non-permanent")
	}
}

func TestBuildTranscodeArgs(t *testing.T) {
	t.Run("without audio filter", func(t *testing.T) {
		args := buildTranscodeArgs("input.webm", "output.mp4", "")
		if slices.Contains(args, "-af") {
			t.Error("expected no -af flag when audioFilter is empty")
		}
		if !slices.Contains(args, "-c:a") {
			t.Error("expected -c:a flag")
		}
	})

	t.Run("with audio filter", func(t *testing.T) {
		args := buildTranscodeArgs("input.webm", "output.mp4", "arnndn=m=/app/models/std.rnnn")
		afIdx := slices.Index(args, "-af")
		if afIdx == -1 {
			t.Fatal("expected -af flag")
		}
		if args[afIdx+1] != "arnndn=m=/app/models/std.rnnn" {
			t.Errorf("expected filter value, got %q", args[afIdx+1])
		}
		// -af should come before -c:a
		caIdx := slices.Index(args, "-c:a")
		if afIdx >= caIdx {
			t.Error("expected -af before -c:a")
		}
	})
}

func TestBuildNormalizeArgs(t *testing.T) {
	t.Run("without audio filter", func(t *testing.T) {
		args := buildNormalizeArgs("input.mp4", "output.mp4", "")
		if slices.Contains(args, "-af") {
			t.Error("expected no -af flag when audioFilter is empty")
		}
	})

	t.Run("with audio filter", func(t *testing.T) {
		args := buildNormalizeArgs("input.mp4", "output.mp4", "afftdn=nr=12:nf=-50")
		afIdx := slices.Index(args, "-af")
		if afIdx == -1 {
			t.Fatal("expected -af flag")
		}
		if args[afIdx+1] != "afftdn=nr=12:nf=-50" {
			t.Errorf("expected filter value, got %q", args[afIdx+1])
		}
	})
}
