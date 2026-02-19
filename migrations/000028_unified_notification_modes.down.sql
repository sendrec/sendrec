UPDATE notification_preferences
SET view_notification = CASE view_notification
    WHEN 'views_only' THEN 'every'
    WHEN 'views_and_comments' THEN 'every'
    WHEN 'comments_only' THEN 'off'
    WHEN 'digest' THEN 'digest'
    WHEN 'off' THEN 'off'
    WHEN 'every' THEN 'every'
    WHEN 'first' THEN 'first'
    ELSE 'off'
END;

ALTER TABLE notification_preferences
    DROP CONSTRAINT IF EXISTS notification_preferences_view_notification_check;

ALTER TABLE notification_preferences
    ADD CONSTRAINT notification_preferences_view_notification_check
    CHECK (view_notification IN ('off', 'every', 'first', 'digest'));

ALTER TABLE videos
    DROP CONSTRAINT IF EXISTS videos_view_notification_check;

ALTER TABLE videos
    ADD CONSTRAINT videos_view_notification_check
    CHECK (view_notification IS NULL OR view_notification IN ('off', 'every', 'first', 'digest'));
