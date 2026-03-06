ALTER TABLE organization_sso_configs DROP COLUMN saml_metadata_xml;
ALTER TABLE organization_sso_configs DROP COLUMN saml_certificate;
ALTER TABLE organization_sso_configs DROP COLUMN saml_sso_url;
ALTER TABLE organization_sso_configs DROP COLUMN saml_entity_id;
ALTER TABLE organization_sso_configs DROP COLUMN saml_metadata_url;

UPDATE organization_sso_configs SET issuer_url = '' WHERE issuer_url IS NULL;
UPDATE organization_sso_configs SET client_id = '' WHERE client_id IS NULL;
UPDATE organization_sso_configs SET client_secret_encrypted = '' WHERE client_secret_encrypted IS NULL;
ALTER TABLE organization_sso_configs ALTER COLUMN issuer_url SET NOT NULL;
ALTER TABLE organization_sso_configs ALTER COLUMN client_id SET NOT NULL;
ALTER TABLE organization_sso_configs ALTER COLUMN client_secret_encrypted SET NOT NULL;
