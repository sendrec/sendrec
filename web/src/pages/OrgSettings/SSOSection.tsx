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
      setSsoMessage("SSO-Einstellungen gespeichert");
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
      setSsoError(err instanceof Error ? err.message : "SSO-Einstellungen konnten nicht gespeichert werden");
    } finally {
      setSavingSso(false);
    }
  }

  function handleRemoveSso() {
    setConfirmDialog({
      message: "SSO-Konfiguration entfernen? Mitglieder müssen sich anschließend mit Passwort anmelden.",
      confirmLabel: "SSO entfernen",
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
          setSsoMessage("SSO-Konfiguration entfernt");
        } catch (err) {
          setSsoError(err instanceof Error ? err.message : "SSO konnte nicht entfernt werden");
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
        setScimMessage("Token erstellt. Kopiere ihn jetzt – er wird später nicht erneut angezeigt.");
      }
    } catch (err) {
      setScimError(err instanceof Error ? err.message : "Token konnte nicht erstellt werden");
    } finally {
      setScimGenerating(false);
    }
  }

  function handleRegenerateScimToken() {
    setConfirmDialog({
      message: "SCIM-Token neu erzeugen? Der aktuelle Token funktioniert danach sofort nicht mehr.",
      confirmLabel: "Neu erstellen",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        await handleGenerateScimToken();
      },
    });
  }

  function handleRevokeScimToken() {
    setConfirmDialog({
      message: "SCIM-Token widerrufen? Die automatische Bereitstellung wird dadurch beendet.",
      confirmLabel: "Widerrufen",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        setScimError("");
        try {
          await apiFetch(`/api/organizations/${orgId}/scim-token`, { method: "DELETE" });
          setScimConfigured(false);
          setScimToken("");
          setScimCreatedAt("");
          setScimMessage("SCIM-Token widerrufen");
        } catch (err) {
          setScimError(err instanceof Error ? err.message : "Token konnte nicht widerrufen werden");
        }
      },
    });
  }

  return (
    <>
      <form onSubmit={handleSsoSave} className="card settings-section">
        <h2>Single Sign-On</h2>
        <p className="card-description">
          Richte Single Sign-On für deinen Arbeitsbereich ein. Mitglieder können sich über deinen Identity Provider anmelden.
        </p>

        <div className="form-field">
          <label className="form-label">Protokoll</label>
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
              <label className="form-label" htmlFor="sso-issuer-url">Issuer-URL</label>
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
              <label className="form-label" htmlFor="sso-client-id">Client-ID</label>
              <input
                id="sso-client-id"
                type="text"
                className="form-input"
                value={ssoClientId}
                onChange={(e) => setSsoClientId(e.target.value)}
              />
            </div>

            <div className="form-field">
              <label className="form-label" htmlFor="sso-client-secret">Client-Secret</label>
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

4. Click "SSO-Einstellungen speichern" and test the login flow.

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
              <label className="form-label" htmlFor="saml-metadata-url">Metadaten-URL</label>
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
              <label className="form-label" htmlFor="saml-metadata-xml">Oder Metadaten-XML einfügen</label>
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
                  <label className="form-label">IdP-Entity-ID</label>
                  <input className="form-input" value={samlEntityId} readOnly />
                </div>
                <div className="form-field">
                  <label className="form-label">IdP-SSO-URL</label>
                  <input className="form-input" value={samlSsoUrl} readOnly />
                </div>
              </>
            )}
            {spMetadataUrl && (
              <div className="form-field">
                <label className="form-label">SP-Metadaten-URL</label>
                <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
                  <input className="form-input" value={spMetadataUrl} readOnly style={{ flex: 1 }} />
                  <button
                    type="button"
                    className="btn btn--secondary btn--sm"
                    onClick={() => navigator.clipboard.writeText(spMetadataUrl)}
                  >
                    Kopieren
                  </button>
                </div>
                <small className="form-hint">Diese URL deinem IdP-Administrator bereitstellen</small>
              </div>
            )}
            <details className="settings-details">
              <summary>SAML-Einrichtungsanleitung</summary>
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
   in "Metadaten-URL" above. Or download the metadata
   XML and paste it in the text area.

4. Click "SSO-Einstellungen speichern". The IdP Entity ID and
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
            SSO für alle Mitglieder erzwingen
          </label>
        </div>
        {ssoEnforce && (
          <p className="form-hint" style={{ color: "var(--color-warning)" }}>
            Wenn dies erzwungen wird, müssen sich Mitglieder über deinen Identity Provider anmelden. Die Passwort-Anmeldung ist für Mitglieder des Arbeitsbereichs dann deaktiviert.
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
            {savingSso ? "Wird gespeichert..." : "SSO-Einstellungen speichern"}
          </button>
          {ssoConfigured && (
            <button
              type="button"
              className="btn btn--danger"
              onClick={handleRemoveSso}
              disabled={removingSso}
            >
              {removingSso ? "Wird entfernt..." : "SSO entfernen"}
            </button>
          )}
        </div>
      </form>

      <div className="card settings-section">
        <h2>SCIM-Bereitstellung</h2>
        <p className="card-description">
          Mitglieder des Arbeitsbereichs automatisch über deinen Identity Provider bereitstellen und entfernen.
        </p>

        {scimError && <p className="form-error">{scimError}</p>}
        {scimMessage && <p className="form-success">{scimMessage}</p>}

        {scimConfigured ? (
          <>
            <div className="form-field">
              <label className="form-label">Status</label>
              <p>
                {scimCreatedAt
                  ? `Aktiv (erstellt am ${new Date(scimCreatedAt).toLocaleDateString("de-DE")})`
                  : "Active"}
              </p>
            </div>

            <div className="form-field">
              <label className="form-label">SCIM-Basis-URL</label>
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
                  Kopieren
                </button>
              </div>
            </div>

            {scimToken && (
              <div className="form-field">
                <label className="form-label">Bearer-Token</label>
                <div style={{ display: "flex", gap: "0.5rem" }}>
                  <input type="text" className="form-input" readOnly value={scimToken} />
                  <button
                    type="button"
                    className="btn"
                    onClick={() => navigator.clipboard.writeText(scimToken)}
                  >
                    Kopieren
                  </button>
                </div>
                <p className="form-hint">Kopiere diesen Token jetzt. Er wird später nicht erneut angezeigt.</p>
              </div>
            )}

            <details className="settings-details">
              <summary>Einrichtungsanleitung</summary>
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
                {scimGenerating ? "Wird erstellt..." : "Token neu erzeugen"}
              </button>
              <button
                type="button"
                className="btn btn--danger"
                onClick={handleRevokeScimToken}
              >
                Token widerrufen
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
            {scimGenerating ? "Wird erstellt..." : "SCIM-Token erzeugen"}
          </button>
        )}
      </div>
    </>
  );
}
