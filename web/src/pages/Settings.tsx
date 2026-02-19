import { type FormEvent, useEffect, useState } from "react";
import { apiFetch } from "../api/client";

interface UserProfile {
  name: string;
  email: string;
}

interface APIKeyItem {
  id: string;
  name: string;
  createdAt: string;
  lastUsedAt: string | null;
}

interface BrandingSettings {
  companyName: string | null;
  logoKey: string | null;
  colorBackground: string | null;
  colorSurface: string | null;
  colorText: string | null;
  colorAccent: string | null;
  footerText: string | null;
  customCss: string | null;
}

const hexColorPattern = /^#[0-9a-fA-F]{6}$/;

export function Settings() {
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [name, setName] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [nameMessage, setNameMessage] = useState("");
  const [nameError, setNameError] = useState("");
  const [passwordMessage, setPasswordMessage] = useState("");
  const [passwordError, setPasswordError] = useState("");
  const [savingName, setSavingName] = useState(false);
  const [savingPassword, setSavingPassword] = useState(false);
  const [notificationMode, setNotificationMode] = useState("off");
  const [notificationMessage, setNotificationMessage] = useState("");
  const [apiKeys, setApiKeys] = useState<APIKeyItem[]>([]);
  const [newKeyName, setNewKeyName] = useState("");
  const [generatedKey, setGeneratedKey] = useState("");
  const [apiKeyError, setApiKeyError] = useState("");
  const [creatingKey, setCreatingKey] = useState(false);
  const [copiedKey, setCopiedKey] = useState(false);
  const [brandingEnabled, setBrandingEnabled] = useState(false);
  const [branding, setBranding] = useState<BrandingSettings>({
    companyName: null, logoKey: null,
    colorBackground: null, colorSurface: null, colorText: null, colorAccent: null,
    footerText: null, customCss: null,
  });
  const [brandingMessage, setBrandingMessage] = useState("");
  const [brandingError, setBrandingError] = useState("");
  const [savingBranding, setSavingBranding] = useState(false);
  const [uploadingLogo, setUploadingLogo] = useState(false);

  useEffect(() => {
    async function fetchProfile() {
      try {
        const [result, notifPrefs, limits, keys] = await Promise.all([
          apiFetch<UserProfile>("/api/user"),
          apiFetch<{ notificationMode: string }>("/api/settings/notifications"),
          apiFetch<{ brandingEnabled: boolean }>("/api/videos/limits"),
          apiFetch<APIKeyItem[]>("/api/settings/api-keys"),
        ]);
        if (result) {
          setProfile(result);
          setName(result.name);
        }
        if (notifPrefs) {
          setNotificationMode(notifPrefs.notificationMode);
        }
        if (keys) {
          setApiKeys(keys);
        }
        if (limits?.brandingEnabled) {
          setBrandingEnabled(true);
          const brandingData = await apiFetch<BrandingSettings>("/api/settings/branding");
          if (brandingData) {
            setBranding(brandingData);
          }
        }
      } catch {
        // stay on page, fields will be empty
      }
    }
    fetchProfile();
  }, []);

  async function handleNameSubmit(event: FormEvent) {
    event.preventDefault();
    setNameError("");
    setNameMessage("");

    if (!name.trim()) {
      setNameError("Name is required");
      return;
    }

    setSavingName(true);
    try {
      await apiFetch("/api/user", {
        method: "PATCH",
        body: JSON.stringify({ name: name.trim() }),
      });
      setNameMessage("Name updated");
      setProfile((prev) => prev ? { ...prev, name: name.trim() } : prev);
    } catch (err) {
      setNameError(err instanceof Error ? err.message : "Failed to update name");
    } finally {
      setSavingName(false);
    }
  }

  async function handlePasswordSubmit(event: FormEvent) {
    event.preventDefault();
    setPasswordError("");
    setPasswordMessage("");

    if (newPassword !== confirmPassword) {
      setPasswordError("Passwords do not match");
      return;
    }

    setSavingPassword(true);
    try {
      await apiFetch("/api/user", {
        method: "PATCH",
        body: JSON.stringify({ currentPassword, newPassword }),
      });
      setPasswordMessage("Password updated");
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
    } catch (err) {
      setPasswordError(err instanceof Error ? err.message : "Failed to update password");
    } finally {
      setSavingPassword(false);
    }
  }

  async function handleNotificationChange(value: string) {
    setNotificationMessage("");
    const previous = notificationMode;
    setNotificationMode(value);
    try {
      await apiFetch("/api/settings/notifications", {
        method: "PUT",
        body: JSON.stringify({ notificationMode: value }),
      });
      setNotificationMessage("Preference saved");
    } catch {
      setNotificationMode(previous);
      setNotificationMessage("Failed to save");
    }
  }

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
      if (!result) throw new Error("Failed to create API key");
      setGeneratedKey(result.key);
      setApiKeys((prev) => [{ id: result.id, name: result.name, createdAt: result.createdAt, lastUsedAt: null }, ...prev]);
      setNewKeyName("");
    } catch (err) {
      setApiKeyError(err instanceof Error ? err.message : "Failed to create API key");
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
      setApiKeyError(err instanceof Error ? err.message : "Failed to delete API key");
    }
  }

  async function handleBrandingSave(event: FormEvent) {
    event.preventDefault();
    setBrandingError("");
    setBrandingMessage("");

    for (const [key, value] of Object.entries(branding)) {
      if (key.startsWith("color") && value && !hexColorPattern.test(value)) {
        setBrandingError(`Invalid color for ${key}`);
        return;
      }
    }

    setSavingBranding(true);
    try {
      await apiFetch("/api/settings/branding", {
        method: "PUT",
        body: JSON.stringify({
          companyName: branding.companyName || null,
          logoKey: branding.logoKey === "none" ? "none" : branding.logoKey || null,
          colorBackground: branding.colorBackground || null,
          colorSurface: branding.colorSurface || null,
          colorText: branding.colorText || null,
          colorAccent: branding.colorAccent || null,
          footerText: branding.footerText || null,
          customCss: branding.customCss || null,
        }),
      });
      setBrandingMessage("Branding saved");
    } catch (err) {
      setBrandingError(err instanceof Error ? err.message : "Failed to save branding");
    } finally {
      setSavingBranding(false);
    }
  }

  function handleBrandingReset() {
    setBranding({
      companyName: null, logoKey: null,
      colorBackground: null, colorSurface: null, colorText: null, colorAccent: null,
      footerText: null, customCss: null,
    });
  }

  async function handleLogoUpload(file: File) {
    if (file.type !== "image/png" && file.type !== "image/svg+xml") {
      setBrandingError("Logo must be PNG or SVG");
      return;
    }
    if (file.size > 512 * 1024) {
      setBrandingError("Logo must be 512KB or smaller");
      return;
    }

    setUploadingLogo(true);
    setBrandingError("");
    try {
      const result = await apiFetch<{ uploadUrl: string; logoKey: string }>("/api/settings/branding/logo", {
        method: "POST",
        body: JSON.stringify({ contentType: file.type, contentLength: file.size }),
      });
      if (!result) throw new Error("Failed to get upload URL");

      const uploadResp = await fetch(result.uploadUrl, {
        method: "PUT",
        headers: { "Content-Type": file.type },
        body: file,
      });
      if (!uploadResp.ok) throw new Error("Failed to upload logo");

      setBranding((prev) => ({ ...prev, logoKey: result.logoKey }));
      setBrandingMessage("Logo uploaded");
    } catch (err) {
      setBrandingError(err instanceof Error ? err.message : "Failed to upload logo");
    } finally {
      setUploadingLogo(false);
    }
  }

  async function handleLogoRemove() {
    setBrandingError("");
    try {
      await apiFetch("/api/settings/branding/logo", { method: "DELETE" });
      setBranding((prev) => ({ ...prev, logoKey: null }));
      setBrandingMessage("Logo removed");
    } catch (err) {
      setBrandingError(err instanceof Error ? err.message : "Failed to remove logo");
    }
  }

  if (!profile) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16 }}>Loading...</p>
      </div>
    );
  }

  const inputStyle = {
    background: "var(--color-bg)",
    border: "1px solid var(--color-border)",
    borderRadius: 4,
    color: "var(--color-text)",
    padding: "8px 12px",
    fontSize: 14,
    width: "100%",
  };

  return (
    <div className="page-container">
      <h1 style={{ color: "var(--color-text)", fontSize: 24, marginBottom: 24 }}>
        Settings
      </h1>

      <form
        onSubmit={handleNameSubmit}
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: 8,
          padding: 24,
          marginBottom: 24,
          display: "flex",
          flexDirection: "column",
          gap: 16,
        }}
      >
        <h2 style={{ color: "var(--color-text)", fontSize: 18, margin: 0 }}>Profile</h2>

        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Email</span>
          <input
            type="email"
            value={profile.email}
            disabled
            style={{ ...inputStyle, opacity: 0.6 }}
          />
        </label>

        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Name</span>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            style={inputStyle}
          />
        </label>

        {nameError && (
          <p style={{ color: "var(--color-error)", fontSize: 14, margin: 0 }}>{nameError}</p>
        )}
        {nameMessage && (
          <p style={{ color: "var(--color-accent)", fontSize: 14, margin: 0 }}>{nameMessage}</p>
        )}

        <button
          type="submit"
          disabled={savingName || name.trim() === profile.name}
          style={{
            background: "var(--color-accent)",
            color: "var(--color-text)",
            borderRadius: 4,
            padding: "10px 16px",
            fontSize: 14,
            fontWeight: 600,
            opacity: savingName || name.trim() === profile.name ? 0.7 : 1,
            alignSelf: "flex-start",
          }}
        >
          {savingName ? "Saving..." : "Save name"}
        </button>
      </form>

      <div
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: 8,
          padding: 24,
          marginBottom: 24,
          display: "flex",
          flexDirection: "column",
          gap: 16,
        }}
      >
        <h2 style={{ color: "var(--color-text)", fontSize: 18, margin: 0 }}>Notifications</h2>
        <p style={{ color: "var(--color-text-secondary)", fontSize: 14, margin: 0 }}>
          Choose when to get email notifications for views and comments.
        </p>

        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Notifications</span>
          <select
            value={notificationMode}
            onChange={(e) => handleNotificationChange(e.target.value)}
            style={inputStyle}
          >
            <option value="off">Off</option>
            <option value="views_only">Views only</option>
            <option value="comments_only">Comments only</option>
            <option value="views_and_comments">Views + comments</option>
            <option value="digest">Daily digest (views + comments)</option>
          </select>
        </label>

        {notificationMessage && (
          <p style={{ color: notificationMessage === "Failed to save" ? "var(--color-error, #e74c3c)" : "var(--color-accent)", fontSize: 14, margin: 0 }}>{notificationMessage}</p>
        )}
      </div>

      <div
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: 8,
          padding: 24,
          marginBottom: 24,
          display: "flex",
          flexDirection: "column",
          gap: 16,
        }}
      >
        <h2 style={{ color: "var(--color-text)", fontSize: 18, margin: 0 }}>API Keys</h2>
        <p style={{ color: "var(--color-text-secondary)", fontSize: 14, margin: 0 }}>
          Generate API keys for integrations like Nextcloud. Keys are shown only once when created.
        </p>

        <form onSubmit={handleCreateAPIKey} className="api-key-form">
          <label style={{ display: "flex", flexDirection: "column", gap: 4, flex: 1 }}>
            <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Label</span>
            <input
              type="text"
              value={newKeyName}
              onChange={(e) => setNewKeyName(e.target.value)}
              placeholder="e.g. My Nextcloud"
              maxLength={100}
              style={inputStyle}
            />
          </label>
          <button
            type="submit"
            disabled={creatingKey}
            style={{
              background: "var(--color-accent)",
              color: "var(--color-text)",
              borderRadius: 4,
              padding: "8px 16px",
              fontSize: 14,
              fontWeight: 600,
              whiteSpace: "nowrap",
              opacity: creatingKey ? 0.7 : 1,
            }}
          >
            {creatingKey ? "Creating..." : "Create key"}
          </button>
        </form>

        {generatedKey && (
          <div
            style={{
              background: "var(--color-bg)",
              border: "1px solid var(--color-accent)",
              borderRadius: 4,
              padding: 12,
              display: "flex",
              flexDirection: "column",
              gap: 8,
            }}
          >
            <p style={{ color: "var(--color-accent)", fontSize: 14, margin: 0, fontWeight: 600 }}>
              Copy this key now — it won't be shown again
            </p>
            <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
              <code
                style={{
                  color: "var(--color-text)",
                  fontSize: 13,
                  background: "var(--color-surface)",
                  padding: "6px 10px",
                  borderRadius: 4,
                  flex: 1,
                  wordBreak: "break-all",
                }}
              >
                {generatedKey}
              </code>
              <button
                type="button"
                onClick={() => {
                  navigator.clipboard.writeText(generatedKey);
                  setCopiedKey(true);
                  setTimeout(() => setCopiedKey(false), 2000);
                }}
                style={{
                  background: "transparent",
                  color: "var(--color-text-secondary)",
                  border: "1px solid var(--color-border)",
                  borderRadius: 4,
                  padding: "6px 12px",
                  fontSize: 13,
                  cursor: "pointer",
                  whiteSpace: "nowrap",
                }}
              >
                {copiedKey ? "Copied" : "Copy"}
              </button>
            </div>
          </div>
        )}

        {apiKeyError && (
          <p style={{ color: "var(--color-error)", fontSize: 14, margin: 0 }}>{apiKeyError}</p>
        )}

        {apiKeys.length > 0 && (
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {apiKeys.map((key) => (
              <div
                key={key.id}
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  background: "var(--color-bg)",
                  borderRadius: 4,
                  padding: "10px 12px",
                }}
              >
                <div>
                  <span style={{ color: "var(--color-text)", fontSize: 14 }}>
                    {key.name || "Unnamed key"}
                  </span>
                  <div style={{ color: "var(--color-text-secondary)", fontSize: 12, marginTop: 2 }}>
                    Created {new Date(key.createdAt).toLocaleDateString()}
                    {key.lastUsedAt && ` · Last used ${new Date(key.lastUsedAt).toLocaleDateString()}`}
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => handleDeleteAPIKey(key.id)}
                  style={{
                    background: "transparent",
                    color: "var(--color-error, #e74c3c)",
                    border: "1px solid var(--color-error, #e74c3c)",
                    borderRadius: 4,
                    padding: "4px 10px",
                    fontSize: 13,
                    cursor: "pointer",
                  }}
                >
                  Delete
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      {brandingEnabled && (
        <form
          onSubmit={handleBrandingSave}
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 24,
            marginBottom: 24,
            display: "flex",
            flexDirection: "column",
            gap: 16,
          }}
        >
          <h2 style={{ color: "var(--color-text)", fontSize: 18, margin: 0 }}>Branding</h2>
          <p style={{ color: "var(--color-text-secondary)", fontSize: 14, margin: 0 }}>
            Customize how your shared video pages look to viewers.
          </p>

          <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Company name</span>
            <input
              type="text"
              value={branding.companyName ?? ""}
              onChange={(e) => setBranding({ ...branding, companyName: e.target.value || null })}
              placeholder="SendRec"
              maxLength={200}
              style={inputStyle}
            />
          </label>

          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Logo</span>
            <div style={{ display: "flex", alignItems: "center", gap: 12, flexWrap: "wrap" }}>
              {branding.logoKey && branding.logoKey !== "none" ? (
                <>
                  <span style={{ color: "var(--color-text)", fontSize: 14 }}>
                    {branding.logoKey.split("/").pop()}
                  </span>
                  <button
                    type="button"
                    onClick={handleLogoRemove}
                    style={{
                      background: "transparent",
                      color: "var(--color-error, #e74c3c)",
                      border: "1px solid var(--color-error, #e74c3c)",
                      borderRadius: 4,
                      padding: "4px 10px",
                      fontSize: 13,
                      cursor: "pointer",
                    }}
                  >
                    Remove
                  </button>
                </>
              ) : branding.logoKey === "none" ? (
                <>
                  <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Logo hidden</span>
                  <button
                    type="button"
                    onClick={handleLogoRemove}
                    style={{
                      background: "transparent",
                      color: "var(--color-text-secondary)",
                      border: "1px solid var(--color-border)",
                      borderRadius: 4,
                      padding: "4px 10px",
                      fontSize: 13,
                      cursor: "pointer",
                    }}
                  >
                    Show default logo
                  </button>
                </>
              ) : (
                <>
                  <label
                    style={{
                      background: "var(--color-bg)",
                      border: "1px solid var(--color-border)",
                      borderRadius: 4,
                      padding: "6px 12px",
                      fontSize: 14,
                      color: "var(--color-text-secondary)",
                      cursor: uploadingLogo ? "default" : "pointer",
                      opacity: uploadingLogo ? 0.7 : 1,
                    }}
                  >
                    {uploadingLogo ? "Uploading..." : "Upload logo (PNG or SVG, max 512KB)"}
                    <input
                      type="file"
                      accept="image/png,image/svg+xml"
                      style={{ display: "none" }}
                      disabled={uploadingLogo}
                      onChange={(e) => {
                        const file = e.target.files?.[0];
                        if (file) handleLogoUpload(file);
                        e.target.value = "";
                      }}
                    />
                  </label>
                  <button
                    type="button"
                    onClick={() => setBranding((prev) => ({ ...prev, logoKey: "none" }))}
                    style={{
                      background: "transparent",
                      color: "var(--color-text-secondary)",
                      border: "1px solid var(--color-border)",
                      borderRadius: 4,
                      padding: "6px 12px",
                      fontSize: 14,
                      cursor: "pointer",
                    }}
                  >
                    Hide logo
                  </button>
                </>
              )}
            </div>
          </div>

          <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Footer text</span>
            <textarea
              value={branding.footerText ?? ""}
              onChange={(e) => setBranding({ ...branding, footerText: e.target.value || null })}
              placeholder="Custom footer message"
              maxLength={500}
              rows={2}
              style={{ ...inputStyle, resize: "vertical" as const }}
            />
          </label>

          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            {(["colorBackground", "colorSurface", "colorText", "colorAccent"] as const).map((key) => {
              const labels: Record<string, string> = {
                colorBackground: "Background",
                colorSurface: "Surface",
                colorText: "Text",
                colorAccent: "Accent",
              };
              const defaults: Record<string, string> = {
                colorBackground: "#0a1628",
                colorSurface: "#1e293b",
                colorText: "#ffffff",
                colorAccent: "#00b67a",
              };
              return (
                <label key={key} style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                  <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>{labels[key]}</span>
                  <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                    <input
                      type="color"
                      value={branding[key] ?? defaults[key]}
                      onChange={(e) => setBranding({ ...branding, [key]: e.target.value })}
                      style={{ width: 36, height: 36, border: "none", borderRadius: 4, cursor: "pointer", padding: 0, background: "transparent" }}
                    />
                    <input
                      type="text"
                      value={branding[key] ?? ""}
                      onChange={(e) => setBranding({ ...branding, [key]: e.target.value || null })}
                      placeholder={defaults[key]}
                      style={{ ...inputStyle, flex: 1 }}
                    />
                  </div>
                </label>
              );
            })}
          </div>

          <div
            style={{
              borderRadius: 8,
              padding: 16,
              background: branding.colorBackground ?? "#0a1628",
              border: "1px solid var(--color-border)",
            }}
          >
            <p style={{ fontSize: 12, color: "var(--color-text-secondary)", marginBottom: 8 }}>Preview</p>
            <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
              <span style={{ color: branding.colorAccent ?? "#00b67a", fontWeight: 600 }}>
                {branding.companyName || "SendRec"}
              </span>
            </div>
            <div style={{ background: branding.colorSurface ?? "#1e293b", borderRadius: 6, padding: 12 }}>
              <span style={{ color: branding.colorText ?? "#ffffff", fontSize: 14 }}>Sample video title</span>
            </div>
          </div>

          <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Custom CSS</span>
            <textarea
              value={branding.customCss ?? ""}
              onChange={(e) => setBranding({ ...branding, customCss: e.target.value || null })}
              placeholder={"/* Override watch page styles */\nbody { font-family: 'Inter', sans-serif; }\n.download-btn { border-radius: 20px; }\n.comment-submit { border-radius: 20px; }"}
              maxLength={10240}
              rows={6}
              style={{ ...inputStyle, resize: "vertical" as const, fontFamily: "monospace" }}
            />
            <span style={{ color: "var(--color-text-secondary)", fontSize: 12, marginTop: 2 }}>
              Injected into the watch page &lt;style&gt; tag. Max 10KB. No @import url() or closing style tags.
            </span>
            <details style={{ marginTop: 4, fontSize: 12, color: "var(--color-text-secondary)" }}>
              <summary style={{ cursor: "pointer" }}>Available CSS selectors</summary>
              <pre style={{
                marginTop: 6,
                padding: "8px 10px",
                background: "var(--color-bg-secondary)",
                borderRadius: 6,
                fontSize: 11,
                lineHeight: 1.6,
                overflowX: "auto",
                whiteSpace: "pre",
              }}>{`/* CSS Variables (override colors set in branding) */
:root { --brand-bg; --brand-surface; --brand-text; --brand-accent }

/* Layout */
body              /* Page background, font-family, text color */
.container        /* Max-width wrapper (960px) */
video             /* Video player element */
h1                /* Video title */
.meta             /* "Creator · Date" line below title */

/* Header & Footer */
.logo             /* Company logo + name link */
.logo img         /* Logo image (20x20) */
.branding         /* Footer: "Shared via SendRec" */
.branding a       /* Footer link */

/* Actions Bar */
.actions          /* Container for download + speed buttons */
.download-btn     /* Download button */
.speed-controls   /* Speed button group */
.speed-btn        /* Individual speed button (0.5x, 1x, ...) */
.speed-btn.active /* Currently selected speed */

/* Comments */
.comments-section    /* Full comments area */
.comments-header     /* "Comments" heading */
.comment             /* Single comment card */
.comment-meta        /* Author + badges row */
.comment-author      /* Commenter name */
.comment-body        /* Comment text */
.comment-owner-badge /* "Owner" badge */
.comment-timestamp   /* Timestamp badge on comment */
.comment-form        /* New comment form */
.comment-form input  /* Name + email fields */
.comment-form textarea /* Comment text area */
.comment-submit      /* "Post comment" button */

/* Comment Markers Bar */
.markers-bar      /* Timeline bar below video */
.marker-dot       /* Individual comment marker */

/* Emoji Picker */
.emoji-trigger    /* Emoji button */
.emoji-grid       /* Emoji dropdown panel */
.emoji-btn        /* Individual emoji button */

/* Transcript */
.transcript-section   /* Full transcript area */
.transcript-header    /* "Transcript" heading */
.transcript-segment   /* Single transcript line */
.transcript-segment.active /* Currently playing segment */
.transcript-timestamp /* Timestamp in transcript */
.transcript-text      /* Transcript text */

/* Utilities */
.hidden           /* display: none */

/* Mobile (max-width: 640px) */
@media (max-width: 640px) { ... }`}</pre>
            </details>
          </label>

          {brandingError && (
            <p style={{ color: "var(--color-error)", fontSize: 14, margin: 0 }}>{brandingError}</p>
          )}
          {brandingMessage && (
            <p style={{ color: "var(--color-accent)", fontSize: 14, margin: 0 }}>{brandingMessage}</p>
          )}

          <div className="settings-button-row">
            <button
              type="submit"
              disabled={savingBranding}
              style={{
                background: "var(--color-accent)",
                color: "var(--color-text)",
                borderRadius: 4,
                padding: "10px 16px",
                fontSize: 14,
                fontWeight: 600,
                opacity: savingBranding ? 0.7 : 1,
              }}
            >
              {savingBranding ? "Saving..." : "Save branding"}
            </button>
            <button
              type="button"
              onClick={handleBrandingReset}
              style={{
                background: "transparent",
                color: "var(--color-text-secondary)",
                border: "1px solid var(--color-border)",
                borderRadius: 4,
                padding: "10px 16px",
                fontSize: 14,
                cursor: "pointer",
              }}
            >
              Reset to defaults
            </button>
          </div>
        </form>
      )}

      <form
        onSubmit={handlePasswordSubmit}
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: 8,
          padding: 24,
          display: "flex",
          flexDirection: "column",
          gap: 16,
        }}
      >
        <h2 style={{ color: "var(--color-text)", fontSize: 18, margin: 0 }}>Change password</h2>

        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Current password</span>
          <input
            type="password"
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.target.value)}
            required
            style={inputStyle}
          />
        </label>

        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>New password</span>
          <input
            type="password"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            required
            minLength={8}
            style={inputStyle}
          />
          <span style={{ color: "var(--color-text-secondary)", fontSize: 12, marginTop: 2 }}>
            Must be at least 8 characters
          </span>
        </label>

        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Confirm new password</span>
          <input
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            required
            minLength={8}
            style={inputStyle}
          />
        </label>

        {passwordError && (
          <p style={{ color: "var(--color-error)", fontSize: 14, margin: 0 }}>{passwordError}</p>
        )}
        {passwordMessage && (
          <p style={{ color: "var(--color-accent)", fontSize: 14, margin: 0 }}>{passwordMessage}</p>
        )}

        <button
          type="submit"
          disabled={savingPassword}
          style={{
            background: "var(--color-accent)",
            color: "var(--color-text)",
            borderRadius: 4,
            padding: "10px 16px",
            fontSize: 14,
            fontWeight: 600,
            opacity: savingPassword ? 0.7 : 1,
            alignSelf: "flex-start",
          }}
        >
          {savingPassword ? "Updating..." : "Change password"}
        </button>
      </form>
    </div>
  );
}
