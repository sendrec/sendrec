ALTER TABLE videos DROP CONSTRAINT IF EXISTS videos_transcript_status_check;
ALTER TABLE videos ADD CONSTRAINT videos_transcript_status_check
    CHECK (transcript_status IN ('none', 'pending', 'processing', 'ready', 'failed', 'no_audio'));
