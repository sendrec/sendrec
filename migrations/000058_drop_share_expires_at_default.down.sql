ALTER TABLE videos ALTER COLUMN share_expires_at SET DEFAULT now() + INTERVAL '7 days';
