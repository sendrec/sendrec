ALTER TABLE videos DROP CONSTRAINT IF EXISTS videos_transcript_status_check;
ALTER TABLE videos ADD CONSTRAINT videos_transcript_status_check
    CHECK (transcript_status IN ('none', 'pending', 'processing', 'ready', 'failed'));

ALTER TABLE videos ADD COLUMN transcript_started_at TIMESTAMPTZ;

CREATE INDEX idx_videos_transcript_pending ON videos (transcript_status)
    WHERE transcript_status IN ('pending', 'processing');
