CREATE TABLE view_milestones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    video_id UUID NOT NULL REFERENCES videos(id),
    viewer_hash TEXT NOT NULL,
    milestone INTEGER NOT NULL CHECK (milestone IN (25, 50, 75, 100)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (video_id, viewer_hash, milestone)
);
CREATE INDEX idx_view_milestones_video_id ON view_milestones(video_id);
