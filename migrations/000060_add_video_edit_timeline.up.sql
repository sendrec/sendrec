ALTER TABLE videos ADD COLUMN edit_timeline JSONB;
ALTER TABLE videos ADD COLUMN edit_render_status TEXT NOT NULL DEFAULT 'none'
    CHECK (edit_render_status IN ('none', 'processing', 'ready', 'failed'));
ALTER TABLE videos ADD COLUMN edit_render_error TEXT;
ALTER TABLE videos ADD COLUMN edit_render_video_id UUID REFERENCES videos(id) ON DELETE SET NULL;

