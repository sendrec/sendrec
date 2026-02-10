ALTER TABLE videos ADD COLUMN comment_mode TEXT NOT NULL DEFAULT 'disabled'
  CHECK (comment_mode IN ('disabled', 'anonymous', 'name_required', 'name_email_required'));
