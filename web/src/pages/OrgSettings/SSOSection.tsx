import { type FormEvent, useState } from "react";
import { apiFetch } from "../../api/client";
import type { SharedSectionProps, SsoConfig } from "./types";

interface SSOSectionProps extends SharedSectionProps {
  ssoConfigured: boolean;
  setSsoConfigured: React.Dispatch<React.SetStateAction<boolean>>;
  ssoProvider: "oidc" | "saml";
  setSsoProvider: React.Dispatch<React.SetStateAction<"oidc" | "saml">>;
  ssoIssuerUrl: string;
  setSsoIssuerUrl: React.Dispatch<React.SetStateAction<string>>;
  ssoClientId: string;
  setSsoClientId: React.Dispatch<React.SetStateAction<string>>;
  ssoEnforce: boolean;
  setSsoEnforce: React.Dispatch<React.SetStateAction<boolean>>;
  samlMetadataUrl: string;
  setSamlMetadataUrl: React.Dispatch<React.SetStateAction<string>>;
  samlEntityId: string;
  setSamlEntityId: React.Dispatch<React.SetStateAction<string>>;
  samlSsoUrl: string;
  setSamlSsoUrl: React.Dispatch<React.SetStateAction<string>>;
  spMetadataUrl: string;
  setSpMetadataUrl: React.Dispatch<React.SetStateAction<string>>;
  scimConfigured: boolean;
  setScimConfigured: React.Dispatch<React.SetStateAction<boolean>>;
  scimCreatedAt: string;
  setScimCreatedAt: React.Dispatch<React.SetStateAction<string>>;
  scimToken: string;
  setScimToken: React.Dispatch<React.SetStateAction<string>>;
  scimError: string;
  setScimError: React.Dispatch<React.SetStateAction<string>>;
  scimMessage: string;
  setScimMessage: React.Dispatch<React.SetStateAction<string>>;
  scimGenerating: boolean;
  setScimGenerating: React.Dispatch<React.SetStateAction<boolean>>;
}

