package video

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sendrec/sendrec/internal/database"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v6"
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
		WithArgs("video-1", "no space left on device", false, maxTranscodeAttempts, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(2))

	recordTranscodeFailure(context.Background(), mock, "video-1", "k", fmt.Errorf("no space left on device"))

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
		WithArgs("video-1", cause.Error(), true, maxTranscodeAttempts, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(maxTranscodeAttempts))
	// Giving up is the one failure the owner has to hear about. Audit 3.8.
	mock.ExpectExec(`UPDATE videos SET processing_error = \$2`).
		WithArgs("video-1", conversionFailedMessage, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	recordTranscodeFailure(context.Background(), mock, "video-1", "k", cause)

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
		WithArgs("video-1", strings.Repeat("x", 2000), false, maxTranscodeAttempts, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(1))

	recordTranscodeFailure(context.Background(), mock, "video-1", "k", fmt.Errorf("%s", long))

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
		WithArgs("video-1", strings.Repeat("日", 666), false, maxTranscodeAttempts, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(1))

	recordTranscodeFailure(context.Background(), mock, "video-1", "k", fmt.Errorf("%s", long))

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
		WithArgs("video-1", "boom", false, maxTranscodeAttempts, pgxmock.AnyArg()).
		WillReturnError(errors.New("connection refused"))

	// Should not panic — the caller has already given up on this attempt.
	recordTranscodeFailure(context.Background(), mock, "video-1", "k", fmt.Errorf("boom"))

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

	mock.ExpectQuery(`SELECT content_type, transcode_attempts, duration FROM videos`).
		WithArgs("video-1").
		WillReturnRows(pgxmock.NewRows([]string{"content_type", "transcode_attempts", "duration"}).
			AddRow("video/webm", maxTranscodeAttempts, 120))

	TranscodeWebMAsync(context.Background(), mock, s, "video-1", "recordings/user/video.webm", "")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	if s.downloadCalls() != 0 {
		t.Errorf("expected no download, got %d", s.downloadCalls())
	}
}

