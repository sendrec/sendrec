DROP INDEX IF EXISTS idx_email_confirmations_user_id;
DROP TABLE IF EXISTS email_confirmations;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