export function SSOSection({
  orgId,
  setConfirmDialog,
  ssoConfigured,
  setSsoConfigured,
  ssoProvider,
  setSsoProvider,
  ssoIssuerUrl,
  setSsoIssuerUrl,
  ssoClientId,
  setSsoClientId,
  ssoEnforce,
  setSsoEnforce,
  samlMetadataUrl,
  setSamlMetadataUrl,
  samlEntityId,
  setSamlEntityId,
  samlSsoUrl,
  setSamlSsoUrl,
  spMetadataUrl,
  setSpMetadataUrl,
  scimConfigured,
  setScimConfigured,
  scimCreatedAt,
  setScimCreatedAt,
  scimToken,
  setScimToken,
  scimError,
  setScimError,
  scimMessage,
  setScimMessage,
  scimGenerating,
  setScimGenerating,
}: SSOSectionProps) {
  const [ssoClientSecret, setSsoClientSecret] = useState("");
  const [samlMetadataXml, setSamlMetadataXml] = useState("");
  const [ssoMessage, setSsoMessage] = useState("");
  const [ssoError, setSsoError] = useState("");
  const [savingSso, setSavingSso] = useState(false);
  const [removingSso, setRemovingSso] = useState(false);

  async function handleSsoSave(event: FormEvent) {
    event.preventDefault();
    setSsoError("");
    setSsoMessage("");
    setSavingSso(true);
    try {
      const body = ssoProvider === "saml"
        ? {
            provider: "saml",
            samlMetadataUrl: samlMetadataUrl.trim() || undefined,
            samlMetadataXml: samlMetadataXml.trim() || undefined,
            enforceSso: ssoEnforce,
          }
        : {
            provider: "oidc",
            issuerUrl: ssoIssuerUrl.trim(),
            clientId: ssoClientId.trim(),
            clientSecret: ssoClientSecret || undefined,
            enforceSso: ssoEnforce,
          };
      await apiFetch(`/api/organizations/${orgId}/sso`, {
        method: "PUT",
        body: JSON.stringify(body),
      });
      setSsoMessage("SSO settings saved");
      setSsoConfigured(true);
      setSsoClientSecret("");
      setSamlMetadataXml("");
      // Reload to get parsed fields
      const ssoData = await apiFetch<SsoConfig>(`/api/organizations/${orgId}/sso`);
      if (ssoData?.provider === "saml") {
        setSamlEntityId(ssoData.samlEntityId || "");
        setSamlSsoUrl(ssoData.samlSsoUrl || "");
        setSpMetadataUrl(ssoData.spMetadataUrl || "");
      }
    } catch (err) {
      setSsoError(err instanceof Error ? err.message : "Failed to save SSO settings");
    } finally {
      setSavingSso(false);
    }
  }

  function handleRemoveSso() {
    setConfirmDialog({
      message: "Remove SSO configuration? Members will need to use password login.",
      confirmLabel: "Remove SSO",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        setRemovingSso(true);
        setSsoError("");
        setSsoMessage("");
        try {
          await apiFetch(`/api/organizations/${orgId}/sso`, { method: "DELETE" });
          setSsoIssuerUrl("");
          setSsoClientId("");
          setSsoClientSecret("");
          setSsoEnforce(false);
          setSsoConfigured(false);
          setSsoProvider("oidc");
          setSamlMetadataUrl("");
          setSamlMetadataXml("");
          setSamlEntityId("");
          setSamlSsoUrl("");
          setSpMetadataUrl("");
          setSsoMessage("SSO configuration removed");
        } catch (err) {
          setSsoError(err instanceof Error ? err.message : "Failed to remove SSO");
        } finally {
          setRemovingSso(false);
        }
      },
    });
  }

  async function handleGenerateScimToken() {
    setScimError("");
    setScimMessage("");
    setScimToken("");
    setScimGenerating(true);
    try {
      const resp = await apiFetch<{ token: string }>(
        `/api/organizations/${orgId}/scim-token`,
        { method: "POST" }
      );
      if (resp) {
        setScimToken(resp.token);
        setScimConfigured(true);
        setScimCreatedAt(new Date().toISOString());
        setScimMessage("Token generated. Copy it now — it won't be shown again.");
      }
    } catch (err) {
      setScimError(err instanceof Error ? err.message : "Failed to generate token");
    } finally {
      setScimGenerating(false);
    }
  }

  function handleRegenerateScimToken() {
    setConfirmDialog({
      message: "Regenerate SCIM token? The current token will stop working immediately.",
      confirmLabel: "Regenerate",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        await handleGenerateScimToken();
      },
    });
  }

  function handleRevokeScimToken() {
    setConfirmDialog({
      message: "Revoke SCIM token? Automated provisioning will stop working.",
      confirmLabel: "Revoke",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        setScimError("");
        try {
          await apiFetch(`/api/organizations/${orgId}/scim-token`, { method: "DELETE" });
          setScimConfigured(false);
          setScimToken("");
          setScimCreatedAt("");
          setScimMessage("SCIM token revoked");
        } catch (err) {
          setScimError(err instanceof Error ? err.message : "Failed to revoke token");
        }
      },
    });
  }

  return (
    <>
      <form onSubmit={handleSsoSave} className="card settings-section">
        <h2>Single Sign-On</h2>
        <p className="card-description">
          Configure single sign-on for your workspace. Members can sign in using your identity provider.
        </p>

        <div className="form-field">
          <label className="form-label">Protocol</label>
          <div style={{ display: "flex", gap: "1rem" }}>
            <label style={{ display: "flex", alignItems: "center", gap: "0.25rem" }}>
              <input
                type="radio"
                name="sso-protocol"
                value="oidc"
                checked={ssoProvider === "oidc"}
                onChange={() => setSsoProvider("oidc")}
              />
              OIDC
            </label>
            <label style={{ display: "flex", alignItems: "center", gap: "0.25rem" }}>
              <input
                type="radio"
                name="sso-protocol"
                value="saml"
                checked={ssoProvider === "saml"}
                onChange={() => setSsoProvider("saml")}
              />
              SAML
            </label>
          </div>
        </div>

        {ssoProvider === "oidc" ? (
          <>
            <div className="form-field">
              <label className="form-label" htmlFor="sso-issuer-url">Issuer URL</label>
              <input
                id="sso-issuer-url"
                type="url"
                className="form-input"
                value={ssoIssuerUrl}
                onChange={(e) => setSsoIssuerUrl(e.target.value)}
                placeholder="https://accounts.google.com"
              />
            </div>

            <div className="form-field">
              <label className="form-label" htmlFor="sso-client-id">Client ID</label>
              <input
                id="sso-client-id"
                type="text"
                className="form-input"
                value={ssoClientId}
                onChange={(e) => setSsoClientId(e.target.value)}
              />
            </div>

            <div className="form-field">
              <label className="form-label" htmlFor="sso-client-secret">Client Secret</label>
              <input
                id="sso-client-secret"
                type="password"
                className="form-input"
                value={ssoClientSecret}
                onChange={(e) => setSsoClientSecret(e.target.value)}
                placeholder={ssoConfigured ? "Unchanged" : ""}
              />
            </div>
            <details className="settings-details">
              <summary>OIDC setup guide</summary>
              <pre>{`1. In your IdP (Google, Okta, Auth0, Azure AD, etc.),
   create an OAuth2/OpenID Connect application.

2. Set the redirect URI to:
   ${window.location.origin}/api/auth/sso/org/callback

3. Copy the following into the fields above:
   • Issuer URL — your IdP's OpenID discovery URL
     (e.g. https://accounts.google.com)
   • Client ID — from the application you created
   • Client Secret — from the application you created

4. Click "Save SSO settings" and test the login flow.

Common issuer URLs:
  Google:   https://accounts.google.com
  Okta:     https://your-org.okta.com
  Auth0:    https://your-tenant.auth0.com
  Azure AD: https://login.microsoftonline.com/{tenant}/v2.0`}</pre>
            </details>
          </>
        ) : (
          <>
            <div className="form-field">
              <label className="form-label" htmlFor="saml-metadata-url">Metadata URL</label>
              <input
                id="saml-metadata-url"
                type="url"
                className="form-input"
                placeholder="https://your-idp.com/saml/metadata"
                value={samlMetadataUrl}
                onChange={(e) => setSamlMetadataUrl(e.target.value)}
              />
            </div>
            <div className="form-field">
              <label className="form-label" htmlFor="saml-metadata-xml">Or paste metadata XML</label>
              <textarea
                id="saml-metadata-xml"
                className="form-input"
                rows={4}
                placeholder="<EntityDescriptor ...>"
                value={samlMetadataXml}
                onChange={(e) => setSamlMetadataXml(e.target.value)}
              />
            </div>
            {samlEntityId && (
              <>
                <div className="form-field">
                  <label className="form-label">IdP Entity ID</label>
                  <input className="form-input" value={samlEntityId} readOnly />
                </div>
                <div className="form-field">
                  <label className="form-label">IdP SSO URL</label>
                  <input className="form-input" value={samlSsoUrl} readOnly />
                </div>
              </>
            )}
            {spMetadataUrl && (
              <div className="form-field">
                <label className="form-label">SP Metadata URL</label>
                <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
                  <input className="form-input" value={spMetadataUrl} readOnly style={{ flex: 1 }} />
                  <button
                    type="button"
                    className="btn btn--secondary btn--sm"
                    onClick={() => navigator.clipboard.writeText(spMetadataUrl)}
                  >
                    Copy
                  </button>
                </div>
                <small className="form-hint">Provide this URL to your IdP administrator</small>
              </div>
            )}
            <details className="settings-details">
              <summary>SAML setup guide</summary>
              <pre>{`1. In your IdP (Okta, Auth0, Azure AD, OneLogin, etc.),
   create a SAML 2.0 application.

2. Configure the IdP with these SP values:
   • SP Entity ID / Audience:
     ${window.location.origin}/api/auth/saml/${orgId}/metadata
   • ACS URL (Assertion Consumer Service):
     ${window.location.origin}/api/auth/saml/${orgId}/acs
   • NameID format: Email address
   ${spMetadataUrl ? `\n   Or import the SP Metadata URL shown above — your
   IdP will auto-configure from it.` : `\n   After saving, the SP Metadata URL will be shown
   above — your IdP can auto-configure from it.`}

3. From your IdP, copy the metadata URL and paste it
   in "Metadata URL" above. Or download the metadata
   XML and paste it in the text area.

4. Click "Save SSO settings". The IdP Entity ID and
   SSO URL will be extracted automatically.

5. Test the login flow by signing in with SSO.

Attribute mapping (sent in SAML assertion):
  email — required (NameID or attribute)
  name  — optional (displayName or name)`}</pre>
            </details>
          </>
        )}

        <div className="form-field" style={{ flexDirection: "row", alignItems: "center", gap: 8 }}>
          <input
            id="sso-enforce"
            type="checkbox"
            checked={ssoEnforce}
            onChange={(e) => setSsoEnforce(e.target.checked)}
            style={{ width: "auto" }}
          />
          <label htmlFor="sso-enforce" className="form-label" style={{ margin: 0 }}>
            Enforce SSO for all members
          </label>
        </div>
        {ssoEnforce && (
          <p className="form-hint" style={{ color: "var(--color-warning)" }}>
            When enforced, members must sign in through your identity provider. Password login will be disabled for workspace members.
          </p>
        )}

        {ssoError && (
          <p className="status-message status-message--error">{ssoError}</p>
        )}
        {ssoMessage && (
          <p className="status-message status-message--success">{ssoMessage}</p>
        )}

        <div className="btn-row">
          <button
            type="submit"
            className="btn btn--primary"
            disabled={
              savingSso ||
              (ssoProvider === "oidc" && (!ssoIssuerUrl.trim() || !ssoClientId.trim())) ||
              (ssoProvider === "saml" && !samlMetadataUrl.trim() && !samlMetadataXml.trim())
            }
          >
            {savingSso ? "Saving..." : "Save SSO settings"}
          </button>
          {ssoConfigured && (
            <button
              type="button"
              className="btn btn--danger"
              onClick={handleRemoveSso}
              disabled={removingSso}
            >
              {removingSso ? "Removing..." : "Remove SSO"}
            </button>
          )}
        </div>
      </form>

      <div className="card settings-section">
        <h2>SCIM Provisioning</h2>
        <p className="card-description">
          Automatically provision and deprovision workspace members from your identity provider.
        </p>

        {scimError && <p className="form-error">{scimError}</p>}
        {scimMessage && <p className="form-success">{scimMessage}</p>}

        {scimConfigured ? (
          <>
            <div className="form-field">
              <label className="form-label">Status</label>
              <p>
                {scimCreatedAt
                  ? `Active (created ${new Date(scimCreatedAt).toLocaleDateString()})`
                  : "Active"}
              </p>
            </div>

            <div className="form-field">
              <label className="form-label">SCIM Base URL</label>
              <div style={{ display: "flex", gap: "0.5rem" }}>
                <input
                  type="text"
                  className="form-input"
                  readOnly
                  value={`${window.location.origin}/api/organizations/${orgId}/scim/v2`}
                />
                <button
                  type="button"
                  className="btn"
                  onClick={() => {
                    navigator.clipboard.writeText(
                      `${window.location.origin}/api/organizations/${orgId}/scim/v2`
                    );
                  }}
                >
                  Copy
                </button>
              </div>
            </div>

            {scimToken && (
              <div className="form-field">
                <label className="form-label">Bearer Token</label>
                <div style={{ display: "flex", gap: "0.5rem" }}>
                  <input type="text" className="form-input" readOnly value={scimToken} />
                  <button
                    type="button"
                    className="btn"
                    onClick={() => navigator.clipboard.writeText(scimToken)}
                  >
                    Copy
                  </button>
                </div>
                <p className="form-hint">Copy this token now. It won't be shown again.</p>
              </div>
            )}

            <details className="settings-details">
              <summary>Setup Guide</summary>
              <pre>{`SCIM Base URL:
  ${window.location.origin}/api/organizations/${orgId}/scim/v2

Authentication:
  Authorization: Bearer <token>

Okta:
  In your Okta app -> Provisioning -> SCIM connector,
  paste the Base URL and Bearer Token.

Azure AD:
  In Enterprise Applications -> your app -> Provisioning,
  set Tenant URL to the Base URL and Secret Token to the Bearer Token.`}</pre>
            </details>

            <div className="btn-row" style={{ marginTop: "1rem" }}>
              <button
                type="button"
                className="btn"
                onClick={handleRegenerateScimToken}
                disabled={scimGenerating}
              >
                {scimGenerating ? "Generating..." : "Regenerate Token"}
              </button>
              <button
                type="button"
                className="btn btn--danger"
                onClick={handleRevokeScimToken}
              >
                Revoke Token
              </button>
            </div>
          </>
        ) : (
          <button
            type="button"
            className="btn btn--primary"
            onClick={handleGenerateScimToken}
            disabled={scimGenerating}
          >
            {scimGenerating ? "Generating..." : "Generate SCIM Token"}
          </button>
        )}
      </div>
    </>
  );
}
