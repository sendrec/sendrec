package video

import (
	"context"
	"testing"
)

// The day-7 upgrade email goes to free users who have recorded something.
// Someone who never recorded has nothing to upgrade from; day 2 covers them.
func TestProcessOnboardingDay7_DB_OnlyUsersWhoRecorded(t *testing.T) {
	pool := accountDB(t)
	recorder := mustID(t, pool, `INSERT INTO users (email, password, name, email_verified, created_at)
		VALUES ('recorder@example.com', 'x', 'R', true, now() - interval '8 days') RETURNING id`)
	mustID(t, pool, `INSERT INTO users (email, password, name, email_verified, created_at)
		VALUES ('idle@example.com', 'x', 'I', true, now() - interval '8 days') RETURNING id`)
	seedVideo(t, pool, recorder, nil, "day7")

	sender := &mockOnboardingSender{}
	processOnboardingDay7(context.Background(), pool, sender, "https://app.example")

	if len(sender.day7Calls) != 1 || sender.day7Calls[0] != "recorder@example.com" {
		t.Fatalf("day7 sent to %v, want only recorder@example.com", sender.day7Calls)
	}
}
