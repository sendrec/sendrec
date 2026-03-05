ALTER TABLE videos DROP COLUMN retention_warned_at;
ALTER TABLE videos DROP COLUMN pinned;
ALTER TABLE users DROP COLUMN retention_days;
ALTER TABLE organizations DROP COLUMN retention_days;
