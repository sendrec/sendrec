DROP TABLE IF EXISTS webhook_deliveries;
ALTER TABLE notification_preferences
    DROP COLUMN IF EXISTS webhook_url,
    DROP COLUMN IF EXISTS webhook_secret;
