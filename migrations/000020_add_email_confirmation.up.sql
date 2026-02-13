ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT false;

UPDATE users SET email_verified = true;

CREATE TABLE email_confirmations (
    token_hash  TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_email_confirmations_user_id ON email_confirmations(user_id);
