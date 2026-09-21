-- A recording can arrive with its video already dead: the browser stops
-- capturing when its window is hidden and the microphone carries on, so the
-- file is a full length soundtrack over one still image. The probe job samples
-- for that and leaves the reason here, because nothing else about the row shows
-- it — status, duration and file size all look like an ordinary recording.
ALTER TABLE videos ADD COLUMN capture_warning TEXT;
