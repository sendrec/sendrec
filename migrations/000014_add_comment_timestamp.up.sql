ALTER TABLE video_comments ADD COLUMN video_timestamp_seconds REAL;
ALTER TABLE video_comments ADD CONSTRAINT author_name_max CHECK (length(author_name) <= 200);
ALTER TABLE video_comments ADD CONSTRAINT author_email_max CHECK (length(author_email) <= 320);
