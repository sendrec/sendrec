package video

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
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

func TestRecordTranscodeFailure_TransientIncrementsAttempts(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`UPDATE videos`).
		WithArgs("video-1", "no space left on device", false, maxTranscodeAttempts).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(2))

	recordTranscodeFailure(context.Background(), mock, "video-1", fmt.Errorf("no space left on device"))

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRecordTranscodeFailure_PermanentConsumesBudget(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	cause := fmt.Errorf("ffmpeg transcode: exit status 183: EBML header parsing failed")

	// permanent = true, so the statement sets attempts straight to the cap
	// instead of incrementing.
	mock.ExpectQuery(`UPDATE videos`).
		WithArgs("video-1", cause.Error(), true, maxTranscodeAttempts).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(maxTranscodeAttempts))

	recordTranscodeFailure(context.Background(), mock, "video-1", cause)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRecordTranscodeFailure_TruncatesLongMessage(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// ffmpeg dumps its whole log into the error, which can be far larger than
	// anything worth keeping in a column.
	long := strings.Repeat("x", 5000)

	mock.ExpectQuery(`UPDATE videos`).
		WithArgs("video-1", strings.Repeat("x", 2000), false, maxTranscodeAttempts).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(1))

	recordTranscodeFailure(context.Background(), mock, "video-1", fmt.Errorf("%s", long))

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRecordTranscodeFailure_TruncationKeepsValidUTF8(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// ffmpeg quotes the input filename, so the log can carry multi-byte runes.
	// 2000 is not a multiple of 3, so the cut lands inside a rune; Postgres
	// rejects invalid UTF-8 and the attempt counter would never advance.
	long := strings.Repeat("日", 2000)

	mock.ExpectQuery(`UPDATE videos`).
		WithArgs("video-1", strings.Repeat("日", 666), false, maxTranscodeAttempts).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(1))

	recordTranscodeFailure(context.Background(), mock, "video-1", fmt.Errorf("%s", long))

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRecordTranscodeFailure_HandlesDBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`UPDATE videos`).
		WithArgs("video-1", "boom", false, maxTranscodeAttempts).
		WillReturnError(errors.New("connection refused"))

	// Should not panic — the caller has already given up on this attempt.
	recordTranscodeFailure(context.Background(), mock, "video-1", fmt.Errorf("boom"))

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestClearTranscodeFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`UPDATE videos SET transcode_attempts = 0`).
		WithArgs("video-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	clearTranscodeFailure(context.Background(), mock, "video-1")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTranscodeExistingWebM_SkipsExhaustedVideos(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`transcode_attempts < \$1`).
		WithArgs(maxTranscodeAttempts).
		WillReturnRows(pgxmock.NewRows([]string{"id", "file_key"}))

	transcodeExistingWebM(context.Background(), mock, &mockStorage{})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestNormalizeExistingVideos_SkipsExhaustedVideos(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`transcode_attempts < \$1`).
		WithArgs(maxTranscodeAttempts).
		WillReturnRows(pgxmock.NewRows([]string{"id", "file_key"}))

	normalizeExistingVideos(context.Background(), mock, &mockStorage{})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// A job enqueued from the finalize handler reaches TranscodeWebMAsync without
// passing through the worker's transcode_attempts filter.
func TestTranscodeWebMAsync_StopsWhenBudgetExhausted(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{}

	mock.ExpectQuery(`SELECT content_type, transcode_attempts FROM videos`).
		WithArgs("video-1").
		WillReturnRows(pgxmock.NewRows([]string{"content_type", "transcode_attempts"}).
			AddRow("video/webm", maxTranscodeAttempts))

	TranscodeWebMAsync(context.Background(), mock, s, "video-1", "recordings/user/video.webm", "")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	if s.downloadToFileCount != 0 {
		t.Errorf("expected no download, got %d", s.downloadToFileCount)
	}
}

func TestTranscodeWebMAsync_ProceedsBelowBudget(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{downloadToFileErr: fmt.Errorf("s3 down")}

	mock.ExpectQuery(`SELECT content_type, transcode_attempts FROM videos`).
		WithArgs("video-1").
		WillReturnRows(pgxmock.NewRows([]string{"content_type", "transcode_attempts"}).
			AddRow("video/webm", maxTranscodeAttempts-1))

	mock.ExpectQuery(`UPDATE videos`).
		WithArgs("video-1", "s3 down", false, maxTranscodeAttempts).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(maxTranscodeAttempts))

	TranscodeWebMAsync(context.Background(), mock, s, "video-1", "recordings/user/video.webm", "")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	if s.downloadToFileCount != 1 {
		t.Errorf("expected 1 download attempt, got %d", s.downloadToFileCount)
	}
}

func TestNormalizeVideoAsync_StopsWhenBudgetExhausted(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{}

	mock.ExpectQuery(`SELECT ios_normalized, transcode_attempts FROM videos`).
		WithArgs("video-1").
		WillReturnRows(pgxmock.NewRows([]string{"ios_normalized", "transcode_attempts"}).
			AddRow(false, maxTranscodeAttempts))

	NormalizeVideoAsync(context.Background(), mock, s, "video-1", "recordings/user/video.mp4", "")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	if s.downloadToFileCount != 0 {
		t.Errorf("expected no download, got %d", s.downloadToFileCount)
	}
}
