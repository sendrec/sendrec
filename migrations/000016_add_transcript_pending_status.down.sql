UPDATE videos SET transcript_status = 'none' WHERE transcript_status = 'pending';

DROP INDEX IF EXISTS idx_videos_transcript_pending;
ALTER TABLE videos DROP COLUMN IF EXISTS transcript_started_at;

ALTER TABLE videos DROP CONSTRAINT IF EXISTS videos_transcript_status_check;
ALTER TABLE videos ADD CONSTRAINT videos_transcript_status_check
    CHECK (transcript_status IN ('none', 'processing', 'ready', 'failed'));
