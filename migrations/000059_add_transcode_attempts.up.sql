ALTER TABLE videos ADD COLUMN transcode_attempts INT NOT NULL DEFAULT 0;
ALTER TABLE videos ADD COLUMN transcode_error TEXT;
