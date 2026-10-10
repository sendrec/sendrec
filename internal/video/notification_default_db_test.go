package video

import (
	"context"
	"testing"
)

// New accounts start with view and comment emails on, whichever path creates
// them (signup, SSO, SCIM), so the first "they watched it" email isn't lost.
// Accounts that existed before the default changed keep having no row (off).
func TestNotificationDefault_DB_NewUsersGetViewsAndComments(t *testing.T) {
	pool := accountDB(t)
	ctx := context.Background()

	var id string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password, name) VALUES ('new@example.com', 'x', 'New') RETURNING id`,
	).Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var mode string
	if err := pool.QueryRow(ctx,
		`SELECT view_notification FROM notification_preferences WHERE user_id = $1`, id,
	).Scan(&mode); err != nil {
		t.Fatalf("read preferences: %v", err)
	}
	if mode != notificationModeViewsAndComments {
		t.Fatalf("mode = %q, want %q", mode, notificationModeViewsAndComments)
	}
}
