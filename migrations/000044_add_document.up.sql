ALTER TABLE videos ADD COLUMN document TEXT;
ALTER TABLE videos ADD COLUMN document_status TEXT NOT NULL DEFAULT 'none';
ALTER TABLE videos ADD COLUMN document_started_at TIMESTAMPTZ;
