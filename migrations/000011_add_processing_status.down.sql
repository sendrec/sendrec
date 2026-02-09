ALTER TABLE videos DROP CONSTRAINT videos_status_check;
ALTER TABLE videos ADD CONSTRAINT videos_status_check CHECK (status IN ('uploading', 'ready', 'deleted'));
