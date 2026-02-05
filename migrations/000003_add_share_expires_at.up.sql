ALTER TABLE videos ADD COLUMN share_expires_at TIMESTAMPTZ;
UPDATE videos SET share_expires_at = created_at + INTERVAL '7 days';
ALTER TABLE videos ALTER COLUMN share_expires_at SET NOT NULL;
ALTER TABLE videos ALTER COLUMN share_expires_at SET DEFAULT now() + INTERVAL '7 days';
