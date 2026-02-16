ALTER TABLE videos
    DROP COLUMN IF EXISTS branding_company_name,
    DROP COLUMN IF EXISTS branding_logo_key,
    DROP COLUMN IF EXISTS branding_color_background,
    DROP COLUMN IF EXISTS branding_color_surface,
    DROP COLUMN IF EXISTS branding_color_text,
    DROP COLUMN IF EXISTS branding_color_accent,
    DROP COLUMN IF EXISTS branding_footer_text;
