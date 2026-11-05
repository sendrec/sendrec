package video

import (
	"context"
	"errors"
	"testing"

	"github.com/pashagolub/pgxmock/v6"
)

func TestResetStuckProcessing_ResetsAbandonedRows(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`UPDATE videos SET status = 'ready'`).
		WithArgs(webcamDroppedWarning, compositeExtraCapSeconds).
		WillReturnRows(stuckRows().AddRow("v1", (*string)(nil), "u1", "tok", 60).AddRow("v2", (*string)(nil), "u1", "tok", 60))

	resetStuckProcessing(context.Background(), mock, &mockStorage{}, videoReadyHook{})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func stuckRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "webcam_key", "user_id", "share_token", "duration"})
}

// The whole point of the sweep is to undo what setReadyFallback would have done
// had the process survived: the webcam of an abandoned composite is deleted and
// only then is its key cleared, so the object is never left with nothing
// pointing at it. #296.
func TestResetStuckProcessing_DeletesAbandonedWebcam(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	webcam := "recordings/u/v_webcam.webm"
	mock.ExpectQuery(`processing_started_at = NULL`).
		WithArgs(webcamDroppedWarning, compositeExtraCapSeconds).
		WillReturnRows(stuckRows().AddRow("v1", &webcam, "u1", "tok", 60))
	mock.ExpectExec(`UPDATE videos SET webcam_key = NULL WHERE id = \$1 AND webcam_key = \$2`).
		WithArgs("v1", webcam).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	storage := &mockStorage{deleteCalled: make(chan string, 2)}
	resetStuckProcessing(context.Background(), mock, storage, videoReadyHook{})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	close(storage.deleteCalled)
	if k := <-storage.deleteCalled; k != webcam {
		t.Errorf("want %s deleted, got %q", webcam, k)
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

	mock.ExpectQuery(`processing_started_at < now\(\) - INTERVAL '15 minutes'`).
		WithArgs(webcamDroppedWarning, compositeExtraCapSeconds).
		WillReturnRows(stuckRows())

	resetStuckProcessing(context.Background(), mock, &mockStorage{}, videoReadyHook{})

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

	mock.ExpectQuery(`processing_started_at IS NULL`).
		WithArgs(webcamDroppedWarning, compositeExtraCapSeconds).
		WillReturnRows(stuckRows().AddRow("v1", (*string)(nil), "u1", "tok", 60))

	resetStuckProcessing(context.Background(), mock, &mockStorage{}, videoReadyHook{})

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

	mock.ExpectQuery(`UPDATE videos SET status = 'ready'`).
		WithArgs(webcamDroppedWarning, compositeExtraCapSeconds).
		WillReturnError(errors.New("connection refused"))

	resetStuckProcessing(context.Background(), mock, &mockStorage{}, videoReadyHook{})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// A composite may legitimately run past the flat deadline: its own timeout
// grows with the recording. The sweep waits as long for rows that still have
// a webcam to composite, and tells the owner when it drops one. #283.
func TestResetStuckProcessing_GivesCompositesTheirLongerDeadline(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`CASE WHEN webcam_key IS NOT NULL THEN make_interval\(secs => LEAST\(2 \* duration, \$2\)\)`).
		WithArgs(webcamDroppedWarning, compositeExtraCapSeconds).
		WillReturnRows(stuckRows())

	resetStuckProcessing(context.Background(), mock, &mockStorage{}, videoReadyHook{})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
