CREATE TABLE user_branding (
    user_id          UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    company_name     TEXT,
    logo_key         TEXT,
    color_background TEXT,
    color_surface    TEXT,
    color_text       TEXT,
    color_accent     TEXT,
    footer_text      TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
