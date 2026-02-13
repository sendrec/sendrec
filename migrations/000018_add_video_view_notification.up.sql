ALTER TABLE videos ADD COLUMN view_notification TEXT DEFAULT NULL
    CHECK (view_notification IN ('off', 'every', 'first', 'digest'));
