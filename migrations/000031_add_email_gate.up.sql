ALTER TABLE videos ADD COLUMN email_gate_enabled BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE video_viewers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    video_id UUID NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    viewer_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(video_id, email)
);
CREATE INDEX idx_video_viewers_video_id ON video_viewers(video_id);
