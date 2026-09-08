import { type FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { apiFetch, setAccessToken } from "../../api/client";
import { ConfirmDialog, ConfirmDialogState } from "../../components/ConfirmDialog";
import { LimitsResponse } from "../../types/limits";
import { providerLabel } from "../../utils/sso";
import { APIKeyItem, LinkedIdentity } from "./types";

interface SecuritySectionProps {
  limits: LimitsResponse | null;
  initialApiKeys: APIKeyItem[];
  initialIdentities: LinkedIdentity[];
  initialIdentityHasPassword: boolean;
}

export function SecuritySection({
  limits,
  initialApiKeys,
  initialIdentities,
  initialIdentityHasPassword,
}: SecuritySectionProps) {
  return (
    <>
      <ChangePassword />
      <ConnectedAccounts
        initialIdentities={initialIdentities}
        initialIdentityHasPassword={initialIdentityHasPassword}
      />
      <APIKeys limits={limits} initialApiKeys={initialApiKeys} />
      <DangerZone />
    </>
  );
}

function ChangePassword() {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [passwordMessage, setPasswordMessage] = useState("");
  const [passwordError, setPasswordError] = useState("");
  const [savingPassword, setSavingPassword] = useState(false);

  async function handlePasswordSubmit(event: FormEvent) {
    event.preventDefault();
    setPasswordError("");
    setPasswordMessage("");

    if (newPassword !== confirmPassword) {
      setPasswordError("Die Passwörter stimmen nicht überein");
      return;
    }

    setSavingPassword(true);
    try {
      await apiFetch("/api/user", {
        method: "PATCH",
        body: JSON.stringify({ currentPassword, newPassword }),
      });
      setPasswordMessage("Passwort aktualisiert");
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
    } catch (err) {
      setPasswordError(err instanceof Error ? err.message : "Passwort konnte nicht aktualisiert werden");
    } finally {
      setSavingPassword(false);
    }
  }

  return (
    <form
      onSubmit={handlePasswordSubmit}
      className="card settings-section"
    >
      <h2>Passwort ändern</h2>

      <div className="form-field">
        <label className="form-label" htmlFor="current-password">Aktuelles Passwort</label>
        <input
          id="current-password"
          type="password"
          className="form-input"
          value={currentPassword}
          onChange={(e) => setCurrentPassword(e.target.value)}
          required
          autoComplete="current-password"
        />
      </div>

      <div className="form-field">
        <label className="form-label" htmlFor="new-password">Neues Passwort</label>
        <input
          id="new-password"
          type="password"
          className="form-input"
          value={newPassword}
          onChange={(e) => setNewPassword(e.target.value)}
          required
          minLength={8}
          autoComplete="new-password"
        />
        <span className="form-hint">Mindestens 8 Zeichen erforderlich</span>
      </div>

      <div className="form-field">
        <label className="form-label" htmlFor="confirm-password">Neues Passwort bestätigen</label>
        <input
          id="confirm-password"
          type="password"
          className="form-input"
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
          required
          minLength={8}
          autoComplete="new-password"
        />
      </div>

      {passwordError && (
        <p className="status-message status-message--error">{passwordError}</p>
      )}
      {passwordMessage && (
        <p className="status-message status-message--success">{passwordMessage}</p>
      )}

      <div className="btn-row">
        <button
          type="submit"
          className="btn btn--primary"
          disabled={savingPassword}
        >
          {savingPassword ? "Wird aktualisiert..." : "Passwort ändern"}
        </button>
      </div>
    </form>
  );
}

interface ConnectedAccountsProps {
  initialIdentities: LinkedIdentity[];
  initialIdentityHasPassword: boolean;
}

function ConnectedAccounts({ initialIdentities, initialIdentityHasPassword }: ConnectedAccountsProps) {
  const [identities, setIdentities] = useState(initialIdentities);
  const [identityError, setIdentityError] = useState("");

  async function handleDisconnectIdentity(provider: string) {
    setIdentityError("");
    try {
      await apiFetch(`/api/user/identities/${provider}`, { method: "DELETE" });
      setIdentities((prev) => prev.filter((i) => i.provider !== provider));
    } catch (err) {
      setIdentityError(err instanceof Error ? err.message : "Trennen fehlgeschlagen");
    }
  }

  if (identities.length === 0) return null;

  return (
    <div className="card settings-section">
      <h2>Verknüpfte Konten</h2>
      <p className="card-description">
        Externe Konten, die mit deinem 99tools-Record-Konto verknüpft sind.
      </p>

      {identityError && (
        <p className="status-message status-message--error">{identityError}</p>
      )}

      <div className="key-list">
        {identities.map((identity) => (
          <div key={identity.provider} className="api-key-row">
            <div className="api-key-info">
              <span className="api-key-name">{providerLabel(identity.provider)}</span>
              <span className="api-key-meta">{identity.email}</span>
            </div>
            <button
              type="button"
              className="btn btn--danger btn--danger-sm"
              onClick={() => handleDisconnectIdentity(identity.provider)}
              disabled={identities.length <= 1 && !initialIdentityHasPassword}
              title={identities.length <= 1 && !initialIdentityHasPassword ? "Die einzige Anmeldemethode kann nicht getrennt werden" : undefined}
            >
              Trennen
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}

interface APIKeysProps {
  limits: LimitsResponse | null;
  initialApiKeys: APIKeyItem[];
}

function APIKeys({ limits, initialApiKeys }: APIKeysProps) {
  const [apiKeys, setApiKeys] = useState(initialApiKeys);
  const [newKeyName, setNewKeyName] = useState("");
  const [generatedKey, setGeneratedKey] = useState("");
  const [apiKeyError, setApiKeyError] = useState("");
  const [creatingKey, setCreatingKey] = useState(false);
  const [copiedKey, setCopiedKey] = useState(false);

  async function handleCreateAPIKey(event: FormEvent) {
    event.preventDefault();
    setApiKeyError("");
    setGeneratedKey("");
    setCreatingKey(true);
    try {
      const result = await apiFetch<{ id: string; key: string; name: string; createdAt: string }>("/api/settings/api-keys", {
        method: "POST",
        body: JSON.stringify({ name: newKeyName.trim() }),
      });
      if (!result) throw new Error("API-Schlüssel konnte nicht erstellt werden");
      setGeneratedKey(result.key);
      setApiKeys((prev) => [{ id: result.id, name: result.name, createdAt: result.createdAt, lastUsedAt: null }, ...prev]);
      setNewKeyName("");
    } catch (err) {
      setApiKeyError(err instanceof Error ? err.message : "API-Schlüssel konnte nicht erstellt werden");
    } finally {
      setCreatingKey(false);
    }
  }

  async function handleDeleteAPIKey(id: string) {
    setApiKeyError("");
    try {
      await apiFetch(`/api/settings/api-keys/${id}`, { method: "DELETE" });
      setApiKeys((prev) => prev.filter((k) => k.id !== id));
    } catch (err) {
      setApiKeyError(err instanceof Error ? err.message : "API-Schlüssel konnte nicht gelöscht werden");
    }
  }

  return (
    <div className="card settings-section">
      <h2>API-Schlüssel</h2>
      <p className="card-description">
        Erstelle API-Schlüssel für Integrationen wie Nextcloud. Schlüssel werden nur einmal direkt nach der Erstellung angezeigt.
      </p>

      <form onSubmit={handleCreateAPIKey} className="api-key-form-row">
        <div className="form-field" style={{ flex: 1 }}>
          <label className="form-label">Bezeichnung</label>
          <input
            type="text"
            className="form-input"
            value={newKeyName}
            onChange={(e) => setNewKeyName(e.target.value)}
            placeholder="z. B. Mein Nextcloud"
            maxLength={limits?.fieldLimits?.apiKeyName ?? 100}
          />
        </div>
        <button
          type="submit"
          className="btn btn--primary"
          disabled={creatingKey}
        >
          {creatingKey ? "Wird erstellt..." : "Schlüssel erstellen"}
        </button>
      </form>

      {generatedKey && (
        <div className="api-key-display">
          <span className="api-key-display-notice">
            Kopiere diesen Schlüssel jetzt – er wird später nicht erneut angezeigt
          </span>
          <div className="api-key-display-row">
            <code className="api-key-display-code">
              {generatedKey}
            </code>
            <button
              type="button"
              className="btn btn--secondary"
              onClick={() => {
                navigator.clipboard.writeText(generatedKey);
                setCopiedKey(true);
                setTimeout(() => setCopiedKey(false), 2000);
              }}
            >
              {copiedKey ? "Copied" : "Kopieren"}
            </button>
          </div>
        </div>
      )}

      {apiKeyError && (
        <p className="status-message status-message--error">{apiKeyError}</p>
      )}

      {apiKeys.length > 0 && (
        <div className="key-list">
          {apiKeys.map((key) => (
            <div key={key.id} className="api-key-row">
              <div className="api-key-info">
                <span className="api-key-name">
                  {key.name || "Unbenannter Schlüssel"}
                </span>
                <span className="api-key-meta">
                  Erstellt am {new Date(key.createdAt).toLocaleDateString("de-DE")}
                  {key.lastUsedAt && ` · Zuletzt verwendet am ${new Date(key.lastUsedAt).toLocaleDateString("de-DE")}`}
                </span>
              </div>
              <button
                type="button"
                className="btn btn--danger btn--danger-sm"
                onClick={() => handleDeleteAPIKey(key.id)}
              >
                Löschen
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function DangerZone() {
  const navigate = useNavigate();
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const [confirmDialog, setConfirmDialog] = useState<ConfirmDialogState | null>(null);

  async function handleSignOut() {
    await fetch("/api/auth/logout", { method: "POST", credentials: "include" }).catch(() => {});
    setAccessToken(null);
    navigate("/login");
  }

  function handleDeleteAccount() {
    setConfirmDialog({
      message: "Konto wirklich löschen? Dies kann nicht rückgängig gemacht werden. Alle Videos und Daten werden dauerhaft gelöscht.",
      onConfirm: async () => {
        setConfirmDialog(null);
        setDeleting(true);
        setDeleteError("");
        try {
          await apiFetch("/api/user", { method: "DELETE" });
          setAccessToken(null);
          navigate("/login");
        } catch (err) {
          setDeleteError(err instanceof Error ? err.message : "Konto konnte nicht gelöscht werden");
        } finally {
          setDeleting(false);
        }
      },
    });
  }

  return (
    <div className="card settings-section card--danger">
      <h2 style={{ color: "var(--color-error)" }}>Gefahrenbereich</h2>

      <div className="form-field" style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}>
        <div>
          <p className="form-label" style={{ margin: 0 }}>Abmelden</p>
          <p className="form-hint">Melde dein Konto auf diesem Gerät ab.</p>
        </div>
        <button type="button" className="btn btn--secondary" onClick={handleSignOut}>
          Abmelden
        </button>
      </div>

      <div className="form-field" style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}>
        <div>
          <p className="form-label" style={{ margin: 0 }}>Konto löschen</p>
          <p className="form-hint">Lösche dein Konto und alle Daten dauerhaft.</p>
        </div>
        <button
          type="button"
          className="btn"
          style={{ background: "var(--color-error)", color: "#fff", borderColor: "var(--color-error)" }}
          onClick={handleDeleteAccount}
          disabled={deleting}
        >
          {deleting ? "Wird gelöscht..." : "Konto löschen"}
        </button>
      </div>
      {deleteError && (
        <p className="status-message status-message--error">{deleteError}</p>
      )}
      {confirmDialog && (
        <ConfirmDialog
          message={confirmDialog.message}
          confirmLabel="Konto löschen"
          danger
          onConfirm={confirmDialog.onConfirm}
          onCancel={() => setConfirmDialog(null)}
        />
      )}
    </div>
  );
}
