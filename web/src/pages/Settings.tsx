import { type FormEvent, useEffect, useState } from "react";
import { apiFetch } from "../api/client";

interface UserProfile {
  name: string;
  email: string;
}

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

  useEffect(() => {
    async function fetchProfile() {
      try {
        const [result, notifPrefs] = await Promise.all([
          apiFetch<UserProfile>("/api/user"),
          apiFetch<{ viewNotification: string }>("/api/settings/notifications"),
        ]);
        if (result) {
          setProfile(result);
          setName(result.name);
        }
        if (notifPrefs) {
          setViewNotification(notifPrefs.viewNotification);
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
    setViewNotification(value);
    try {
      await apiFetch("/api/settings/notifications", {
        method: "PUT",
        body: JSON.stringify({ viewNotification: value }),
      });
      setNotificationMessage("Preference saved");
    } catch {
      setNotificationMessage("Failed to save");
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
