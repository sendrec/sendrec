package video

import (
	"context"
	"errors"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func TestPurgeOrphanedFiles_DeletesUnpurgedFiles(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}

	mock.ExpectQuery(`SELECT file_key, thumbnail_key, transcript_key FROM videos`).
		WillReturnRows(
			pgxmock.NewRows([]string{"file_key", "thumbnail_key", "transcript_key"}).
				AddRow("recordings/user-1/abc.webm", (*string)(nil), (*string)(nil)).
				AddRow("recordings/user-2/def.webm", (*string)(nil), (*string)(nil)),
		)

	mock.ExpectExec(`UPDATE videos SET file_purged_at`).
		WithArgs("recordings/user-1/abc.webm").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mock.ExpectExec(`UPDATE videos SET file_purged_at`).
		WithArgs("recordings/user-2/def.webm").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	PurgeOrphanedFiles(context.Background(), mock, storage)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	if storage.deleteCallCount != 2 {
		t.Errorf("expected 2 delete calls, got %d", storage.deleteCallCount)
	}
}

func TestPurgeOrphanedFiles_SkipsWhenNoOrphans(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{}

	mock.ExpectQuery(`SELECT file_key, thumbnail_key, transcript_key FROM videos`).
		WillReturnRows(pgxmock.NewRows([]string{"file_key", "thumbnail_key", "transcript_key"}))

	PurgeOrphanedFiles(context.Background(), mock, storage)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	if storage.deleteCallCount != 0 {
		t.Errorf("expected 0 delete calls, got %d", storage.deleteCallCount)
	}
}

func TestPurgeOrphanedFiles_HandlesStorageFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	storage := &mockStorage{deleteErr: errors.New("s3 down")}

	mock.ExpectQuery(`SELECT file_key, thumbnail_key, transcript_key FROM videos`).
		WillReturnRows(
			pgxmock.NewRows([]string{"file_key", "thumbnail_key", "transcript_key"}).
				AddRow("recordings/user-1/abc.webm", (*string)(nil), (*string)(nil)),
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

	mock.ExpectQuery(`SELECT file_key, thumbnail_key, transcript_key FROM videos`).
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
	mock.ExpectQuery(`SELECT file_key, thumbnail_key, transcript_key FROM videos`).
		WillReturnRows(
			pgxmock.NewRows([]string{"file_key", "thumbnail_key", "transcript_key"}).
				AddRow("recordings/user-1/abc.webm", (*string)(nil), &transcriptKey),
		)

	mock.ExpectExec(`UPDATE videos SET file_purged_at`).
		WithArgs("recordings/user-1/abc.webm").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	PurgeOrphanedFiles(context.Background(), mock, storage)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
	if storage.deleteCallCount != 2 {
		t.Errorf("expected 2 delete calls (video + transcript), got %d", storage.deleteCallCount)
	}
}
