ALTER TABLE user_branding DROP COLUMN IF EXISTS organization_id;
ALTER TABLE tags DROP COLUMN IF EXISTS organization_id;
ALTER TABLE folders DROP COLUMN IF EXISTS organization_id;
DROP INDEX IF EXISTS idx_videos_organization_id;
ALTER TABLE videos DROP COLUMN IF EXISTS organization_id;
DROP TABLE IF EXISTS organization_invites;
DROP TABLE IF EXISTS organization_members;
DROP TABLE IF EXISTS organizations;
