package video

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func TestEnqueueTranscription(t *testing.T) {
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

func TestProcessNextTranscription_NoJobs(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Stuck job reset
	mock.ExpectExec(`UPDATE videos SET transcript_status = 'pending'`).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	// Claim query returns no rows
	mock.ExpectQuery(`UPDATE videos SET transcript_status = 'processing'`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "file_key", "user_id", "share_token"}))

	storage := &mockStorage{}
	processNextTranscription(context.Background(), mock, storage)

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
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// Claim query returns no rows (the reset job will be picked up next tick)
	mock.ExpectQuery(`UPDATE videos SET transcript_status = 'processing'`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "file_key", "user_id", "share_token"}))

	storage := &mockStorage{}
	processNextTranscription(context.Background(), mock, storage)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
