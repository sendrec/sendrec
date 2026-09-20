-- A branding preview is minted by an authenticated POST and rendered by a later
-- GET from the settings iframe. Holding it in process memory made the second
-- request depend on landing in the same process as the first, which nothing
-- guarantees behind more than one replica or across a restart.
CREATE TABLE branding_previews (
    id TEXT PRIMARY KEY,
    branding JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

-- Rows live for minutes and are swept whenever a new preview is written.
CREATE INDEX branding_previews_expires_at_idx ON branding_previews (expires_at);
