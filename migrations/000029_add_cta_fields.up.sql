ALTER TABLE videos ADD COLUMN cta_text TEXT DEFAULT NULL;
ALTER TABLE videos ADD COLUMN cta_url TEXT DEFAULT NULL;

CREATE TABLE cta_clicks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    video_id UUID NOT NULL REFERENCES videos(id),
    viewer_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cta_clicks_video_id ON cta_clicks(video_id);
