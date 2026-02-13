package video

import (
	"context"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
)

func TestProcessDigest_SendsEmails(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	notifier := &mockViewNotifier{}

	mock.ExpectQuery(`SELECT v\.id, v\.title, v\.share_token, v\.user_id, u\.email, u\.name`).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "title", "share_token", "user_id", "email", "name", "view_count",
		}).AddRow(
			"vid-1", "Video One", "tok-1", "user-1", "alice@example.com", "Alice", int64(10),
		).AddRow(
			"vid-2", "Video Two", "tok-2", "user-1", "alice@example.com", "Alice", int64(3),
		))

	processDigest(context.Background(), mock, notifier, "https://app.sendrec.eu")

	if !notifier.called {
		t.Error("expected notifier to be called")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessDigest_NoViewsNoEmail(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	notifier := &mockViewNotifier{}

	mock.ExpectQuery(`SELECT v\.id, v\.title, v\.share_token, v\.user_id, u\.email, u\.name`).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "title", "share_token", "user_id", "email", "name", "view_count",
		}))

	processDigest(context.Background(), mock, notifier, "https://app.sendrec.eu")

	if notifier.called {
		t.Error("expected no notification when no digest views")
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
