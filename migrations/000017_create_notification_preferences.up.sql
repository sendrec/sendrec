CREATE TABLE notification_preferences (
    user_id UUID PRIMARY KEY REFERENCES users(id),
    view_notification TEXT NOT NULL DEFAULT 'off'
        CHECK (view_notification IN ('off', 'every', 'first', 'digest')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
