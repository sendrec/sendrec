ALTER TABLE video_comments DROP CONSTRAINT IF EXISTS author_email_max;
ALTER TABLE video_comments DROP CONSTRAINT IF EXISTS author_name_max;
ALTER TABLE video_comments DROP COLUMN IF EXISTS video_timestamp_seconds;
