package video

import (
	"context"
	"errors"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func TestResetStuckProcessing_ResetsAbandonedRows(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`UPDATE videos SET status = 'ready'`).
		WillReturnResult(pgxmock.NewResult("UPDATE", 2))

	resetStuckProcessing(context.Background(), mock)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// The whole point of the sweep is to undo what setReadyFallback would have done
// had the process survived: composite's fallback also drops webcam_key, so a row
// abandoned mid-composite must not keep pointing at a webcam file that will never
// be overlaid.
func TestResetStuckProcessing_ClearsWebcamKeyAndStartedAt(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`webcam_key = NULL`).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	resetStuckProcessing(context.Background(), mock)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// processing_started_at is the ownership clock. Keying off updated_at instead
// would let any unrelated write to the row push the deadline out forever.
func TestResetStuckProcessing_KeysOffProcessingStartedAt(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`processing_started_at < now\(\) - INTERVAL '15 minutes'`).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	resetStuckProcessing(context.Background(), mock)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// Rows stuck before this migration shipped have no processing_started_at at all.
// They are exactly the rows the bug report is about, so they must be swept too.
func TestResetStuckProcessing_SweepsRowsWithNullStartedAt(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`processing_started_at IS NULL`).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	resetStuckProcessing(context.Background(), mock)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestResetStuckProcessing_SurvivesQueryFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`UPDATE videos SET status = 'ready'`).
		WillReturnError(errors.New("connection refused"))

	resetStuckProcessing(context.Background(), mock)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
