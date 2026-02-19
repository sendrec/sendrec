UPDATE notification_preferences
SET view_notification = CASE view_notification
    WHEN 'every' THEN 'views_only'
    WHEN 'first' THEN 'views_only'
    WHEN 'off' THEN 'off'
    WHEN 'digest' THEN 'digest'
    WHEN 'views_only' THEN 'views_only'
    WHEN 'comments_only' THEN 'comments_only'
    WHEN 'views_and_comments' THEN 'views_and_comments'
    ELSE 'off'
END;

ALTER TABLE notification_preferences
    DROP CONSTRAINT IF EXISTS notification_preferences_view_notification_check;

ALTER TABLE notification_preferences
    ADD CONSTRAINT notification_preferences_view_notification_check
    CHECK (view_notification IN ('off', 'views_only', 'comments_only', 'views_and_comments', 'digest'));

UPDATE videos
SET view_notification = 'every'
WHERE view_notification = 'first';

ALTER TABLE videos
    DROP CONSTRAINT IF EXISTS videos_view_notification_check;

ALTER TABLE videos
    ADD CONSTRAINT videos_view_notification_check
    CHECK (view_notification IS NULL OR view_notification IN ('off', 'every', 'digest'));
