CREATE TABLE creem_webhook_events (
    event_id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    user_id UUID,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
