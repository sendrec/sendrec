package video

import (
	"context"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
)

func TestProcessDigest_SendsSingleEmailPerUser(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	notifier := &mockViewNotifier{}

	mock.ExpectQuery(`SELECT v\.id, v\.title, v\.share_token, v\.user_id, u\.email, u\.name`).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "title", "share_token", "user_id", "email", "name", "view_count", "comment_count",
		}).AddRow(
			"vid-1", "Video One", "tok-1", "user-1", "alice@example.com", "Alice", int64(10), int64(2),
		).AddRow(
			"vid-2", "Video Two", "tok-2", "user-1", "alice@example.com", "Alice", int64(3), int64(0),
		))

	processDigest(context.Background(), mock, notifier, "https://app.sendrec.eu")

	if !notifier.digestCalled {
		t.Fatal("expected digest notifier to be called")
	}

	if len(notifier.digestVideos) != 2 {
		t.Fatalf("expected 2 videos in digest, got %d", len(notifier.digestVideos))
	}
	if notifier.digestVideos[0].Title != "Video One" {
		t.Errorf("expected first video title 'Video One', got %q", notifier.digestVideos[0].Title)
	}
	if notifier.digestVideos[0].ViewCount != 10 {
		t.Errorf("expected first video 10 views, got %d", notifier.digestVideos[0].ViewCount)
	}
	if notifier.digestVideos[0].CommentCount != 2 {
		t.Errorf("expected first video 2 comments, got %d", notifier.digestVideos[0].CommentCount)
	}
	if notifier.digestVideos[1].Title != "Video Two" {
		t.Errorf("expected second video title 'Video Two', got %q", notifier.digestVideos[1].Title)
	}
	if notifier.digestVideos[1].ViewCount != 3 {
		t.Errorf("expected second video 3 views, got %d", notifier.digestVideos[1].ViewCount)
	}
	if notifier.toEmail != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", notifier.toEmail)
	}

	if notifier.called {
		t.Error("expected SendViewNotification NOT to be called for digest")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessDigest_MultipleUsers(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	callCount := 0
	notifier := &countingDigestNotifier{callCount: &callCount}

	mock.ExpectQuery(`SELECT v\.id, v\.title, v\.share_token, v\.user_id, u\.email, u\.name`).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "title", "share_token", "user_id", "email", "name", "view_count", "comment_count",
		}).AddRow(
			"vid-1", "Video One", "tok-1", "user-1", "alice@example.com", "Alice", int64(5), int64(1),
		).AddRow(
			"vid-2", "Video Two", "tok-2", "user-2", "bob@example.com", "Bob", int64(8), int64(0),
		).AddRow(
			"vid-3", "Video Three", "tok-3", "user-1", "alice@example.com", "Alice", int64(2), int64(3),
		))

	processDigest(context.Background(), mock, notifier, "https://app.sendrec.eu")

	if callCount != 2 {
		t.Errorf("expected 2 digest emails (one per user), got %d", callCount)
	}
}

func TestProcessDigest_NoActivityNoEmail(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	notifier := &mockViewNotifier{}

	mock.ExpectQuery(`SELECT v\.id, v\.title, v\.share_token, v\.user_id, u\.email, u\.name`).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "title", "share_token", "user_id", "email", "name", "view_count", "comment_count",
		}))

	processDigest(context.Background(), mock, notifier, "https://app.sendrec.eu")

	if notifier.digestCalled {
		t.Error("expected no digest notification when there is no activity")
	}
}

func TestProcessDigest_CommentsOnlyStillSendsDigest(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	notifier := &mockViewNotifier{}

	mock.ExpectQuery(`SELECT v\.id, v\.title, v\.share_token, v\.user_id, u\.email, u\.name`).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "title", "share_token", "user_id", "email", "name", "view_count", "comment_count",
		}).AddRow(
			"vid-1", "Video One", "tok-1", "user-1", "alice@example.com", "Alice", int64(0), int64(4),
		))

	processDigest(context.Background(), mock, notifier, "https://app.sendrec.eu")

	if !notifier.digestCalled {
		t.Fatal("expected digest notification for comments-only activity")
	}
	if len(notifier.digestVideos) != 1 {
		t.Fatalf("expected 1 digest row, got %d", len(notifier.digestVideos))
	}
	if notifier.digestVideos[0].ViewCount != 0 {
		t.Fatalf("expected 0 views in digest row, got %d", notifier.digestVideos[0].ViewCount)
	}
	if notifier.digestVideos[0].CommentCount != 4 {
		t.Fatalf("expected 4 comments in digest row, got %d", notifier.digestVideos[0].CommentCount)
	}
}

func TestDurationUntilNextRun(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		expected time.Duration
	}{
		{
			name:     "before 9am",
			now:      time.Date(2026, 2, 13, 8, 0, 0, 0, time.UTC),
			expected: 1 * time.Hour,
		},
		{
			name:     "after 9am",
			now:      time.Date(2026, 2, 13, 10, 0, 0, 0, time.UTC),
			expected: 23 * time.Hour,
		},
		{
			name:     "exactly 9am",
			now:      time.Date(2026, 2, 13, 9, 0, 0, 0, time.UTC),
			expected: 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := durationUntilNextRun(tt.now)
			if d != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, d)
			}
		})
	}
}