func TestTranscodeWebMAsync_ProceedsBelowBudget(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{downloadToFileErr: fmt.Errorf("s3 down")}

	mock.ExpectQuery(`SELECT content_type, transcode_attempts, duration FROM videos`).
		WithArgs("video-1").
		WillReturnRows(pgxmock.NewRows([]string{"content_type", "transcode_attempts", "duration"}).
			AddRow("video/webm", maxTranscodeAttempts-1, 120))

	mock.ExpectQuery(`UPDATE videos`).
		WithArgs("video-1", "s3 down", false, maxTranscodeAttempts, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(maxTranscodeAttempts))
	// Giving up is the one failure the owner has to hear about. Audit 3.8.
	mock.ExpectExec(`UPDATE videos SET processing_error = \$2`).
		WithArgs("video-1", conversionFailedMessage, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	TranscodeWebMAsync(context.Background(), mock, s, "video-1", "recordings/user/video.webm", "")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	if s.downloadCalls() != 1 {
		t.Errorf("expected 1 download attempt, got %d", s.downloadCalls())
	}
}

func TestNormalizeVideoAsync_StopsWhenBudgetExhausted(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	s := &mockStorage{}

	mock.ExpectQuery(`SELECT ios_normalized, transcode_attempts, duration FROM videos`).
		WithArgs("video-1").
		WillReturnRows(pgxmock.NewRows([]string{"ios_normalized", "transcode_attempts", "duration"}).
			AddRow(false, maxTranscodeAttempts, 120))

	NormalizeVideoAsync(context.Background(), mock, s, "video-1", "recordings/user/video.mp4", "")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	if s.downloadCalls() != 0 {
		t.Errorf("expected no download, got %d", s.downloadCalls())
	}
}

// stubFFmpeg replaces the ffmpeg call with one that just writes the output
// file, so a test can reach the upload and db-update steps that follow.
func stubFFmpeg(t *testing.T, target *func(context.Context, string, string, string) error) {
	t.Helper()
	original := *target
	*target = func(_ context.Context, _, outputPath, _ string) error {
		return os.WriteFile(outputPath, []byte("fake mp4"), 0o600)
	}
	t.Cleanup(func() { *target = original })
}

// An upload failure leaves the video exactly as the worker query found it, so
// without an attempt bump the worker re-downloads and re-runs ffmpeg forever.
func TestTranscodeWebMAsync_UploadFailureConsumesBudget(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	stubFFmpeg(t, &transcodeToMP4)
	s := &mockStorage{uploadFileErr: fmt.Errorf("s3 unavailable")}

	mock.ExpectQuery(`SELECT content_type, transcode_attempts, duration FROM videos`).
		WithArgs("video-1").
		WillReturnRows(pgxmock.NewRows([]string{"content_type", "transcode_attempts", "duration"}).
			AddRow("video/webm", 0, 120))
	mock.ExpectExec(`INSERT INTO retired_objects`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec(`INSERT INTO retired_objects \(key, delete_after\) VALUES \(\$1, now\(\)\)`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectQuery(`UPDATE videos`).
		WithArgs("video-1", "s3 unavailable", false, maxTranscodeAttempts, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(1))

	TranscodeWebMAsync(context.Background(), mock, s, "video-1", "recordings/user/video.webm", "")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// The transcoded object is already uploaded by this point, so an unbounded
// retry also leaves a duplicate .mp4 behind on every pass.
func TestTranscodeWebMAsync_DBUpdateFailureConsumesBudget(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	stubFFmpeg(t, &transcodeToMP4)
	s := &mockStorage{}

	mock.ExpectQuery(`SELECT content_type, transcode_attempts, duration FROM videos`).
		WithArgs("video-1").
		WillReturnRows(pgxmock.NewRows([]string{"content_type", "transcode_attempts", "duration"}).
			AddRow("video/webm", 0, 120))
	mock.ExpectExec(`INSERT INTO retired_objects`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectQuery(`WITH held AS`).
		WithArgs("video-1", "recordings/user/video.webm", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(errors.New("deadlock detected"))
	mock.ExpectQuery(`UPDATE videos`).
		WithArgs("video-1", "deadlock detected", false, maxTranscodeAttempts, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(1))

	TranscodeWebMAsync(context.Background(), mock, s, "video-1", "recordings/user/video.webm", "")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestNormalizeVideoAsync_UploadFailureConsumesBudget(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	stubFFmpeg(t, &transcodeToIOSCompatible)
	// Report a non-h264 codec so needsNormalization() is true and the re-encode
	// path runs instead of the early "already compatible" exit.
	originalProbe := probeVideoProperties
	probeVideoProperties = func(context.Context, string) (videoProperties, error) {
		return videoProperties{CodecName: "vp9", Width: 1280, Height: 720}, nil
	}
	t.Cleanup(func() { probeVideoProperties = originalProbe })

	s := &mockStorage{uploadFileErr: fmt.Errorf("s3 unavailable")}

	mock.ExpectQuery(`SELECT ios_normalized, transcode_attempts, duration FROM videos`).
		WithArgs("video-1").
		WillReturnRows(pgxmock.NewRows([]string{"ios_normalized", "transcode_attempts", "duration"}).
			AddRow(false, 0, 120))
	mock.ExpectExec(`INSERT INTO retired_objects`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec(`INSERT INTO retired_objects \(key, delete_after\) VALUES \(\$1, now\(\)\)`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectQuery(`UPDATE videos`).
		WithArgs("video-1", "s3 unavailable", false, maxTranscodeAttempts, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(1))

	NormalizeVideoAsync(context.Background(), mock, s, "video-1", "recordings/user/video.mp4", "")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ffmpeg output reaches this column verbatim, so a short message can still
// carry bytes Postgres refuses even when no truncation happens.
func TestRecordTranscodeFailure_SanitizesShortMessages(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Invalid UTF-8 and a NUL, well under the truncation threshold. NUL is
	// valid UTF-8, so ToValidUTF8 alone does not remove it.
	cause := fmt.Errorf("ffmpeg: %s", "bad\xffbyte\x00here")

	mock.ExpectQuery(`UPDATE videos`).
		WithArgs("video-1", "ffmpeg: badbytehere", false, maxTranscodeAttempts, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(1))

	recordTranscodeFailure(context.Background(), mock, "video-1", "k", cause)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestNormalizeVideoAsync_DBUpdateFailureConsumesBudget(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	stubFFmpeg(t, &transcodeToIOSCompatible)
	originalProbe := probeVideoProperties
	probeVideoProperties = func(context.Context, string) (videoProperties, error) {
		return videoProperties{CodecName: "vp9", Width: 1280, Height: 720}, nil
	}
	t.Cleanup(func() { probeVideoProperties = originalProbe })

	s := &mockStorage{}

	mock.ExpectQuery(`SELECT ios_normalized, transcode_attempts, duration FROM videos`).
		WithArgs("video-1").
		WillReturnRows(pgxmock.NewRows([]string{"ios_normalized", "transcode_attempts", "duration"}).
			AddRow(false, 0, 120))
	mock.ExpectExec(`INSERT INTO retired_objects`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectQuery(`WITH held AS`).
		WithArgs("video-1", "recordings/user/video.mp4", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(errors.New("deadlock detected"))
	mock.ExpectQuery(`UPDATE videos`).
		WithArgs("video-1", "deadlock detected", false, maxTranscodeAttempts, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(1))

	NormalizeVideoAsync(context.Background(), mock, s, "video-1", "recordings/user/video.mp4", "")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ctxDB fails on an expired context the way a real pgx pool does; pgxmock
// alone ignores the context.
type ctxDB struct{ database.DBTX }

type errRow struct{ err error }

func (r errRow) Scan(...any) error { return r.err }

func (d ctxDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if err := ctx.Err(); err != nil {
		return errRow{err}
	}
	return d.DBTX.QueryRow(ctx, sql, args...)
}

func (d ctxDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if err := ctx.Err(); err != nil {
		return pgconn.CommandTag{}, err
	}
	return d.DBTX.Exec(ctx, sql, args...)
}

// The commonest failure is the job's own deadline. By then its context has
// expired, and writing the failure on it fails too, so the attempt never
// counted and the job retried forever on the only encoder slot.
func TestRecordTranscodeFailure_CountsAfterTheJobTimedOut(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`UPDATE videos`).
		WithArgs("video-1", pgxmock.AnyArg(), false, maxTranscodeAttempts, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(1))

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	recordTranscodeFailure(ctx, ctxDB{mock}, "video-1", "k", ctx.Err())

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("the failure was not recorded after the timeout: %v", err)
	}
}

// A probe that fails tells us nothing about the file, so the video must not
// be recorded as compatible: that would take it out of every later pass while
// it may still fail on iPhones. It counts as an attempt and is retried within
// the budget. Audit BG-05.
func TestNormalizeVideoAsync_ProbeFailureIsAnAttemptNotAVerdict(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	originalProbe := probeVideoProperties
	probeVideoProperties = func(context.Context, string) (videoProperties, error) {
		return videoProperties{}, fmt.Errorf("ffprobe: exit status 1")
	}
	t.Cleanup(func() { probeVideoProperties = originalProbe })

	mock.ExpectQuery(`SELECT ios_normalized, transcode_attempts, duration FROM videos`).
		WithArgs("video-1").
		WillReturnRows(pgxmock.NewRows([]string{"ios_normalized", "transcode_attempts", "duration"}).
			AddRow(false, 0, 120))
	mock.ExpectQuery(`UPDATE videos`).
		WithArgs("video-1", "ffprobe: exit status 1", false, maxTranscodeAttempts, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"transcode_attempts"}).AddRow(1))

	NormalizeVideoAsync(context.Background(), mock, &mockStorage{}, "video-1", "recordings/user/video.mp4", "")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
