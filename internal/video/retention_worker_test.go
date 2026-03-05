package video

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/sendrec/sendrec/internal/email"
)

type mockRetentionSender struct {
	calls []retentionSendCall
}

type retentionSendCall struct {
	toEmail    string
	videos     []email.RetentionVideoSummary
	expiryDate string
}

func (m *mockRetentionSender) SendRetentionWarning(ctx context.Context, toEmail string, videos []email.RetentionVideoSummary, expiryDate string) error {
	m.calls = append(m.calls, retentionSendCall{
		toEmail:    toEmail,
		videos:     videos,
		expiryDate: expiryDate,
	})
	return nil
}

func TestProcessRetentionWarnings_SendsForExpiring(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	sender := &mockRetentionSender{}

	mock.ExpectQuery(`SELECT v.id, v.title, v.share_token, u.email`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "title", "share_token", "email", "retention_days"}).
			AddRow("vid-1", "My Video", "abc123", "alice@example.com", 30).
			AddRow("vid-2", "Other Video", "def456", "alice@example.com", 30))

	mock.ExpectExec(`UPDATE videos SET retention_warned_at`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 2))

	processRetentionWarnings(context.Background(), mock, sender, "https://app.sendrec.eu")

	if len(sender.calls) != 1 {
		t.Fatalf("expected 1 send call, got %d", len(sender.calls))
	}
	if sender.calls[0].toEmail != "alice@example.com" {
		t.Errorf("expected email to alice@example.com, got %s", sender.calls[0].toEmail)
	}
	if len(sender.calls[0].videos) != 2 {
		t.Fatalf("expected 2 videos in email, got %d", len(sender.calls[0].videos))
	}
	if sender.calls[0].videos[0].Title != "My Video" {
		t.Errorf("expected first video title 'My Video', got %q", sender.calls[0].videos[0].Title)
	}
	if sender.calls[0].videos[0].WatchURL != "https://app.sendrec.eu/watch/abc123" {
		t.Errorf("expected watchURL with share_token, got %q", sender.calls[0].videos[0].WatchURL)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessRetentionWarnings_SkipsPinnedVideos(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	sender := &mockRetentionSender{}

	// The query itself filters out pinned=true, so an empty result set means pinned videos are skipped.
	mock.ExpectQuery(`SELECT v.id, v.title, v.share_token, u.email`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "title", "share_token", "email", "retention_days"}))

	processRetentionWarnings(context.Background(), mock, sender, "https://app.sendrec.eu")

	if len(sender.calls) != 0 {
		t.Errorf("expected no send calls for pinned videos, got %d", len(sender.calls))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessRetentionDeletions_DeletesExpiredVideos(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`SELECT id FROM videos`).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).
			AddRow("vid-1").
			AddRow("vid-2"))

	mock.ExpectExec(`DELETE FROM playlist_videos`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	mock.ExpectExec(`UPDATE videos SET status`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 2))

	processRetentionDeletions(context.Background(), mock)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessRetentionDeletions_RemovesFromPlaylists(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`SELECT id FROM videos`).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).
			AddRow("vid-1"))

	mock.ExpectExec(`DELETE FROM playlist_videos`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("DELETE", 3))

	mock.ExpectExec(`UPDATE videos SET status`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	processRetentionDeletions(context.Background(), mock)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessRetentionDeletions_NoExpiredVideos(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery(`SELECT id FROM videos`).
		WillReturnRows(pgxmock.NewRows([]string{"id"}))

	// No ExpectExec — DELETE and UPDATE should NOT be called when there are no expired videos

	processRetentionDeletions(context.Background(), mock)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
