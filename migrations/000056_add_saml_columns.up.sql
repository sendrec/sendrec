-- Make OIDC-specific columns nullable for SAML configs
ALTER TABLE organization_sso_configs ALTER COLUMN issuer_url DROP NOT NULL;
ALTER TABLE organization_sso_configs ALTER COLUMN client_id DROP NOT NULL;
ALTER TABLE organization_sso_configs ALTER COLUMN client_secret_encrypted DROP NOT NULL;

-- Add SAML columns
ALTER TABLE organization_sso_configs ADD COLUMN saml_metadata_url TEXT;
ALTER TABLE organization_sso_configs ADD COLUMN saml_entity_id TEXT;
ALTER TABLE organization_sso_configs ADD COLUMN saml_sso_url TEXT;
ALTER TABLE organization_sso_configs ADD COLUMN saml_certificate TEXT;
ALTER TABLE organization_sso_configs ADD COLUMN saml_metadata_xml TEXT;
