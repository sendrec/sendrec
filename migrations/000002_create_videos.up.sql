CREATE TABLE videos (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'uploading' CHECK (status IN ('uploading', 'ready', 'deleted')),
    duration    INTEGER NOT NULL DEFAULT 0,
    file_size   BIGINT NOT NULL DEFAULT 0,
    file_key    TEXT NOT NULL,
    share_token TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_videos_user_id ON videos(user_id);
CREATE INDEX idx_videos_share_token ON videos(share_token);
