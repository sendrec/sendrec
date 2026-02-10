ALTER TABLE videos ADD COLUMN transcript_key TEXT;
ALTER TABLE videos ADD COLUMN transcript_json JSONB;
ALTER TABLE videos ADD COLUMN transcript_status TEXT NOT NULL DEFAULT 'none'
    CHECK (transcript_status IN ('none', 'processing', 'ready', 'failed'));
