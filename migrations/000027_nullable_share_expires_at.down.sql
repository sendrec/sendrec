UPDATE videos SET share_expires_at = now() + INTERVAL '7 days' WHERE share_expires_at IS NULL;
ALTER TABLE videos ALTER COLUMN share_expires_at SET NOT NULL;
