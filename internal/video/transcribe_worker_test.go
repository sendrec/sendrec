package video

import (
	"context"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v6"
)

type stubTranscriber struct {
	available bool
	segments  []TranscriptSegment
	err       error
}

func (s stubTranscriber) Name() string    { return "stub" }
func (s stubTranscriber) Available() bool { return s.available }
func (s stubTranscriber) Transcribe(ctx context.Context, audioPath, language string) ([]TranscriptSegment, error) {
	return s.segments, s.err
}

func TestEnqueueTranscription(t *testing.T) {
	t.Setenv("TRANSCRIPTION_ENABLED", "true")
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`UPDATE videos SET transcript_status = 'pending', updated_at = now\(\)`).
		WithArgs("video-123").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := EnqueueTranscription(context.Background(), mock, "video-123"); err != nil {
		t.Errorf("EnqueueTranscription failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEnqueueTranscription_DisabledIsNoOp(t *testing.T) {
	t.Setenv("TRANSCRIPTION_ENABLED", "false")
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// No DB expectations - call should short-circuit before touching DB.
	if err := EnqueueTranscription(context.Background(), mock, "video-123"); err != nil {
		t.Errorf("EnqueueTranscription failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessNextTranscription_NoJobs(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Stuck job reset
	mock.ExpectExec(`UPDATE videos SET transcript_status = 'pending'`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	// Claim query returns no rows
	mock.ExpectQuery(`UPDATE videos SET transcript_status = 'processing'`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "file_key", "media_version", "user_id", "share_token", "language"}))

	storage := &mockStorage{}
	processNextTranscription(context.Background(), mock, storage, stubTranscriber{available: true}, false)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessNextTranscription_ResetsStuckJobs(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Stuck job reset — should match 1 row
	mock.ExpectExec(`UPDATE videos SET transcript_status = 'pending'`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// Claim query returns no rows (the reset job will be picked up next tick)
	mock.ExpectQuery(`UPDATE videos SET transcript_status = 'processing'`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "file_key", "media_version", "user_id", "share_token", "language"}))

	storage := &mockStorage{}
	processNextTranscription(context.Background(), mock, storage, stubTranscriber{available: true}, false)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// hangingStorage stands in for a job that never finishes on its own, like a
// hung whisper-cli: the download blocks until its context ends.
type hangingStorage struct{ *mockStorage }

func (hangingStorage) DownloadToFile(ctx context.Context, _, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

// One hung job used to hold the single transcription worker forever. Each job
// now has a deadline, and the failure is recorded once it passes. BG-07.
func TestProcessTranscription_HungJobFailsAtItsDeadline(t *testing.T) {
	t.Setenv("TRANSCRIPTION_ENABLED", "true")
	orig := transcriptionJobTimeout
	transcriptionJobTimeout = 50 * time.Millisecond
	t.Cleanup(func() { transcriptionJobTimeout = orig })

	db := &recoveryDB{}
	done := make(chan struct{})
	go func() {
		processTranscription(context.Background(), db, hangingStorage{&mockStorage{}}, stubTranscriber{available: true},
			"video-123", "recordings/u/v.webm", 0, "u", "tok", "auto", false)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("transcription job never gave up")
	}
	if !db.updated {
		t.Error("want the timed-out job marked failed")
	}
}

// The stale-job reset must not requeue a job that is still inside its
// deadline, or a second worker would start it again.
func TestProcessNextTranscription_ResetWaitsOutTheDeadline(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`transcript_started_at < now\(\) - make_interval\(secs => \$1\)`).
		WithArgs((transcriptionJobTimeout + time.Minute).Seconds()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	mock.ExpectQuery(`UPDATE videos SET transcript_status = 'processing'`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "file_key", "media_version", "user_id", "share_token", "language"}))

	processNextTranscription(context.Background(), mock, &mockStorage{}, stubTranscriber{available: true}, false)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
