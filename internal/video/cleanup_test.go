package video

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/pashagolub/pgxmock/v6"
)

func TestPurgeOrphanedFiles_DeletesUnpurgedFiles(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}

	mock.ExpectQuery(`SELECT id, file_key, thumbnail_key, webcam_key, transcript_key FROM videos`).
		WillReturnRows(
			pgxmock.NewRows(purgeColumns).
				AddRow("video-1", "recordings/user-1/abc.webm", (*string)(nil), (*string)(nil), (*string)(nil)).
				AddRow("video-2", "recordings/user-2/def.webm", (*string)(nil), (*string)(nil), (*string)(nil)),
		)

	mock.ExpectExec(`UPDATE videos SET file_purged_at`).
		WithArgs("video-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec(`UPDATE videos SET file_purged_at`).
		WithArgs("video-2").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	PurgeOrphanedFiles(context.Background(), mock, storage)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	if storage.deleteCalls() != 2 {
		t.Errorf("expected 2 delete calls, got %d", storage.deleteCalls())
	}
}

func TestPurgeOrphanedFiles_SkipsWhenNoOrphans(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}

	mock.ExpectQuery(`SELECT id, file_key, thumbnail_key, webcam_key, transcript_key FROM videos`).
		WillReturnRows(pgxmock.NewRows(purgeColumns))

	PurgeOrphanedFiles(context.Background(), mock, storage)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	if storage.deleteCalls() != 0 {
		t.Errorf("expected 0 delete calls, got %d", storage.deleteCalls())
	}
}

func TestPurgeOrphanedFiles_HandlesStorageFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{deleteErr: errors.New("s3 down")}

	mock.ExpectQuery(`SELECT id, file_key, thumbnail_key, webcam_key, transcript_key FROM videos`).
		WillReturnRows(
			pgxmock.NewRows(purgeColumns).
				AddRow("video-1", "recordings/user-1/abc.webm", (*string)(nil), (*string)(nil), (*string)(nil)),
		)

	// No UPDATE expectation — storage fails so purge mark should be skipped
	PurgeOrphanedFiles(context.Background(), mock, storage)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPurgeOrphanedFiles_HandlesDBQueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}

	mock.ExpectQuery(`SELECT id, file_key, thumbnail_key, webcam_key, transcript_key FROM videos`).
		WillReturnError(errors.New("connection refused"))

	// Should not panic
	PurgeOrphanedFiles(context.Background(), mock, storage)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPurgeOrphanedFiles_DeletesTranscriptFile(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}

	transcriptKey := "recordings/user-1/abc.vtt"
	mock.ExpectQuery(`SELECT id, file_key, thumbnail_key, webcam_key, transcript_key FROM videos`).
		WillReturnRows(
			pgxmock.NewRows(purgeColumns).
				AddRow("video-1", "recordings/user-1/abc.webm", (*string)(nil), (*string)(nil), &transcriptKey),
		)

	mock.ExpectExec(`UPDATE videos SET file_purged_at`).
		WithArgs("video-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	PurgeOrphanedFiles(context.Background(), mock, storage)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	if storage.deleteCalls() != 2 {
		t.Errorf("expected 2 delete calls (video + transcript), got %d", storage.deleteCalls())
	}
}

var purgeColumns = []string{"id", "file_key", "thumbnail_key", "webcam_key", "transcript_key"}

// A webcam recording that was never composited is still the row's object, so
// the sweep removes it with the rest. #296.
func TestPurgeOrphanedFiles_DeletesWebcamFile(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{deleteCalled: make(chan string, 4)}
	webcamKey := "recordings/user-1/abc_webcam.webm"
	mock.ExpectQuery(`SELECT id, file_key, thumbnail_key, webcam_key, transcript_key FROM videos`).
		WillReturnRows(pgxmock.NewRows(purgeColumns).
			AddRow("video-1", "recordings/user-1/abc.webm", (*string)(nil), &webcamKey, (*string)(nil)))
	mock.ExpectExec(`UPDATE videos SET file_purged_at = now\(\) WHERE id = \$1`).
		WithArgs("video-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	PurgeOrphanedFiles(context.Background(), mock, storage)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	close(storage.deleteCalled)
	var deleted []string
	for k := range storage.deleteCalled {
		deleted = append(deleted, k)
	}
	if !slices.Contains(deleted, webcamKey) {
		t.Errorf("want %s deleted, deleted %v", webcamKey, deleted)
	}
}

func TestPurgeVideoObjects_DeletesEveryKeyBeforeMarking(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}
	file, thumb, webcam, vtt := "f.webm", "f.jpg", "f_webcam.webm", "f.vtt"
	mock.ExpectExec(`UPDATE videos SET file_purged_at = now\(\) WHERE id = \$1`).
		WithArgs("video-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := purgeVideoObjects(context.Background(), mock, storage, "video-1", &file, &thumb, &webcam, &vtt, nil); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	if storage.deleteCalls() != 4 {
		t.Errorf("want 4 deletes, got %d", storage.deleteCalls())
	}
}

// One failed object still lets the others go. That the row then stays
// unpurged is checked against Postgres in cleanup_db_test.go.
func TestPurgeVideoObjects_FailedKeyStillDeletesTheRest(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	file, webcam, vtt := "f.webm", "f_webcam.webm", "f.vtt"
	storage := &mockStorage{deleteFailKey: webcam, deleteCalled: make(chan string, 8)}

	if err := purgeVideoObjects(context.Background(), mock, storage, "video-1", &file, &webcam, &vtt); err == nil {
		t.Fatal("want an error when the webcam object would not delete")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	close(storage.deleteCalled)
	var deleted []string
	for k := range storage.deleteCalled {
		deleted = append(deleted, k)
	}
	if !slices.Contains(deleted, vtt) {
		t.Errorf("want the transcript deleted despite the webcam failure, deleted %v", deleted)
	}
}

func TestAbandonStaleUploads_RetiresOldUploads(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`UPDATE videos SET status = 'deleted'`).
		WithArgs(staleUploadAgeHours).
		WillReturnResult(pgxmock.NewResult("UPDATE", 4))

	AbandonStaleUploads(context.Background(), mock)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestAbandonStaleUploads_NoStaleRows(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`UPDATE videos SET status = 'deleted'`).
		WithArgs(staleUploadAgeHours).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	AbandonStaleUploads(context.Background(), mock)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestAbandonStaleUploads_HandlesDBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`UPDATE videos SET status = 'deleted'`).
		WithArgs(staleUploadAgeHours).
		WillReturnError(errors.New("connection refused"))

	// Should not panic, and must not stop the surrounding cleanup tick.
	AbandonStaleUploads(context.Background(), mock)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
