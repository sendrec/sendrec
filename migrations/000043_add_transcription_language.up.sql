ALTER TABLE users ADD COLUMN transcription_language TEXT NOT NULL DEFAULT 'auto';
ALTER TABLE videos ADD COLUMN transcription_language TEXT;
