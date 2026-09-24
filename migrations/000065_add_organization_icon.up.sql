-- A short emoji or a couple of letters shown next to the workspace name in
-- menus and titles. NULL is the default icon. #261.
ALTER TABLE organizations ADD COLUMN icon TEXT;
