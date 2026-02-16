import { type FormEvent, useEffect, useState } from "react";
import { apiFetch } from "../api/client";

interface UserProfile {
  name: string;
  email: string;
}

interface BrandingSettings {
  companyName: string | null;
  logoKey: string | null;
  colorBackground: string | null;
  colorSurface: string | null;
  colorText: string | null;
  colorAccent: string | null;
  footerText: string | null;
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
  const [viewNotification, setViewNotification] = useState("off");
  const [notificationMessage, setNotificationMessage] = useState("");
  const [brandingEnabled, setBrandingEnabled] = useState(false);
  const [branding, setBranding] = useState<BrandingSettings>({
    companyName: null, logoKey: null,
    colorBackground: null, colorSurface: null, colorText: null, colorAccent: null,
    footerText: null,
  });
  const [brandingMessage, setBrandingMessage] = useState("");
  const [brandingError, setBrandingError] = useState("");
  const [savingBranding, setSavingBranding] = useState(false);

  useEffect(() => {
    async function fetchProfile() {
      try {
        const [result, notifPrefs, limits] = await Promise.all([
          apiFetch<UserProfile>("/api/user"),
          apiFetch<{ viewNotification: string }>("/api/settings/notifications"),
          apiFetch<{ brandingEnabled: boolean }>("/api/videos/limits"),
        ]);
        if (result) {
          setProfile(result);
          setName(result.name);
        }
        if (notifPrefs) {
          setViewNotification(notifPrefs.viewNotification);
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
    const previous = viewNotification;
    setViewNotification(value);
    try {
      await apiFetch("/api/settings/notifications", {
        method: "PUT",
        body: JSON.stringify({ viewNotification: value }),
      });
      setNotificationMessage("Preference saved");
    } catch {
      setViewNotification(previous);
      setNotificationMessage("Failed to save");
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
          colorBackground: branding.colorBackground || null,
          colorSurface: branding.colorSurface || null,
          colorText: branding.colorText || null,
          colorAccent: branding.colorAccent || null,
          footerText: branding.footerText || null,
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
      footerText: null,
    });
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
          When someone watches one of your videos, get notified by email.
        </p>

        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>View notifications</span>
          <select
            value={viewNotification}
            onChange={(e) => handleNotificationChange(e.target.value)}
            style={inputStyle}
          >
            <option value="off">Off</option>
            <option value="every">Every view</option>
            <option value="first">First view only</option>
            <option value="digest">Daily digest</option>
          </select>
        </label>

        {notificationMessage && (
          <p style={{ color: notificationMessage === "Failed to save" ? "var(--color-error, #e74c3c)" : "var(--color-accent)", fontSize: 14, margin: 0 }}>{notificationMessage}</p>
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

          {brandingError && (
            <p style={{ color: "var(--color-error)", fontSize: 14, margin: 0 }}>{brandingError}</p>
          )}
          {brandingMessage && (
            <p style={{ color: "var(--color-accent)", fontSize: 14, margin: 0 }}>{brandingMessage}</p>
          )}

          <div style={{ display: "flex", gap: 12 }}>
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
