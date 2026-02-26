import { type FormEvent, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { apiFetch, setAccessToken } from "../api/client";
import { useTheme } from "../hooks/useTheme";
import { TRANSCRIPTION_LANGUAGES } from "../constants/languages";

interface UserProfile {
  name: string;
  email: string;
  transcriptionLanguage?: string;
}

interface APIKeyItem {
  id: string;
  name: string;
  createdAt: string;
  lastUsedAt: string | null;
}

interface WebhookDelivery {
  id: string;
  event: string;
  payload: string;
  statusCode: number;
  responseBody: string;
  attempt: number;
  createdAt: string;
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

function formatJson(value: string): string {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

export function Settings() {
  const { theme, setTheme } = useTheme();
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
  const [slackWebhookUrl, setSlackWebhookUrl] = useState("");
  const [savedSlackUrl, setSavedSlackUrl] = useState("");
  const [slackMessage, setSlackMessage] = useState("");
  const [slackError, setSlackError] = useState("");
  const [savingSlack, setSavingSlack] = useState(false);
  const [testingSlack, setTestingSlack] = useState(false);
  const [webhookUrl, setWebhookUrl] = useState("");
  const [savedWebhookUrl, setSavedWebhookUrl] = useState("");
  const [webhookSecret, setWebhookSecret] = useState("");
  const [savingWebhook, setSavingWebhook] = useState(false);
  const [testingWebhook, setTestingWebhook] = useState(false);
  const [webhookError, setWebhookError] = useState("");
  const [webhookMessage, setWebhookMessage] = useState("");
  const [webhookDeliveries, setWebhookDeliveries] = useState<WebhookDelivery[]>([]);
  const [expandedDelivery, setExpandedDelivery] = useState<string | null>(null);
  const [deliverySearch, setDeliverySearch] = useState("");
  const [deliveryFilter, setDeliveryFilter] = useState<"all" | "success" | "error">("all");
  const [regeneratingSecret, setRegeneratingSecret] = useState(false);
  const [copiedSecret, setCopiedSecret] = useState(false);
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
  const [billing, setBilling] = useState<{ plan: string; subscriptionId: string | null; subscriptionStatus: string | null; portalUrl: string | null } | null>(null);
  const [billingEnabled, setBillingEnabled] = useState(false);
  const [upgrading, setUpgrading] = useState(false);
  const [canceling, setCanceling] = useState(false);
  const [billingMessage, setBillingMessage] = useState("");
  const [transcriptionEnabled, setTranscriptionEnabled] = useState(false);
  const [transcriptionLanguage, setTranscriptionLanguage] = useState("auto");

  useEffect(() => {
    async function fetchProfile() {
      try {
        const [result, notifPrefs, limits, keys] = await Promise.all([
          apiFetch<UserProfile>("/api/user"),
          apiFetch<{ notificationMode: string; slackWebhookUrl: string | null; webhookUrl: string | null; webhookSecret: string | null }>("/api/settings/notifications"),
          apiFetch<{ brandingEnabled: boolean; transcriptionEnabled: boolean }>("/api/videos/limits"),
          apiFetch<APIKeyItem[]>("/api/settings/api-keys"),
        ]);
        if (result) {
          setProfile(result);
          setName(result.name);
          if (result.transcriptionLanguage) {
            setTranscriptionLanguage(result.transcriptionLanguage);
          }
        }
        if (notifPrefs) {
          setNotificationMode(notifPrefs.notificationMode);
          if (notifPrefs.slackWebhookUrl) {
            setSlackWebhookUrl(notifPrefs.slackWebhookUrl);
            setSavedSlackUrl(notifPrefs.slackWebhookUrl);
          }
          if (notifPrefs.webhookUrl) {
            setWebhookUrl(notifPrefs.webhookUrl);
            setSavedWebhookUrl(notifPrefs.webhookUrl);
          }
          if (notifPrefs.webhookSecret) {
            setWebhookSecret(notifPrefs.webhookSecret);
          }
        }
        if (keys) {
          setApiKeys(keys);
        }
        if (limits?.transcriptionEnabled) {
          setTranscriptionEnabled(true);
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

      try {
        const billingData = await apiFetch<{ plan: string; subscriptionId: string | null; subscriptionStatus: string | null; portalUrl: string | null }>("/api/settings/billing");
        if (billingData) {
          setBilling(billingData);
          setBillingEnabled(true);
        }
      } catch {
        // Billing not configured (self-hosted) — hide billing section
        setBillingEnabled(false);
      }

      const params = new URLSearchParams(window.location.search);
      if (params.get("billing") === "success") {
        setBillingMessage("Subscription activated successfully!");
        window.history.replaceState({}, "", window.location.pathname);
      }
    }
    fetchProfile();
  }, []);

  useEffect(() => {
    if (!savedWebhookUrl) return;
    apiFetch<WebhookDelivery[]>("/api/settings/notifications/webhook-deliveries")
      .then((data) => setWebhookDeliveries(data ?? []))
      .catch(() => {});
  }, [savedWebhookUrl]);

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

  async function handleSlackSave() {
    setSlackError("");
    setSlackMessage("");
    setSavingSlack(true);
    try {
      await apiFetch("/api/settings/notifications", {
        method: "PUT",
        body: JSON.stringify({ notificationMode, slackWebhookUrl }),
      });
      setSavedSlackUrl(slackWebhookUrl);
      setSlackMessage("Webhook URL saved");
    } catch (err) {
      setSlackError(err instanceof Error ? err.message : "Failed to save webhook URL");
    } finally {
      setSavingSlack(false);
    }
  }

  async function handleSlackTest() {
    setSlackError("");
    setSlackMessage("");
    setTestingSlack(true);
    try {
      await apiFetch("/api/settings/notifications/test-slack", {
        method: "POST",
      });
      setSlackMessage("Test message sent");
    } catch (err) {
      setSlackError(err instanceof Error ? err.message : "Failed to send test message");
    } finally {
      setTestingSlack(false);
    }
  }

  async function handleWebhookSave() {
    setSavingWebhook(true);
    setWebhookError("");
    setWebhookMessage("");
    try {
      await apiFetch("/api/settings/notifications", {
        method: "PUT",
        body: JSON.stringify({
          notificationMode,
          slackWebhookUrl: savedSlackUrl || undefined,
          webhookUrl: webhookUrl || undefined,
        }),
      });
      setSavedWebhookUrl(webhookUrl);
      setWebhookMessage("Saved");
      const prefs = await apiFetch<{ webhookSecret: string | null }>("/api/settings/notifications");
      if (prefs?.webhookSecret) setWebhookSecret(prefs.webhookSecret);
    } catch (err) {
      setWebhookError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSavingWebhook(false);
    }
  }

  async function handleWebhookTest() {
    setTestingWebhook(true);
    setWebhookError("");
    setWebhookMessage("");
    try {
      await apiFetch("/api/settings/notifications/test-webhook", { method: "POST" });
      setWebhookMessage("Test event sent");
      const data = await apiFetch<WebhookDelivery[]>("/api/settings/notifications/webhook-deliveries");
      setWebhookDeliveries(data ?? []);
    } catch (err) {
      setWebhookError(err instanceof Error ? err.message : "Failed to send test");
    } finally {
      setTestingWebhook(false);
    }
  }

  async function handleRegenerateSecret() {
    setRegeneratingSecret(true);
    setWebhookError("");
    try {
      const resp = await apiFetch<{ webhookSecret: string }>("/api/settings/notifications/regenerate-webhook-secret", { method: "POST" });
      if (resp?.webhookSecret) setWebhookSecret(resp.webhookSecret);
      setWebhookMessage("Secret regenerated");
    } catch (err) {
      setWebhookError(err instanceof Error ? err.message : "Failed to regenerate");
    } finally {
      setRegeneratingSecret(false);
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

  async function handleUpgrade() {
    setUpgrading(true);
    setBillingMessage("");
    try {
      const resp = await apiFetch<{ checkoutUrl: string }>("/api/settings/billing/checkout", {
        method: "POST",
        body: JSON.stringify({ plan: "pro" }),
      });
      if (resp?.checkoutUrl) {
        window.location.href = resp.checkoutUrl;
      }
    } catch (err: unknown) {
      setBillingMessage(err instanceof Error ? err.message : "Failed to start checkout");
    } finally {
      setUpgrading(false);
    }
  }

  async function handleCancelSubscription() {
    if (!confirm("Cancel your Pro subscription? You'll keep access until the end of your billing period.")) return;
    setCanceling(true);
    setBillingMessage("");
    try {
      await apiFetch("/api/settings/billing/cancel", { method: "POST" });
      setBillingMessage("Subscription canceled. Access continues until end of billing period.");
      setBilling((b) => b ? { ...b, subscriptionStatus: "canceled" } : b);
    } catch (err: unknown) {
      setBillingMessage(err instanceof Error ? err.message : "Failed to cancel");
    } finally {
      setCanceling(false);
    }
  }

  if (!profile) {
    return (
      <div className="page-container page-container--centered">
        <p className="status-message status-message--success">Loading...</p>
      </div>
    );
  }

  async function handleTranscriptionLanguageChange(value: string) {
    const previous = transcriptionLanguage;
    setTranscriptionLanguage(value);
    try {
      await apiFetch("/api/user", {
        method: "PATCH",
        body: JSON.stringify({ transcriptionLanguage: value }),
      });
    } catch {
      setTranscriptionLanguage(previous);
    }
  }

  return (
    <div className="page-container">
      <h1 className="page-title">Settings</h1>

      {billingEnabled && billing && (
        <div className="card settings-section">
          <div className="card-header">
            <h2>Subscription</h2>
            <span className={`plan-badge ${billing.plan === "pro" ? "plan-badge--pro" : ""}`}>
              {billing.plan === "pro" ? "Pro" : "Free"}
            </span>
          </div>

          {billing.plan === "free" && !billing.subscriptionStatus && (
            <>
              <p className="card-description">
                Upgrade to Pro for unlimited videos and recording duration.
              </p>
              <div className="upgrade-card">
                <div className="upgrade-card-info">
                  <span className="upgrade-card-plan">Pro</span>
                  <span className="upgrade-card-desc">Unlimited videos and duration</span>
                </div>
                <div className="upgrade-card-actions">
                  <span className="upgrade-card-price">&euro;8/mo</span>
                  <button
                    type="button"
                    className="btn btn--primary"
                    onClick={handleUpgrade}
                    disabled={upgrading}
                  >
                    {upgrading ? "Redirecting..." : "Upgrade to Pro"}
                  </button>
                </div>
              </div>
            </>
          )}

          {billing.subscriptionStatus === "canceled" && (
            <p className="card-description">
              Your subscription has been canceled. You have access to Pro features until the end of your billing period.
            </p>
          )}

          {billing.plan === "pro" && billing.subscriptionStatus !== "canceled" && (
            <div className="btn-row">
              {billing.portalUrl && (
                <a
                  href={billing.portalUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="billing-portal-link"
                >
                  Manage subscription
                </a>
              )}
              <button
                type="button"
                className="btn btn--danger"
                onClick={handleCancelSubscription}
                disabled={canceling}
              >
                {canceling ? "Canceling..." : "Cancel subscription"}
              </button>
            </div>
          )}

          {billingMessage && (
            <p className="status-message">{billingMessage}</p>
          )}
        </div>
      )}

      <form
        onSubmit={handleNameSubmit}
        className="card settings-section"
      >
        <h2>Profile</h2>

        <div className="form-field">
          <label className="form-label" htmlFor="profile-email">Email</label>
          <input
            id="profile-email"
            type="email"
            className="form-input"
            value={profile.email}
            disabled
          />
        </div>

        <div className="form-field">
          <label className="form-label" htmlFor="profile-name">Name</label>
          <input
            id="profile-name"
            type="text"
            className="form-input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        </div>

        {nameError && (
          <p className="status-message status-message--error">{nameError}</p>
        )}
        {nameMessage && (
          <p className="status-message status-message--success">{nameMessage}</p>
        )}

        <div className="btn-row">
          <button
            type="submit"
            className="btn btn--primary"
            disabled={savingName || name.trim() === profile.name}
          >
            {savingName ? "Saving..." : "Save name"}
          </button>
        </div>
      </form>

      <div className="card settings-section">
        <h2>Appearance</h2>
        <p className="card-description">
          Choose how SendRec looks to you.
        </p>

        <fieldset className="btn-row" style={{ border: "none", padding: 0, margin: 0 }}>
          <legend className="sr-only">Theme preference</legend>
          {(["dark", "light", "system"] as const).map((option) => {
            const labels: Record<string, string> = { dark: "Dark", light: "Light", system: "System" };
            const selected = theme === option;
            return (
              <label
                key={option}
                className={`theme-option${selected ? " theme-option--active" : ""}`}
              >
                <input
                  type="radio"
                  name="theme"
                  value={option}
                  checked={selected}
                  onChange={() => setTheme(option)}
                  className="sr-only"
                  aria-label={labels[option]}
                />
                {labels[option]}
              </label>
            );
          })}
        </fieldset>
      </div>

      <RecordingDefaults />

      {transcriptionEnabled && (
        <div className="card settings-section">
          <h2>Transcription</h2>
          <p className="card-description">
            Choose the default language for video transcription.
          </p>
          <div className="form-field">
            <label className="form-label" htmlFor="transcription-language">Default transcription language</label>
            <select
              id="transcription-language"
              className="form-input"
              value={transcriptionLanguage}
              onChange={(e) => handleTranscriptionLanguageChange(e.target.value)}
            >
              {TRANSCRIPTION_LANGUAGES.map((lang) => (
                <option key={lang.code} value={lang.code}>{lang.name}</option>
              ))}
            </select>
          </div>
        </div>
      )}

      <div className="card settings-section">
        <h2>Email Notifications</h2>
        <p className="card-description">
          Choose when to get email notifications for views and comments.
        </p>

        <div className="form-field">
          <label className="form-label" htmlFor="notification-mode">Notifications</label>
          <select
            id="notification-mode"
            className="form-input"
            value={notificationMode}
            onChange={(e) => handleNotificationChange(e.target.value)}
          >
            <option value="off">Off</option>
            <option value="views_only">Views only</option>
            <option value="comments_only">Comments only</option>
            <option value="views_and_comments">Views + comments</option>
            <option value="digest">Daily digest (views + comments)</option>
          </select>
        </div>

        {notificationMessage && (
          <p className={`status-message ${notificationMessage === "Failed to save" ? "status-message--error" : "status-message--success"}`}>{notificationMessage}</p>
        )}
      </div>

      <div className="card settings-section">
        <h2>Slack Notifications</h2>
        <p className="card-description">
          Send video view and comment notifications to a Slack channel.
        </p>

        <div className="form-field">
          <label className="form-label">Slack webhook URL</label>
          <input
            type="url"
            className="form-input"
            value={slackWebhookUrl}
            onChange={(e) => setSlackWebhookUrl(e.target.value)}
            placeholder="https://hooks.slack.com/services/..."
          />
        </div>

        <div className="btn-row">
          <button
            type="button"
            className="btn btn--primary"
            onClick={handleSlackSave}
            disabled={savingSlack}
          >
            {savingSlack ? "Saving..." : "Save"}
          </button>
          <button
            type="button"
            className="btn btn--secondary"
            onClick={handleSlackTest}
            disabled={testingSlack || !savedSlackUrl}
          >
            {testingSlack ? "Sending..." : "Send test message"}
          </button>
        </div>

        {slackError && (
          <p className="status-message status-message--error">{slackError}</p>
        )}
        {slackMessage && (
          <p className="status-message status-message--success">{slackMessage}</p>
        )}

        <details className="settings-details">
          <summary>How to get a webhook URL</summary>
          <ol>
            <li>Go to <a href="https://api.slack.com/apps" target="_blank" rel="noopener noreferrer">api.slack.com/apps</a></li>
            <li>Click <strong>Create New App</strong> and choose <strong>From scratch</strong></li>
            <li>Under <strong>Features</strong>, select <strong>Incoming Webhooks</strong></li>
            <li>Activate webhooks and click <strong>Add New Webhook to Workspace</strong></li>
            <li>Choose a channel and copy the webhook URL</li>
          </ol>
        </details>
      </div>

      <div className="card settings-section">
        <h2>Webhooks</h2>
        <p className="card-description">
          Receive HTTP POST notifications for video events. Use with n8n, Zapier, or custom integrations.
        </p>

        <div className="form-field">
          <label className="form-label">Webhook URL</label>
          <input
            type="url"
            className="form-input"
            value={webhookUrl}
            onChange={(e) => setWebhookUrl(e.target.value)}
            placeholder="https://example.com/webhook"
          />
          <span className="form-hint">Receive HTTP POST notifications for video events (n8n, Zapier, custom).</span>
        </div>

        {webhookSecret && (
          <div className="form-field">
            <span className="form-label">Signing secret</span>
            <div className="secret-row">
              <code className="secret-code">
                {webhookSecret}
              </code>
              <button
                type="button"
                className="btn btn--secondary"
                onClick={() => {
                  navigator.clipboard.writeText(webhookSecret);
                  setCopiedSecret(true);
                  setTimeout(() => setCopiedSecret(false), 2000);
                }}
              >
                {copiedSecret ? "Copied" : "Copy"}
              </button>
              <button
                type="button"
                className="btn btn--secondary"
                onClick={handleRegenerateSecret}
                disabled={regeneratingSecret}
              >
                {regeneratingSecret ? "Regenerating..." : "Regenerate"}
              </button>
            </div>
          </div>
        )}

        <div className="btn-row">
          <button
            type="button"
            className="btn btn--primary"
            onClick={handleWebhookSave}
            disabled={savingWebhook}
          >
            {savingWebhook ? "Saving..." : "Save webhook"}
          </button>
          <button
            type="button"
            className="btn btn--secondary"
            onClick={handleWebhookTest}
            disabled={testingWebhook || !savedWebhookUrl}
          >
            {testingWebhook ? "Sending..." : "Send test event"}
          </button>
        </div>

        {webhookError && (
          <p className="status-message status-message--error">{webhookError}</p>
        )}
        {webhookMessage && (
          <p className="status-message status-message--success">{webhookMessage}</p>
        )}

        {webhookDeliveries.length > 0 && (() => {
          const filtered = webhookDeliveries.filter((d) => {
            if (deliveryFilter === "success" && (d.statusCode < 200 || d.statusCode >= 300)) return false;
            if (deliveryFilter === "error" && d.statusCode >= 200 && d.statusCode < 300) return false;
            if (deliverySearch && !d.event.toLowerCase().includes(deliverySearch.toLowerCase())) return false;
            return true;
          });
          return (
            <div className="delivery-list">
              <h3 className="delivery-list-title">Recent deliveries</h3>
              <div className="delivery-toolbar">
                <input
                  type="text"
                  className="delivery-search"
                  placeholder="Filter by event..."
                  value={deliverySearch}
                  onChange={(e) => setDeliverySearch(e.target.value)}
                />
                <div className="delivery-filters">
                  {(["all", "success", "error"] as const).map((f) => (
                    <button
                      key={f}
                      type="button"
                      className={`delivery-filter-btn${deliveryFilter === f ? " delivery-filter-btn--active" : ""}`}
                      onClick={() => setDeliveryFilter(f)}
                    >
                      {f === "all" ? "All" : f === "success" ? "Success" : "Errors"}
                    </button>
                  ))}
                </div>
              </div>
              <div className="delivery-scroll">
                {filtered.length === 0 ? (
                  <p className="delivery-empty">No matching deliveries</p>
                ) : (
                  filtered.map((delivery) => {
                    const isSuccess = delivery.statusCode >= 200 && delivery.statusCode < 300;
                    const isExpanded = expandedDelivery === delivery.id;
                    return (
                      <div key={delivery.id}>
                        <button
                          type="button"
                          className="delivery-row"
                          onClick={() => setExpandedDelivery(isExpanded ? null : delivery.id)}
                        >
                          <span className={`delivery-dot ${isSuccess ? "delivery-dot--success" : "delivery-dot--error"}`} />
                          <code className="delivery-event">
                            {delivery.event}
                          </code>
                          <span className="delivery-status">
                            {delivery.statusCode}
                          </span>
                          <span className="delivery-time">
                            {new Date(delivery.createdAt).toLocaleString("en-GB")}
                          </span>
                        </button>
                        {isExpanded && (
                          <div className="delivery-detail">
                            <div>
                              <span className="delivery-detail-label">Payload</span>
                              <pre className="delivery-detail-pre">
                                {formatJson(delivery.payload)}
                              </pre>
                            </div>
                            {delivery.responseBody && (
                              <div>
                                <span className="delivery-detail-label">Response</span>
                                <pre className="delivery-detail-pre">
                                  {delivery.responseBody}
                                </pre>
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    );
                  })
                )}
              </div>
            </div>
          );
        })()}

        <details className="settings-details">
          <summary>Supported events</summary>
          <ul>
            <li><code>video.viewed</code> — A viewer watched a video</li>
            <li><code>video.comment.created</code> — A new comment was posted</li>
            <li><code>video.reaction.created</code> — An emoji reaction was added</li>
            <li><code>video.transcription.ready</code> — Transcription completed</li>
            <li><code>video.summary.ready</code> — AI summary completed</li>
            <li><code>video.cta.clicked</code> — A CTA button was clicked</li>
            <li><code>test</code> — Test event from Settings</li>
          </ul>
        </details>
      </div>

      <div className="card settings-section">
        <h2>API Keys</h2>
        <p className="card-description">
          Generate API keys for integrations like Nextcloud. Keys are shown only once when created.
        </p>

        <form onSubmit={handleCreateAPIKey} className="api-key-form-row">
          <div className="form-field" style={{ flex: 1 }}>
            <label className="form-label">Label</label>
            <input
              type="text"
              className="form-input"
              value={newKeyName}
              onChange={(e) => setNewKeyName(e.target.value)}
              placeholder="e.g. My Nextcloud"
              maxLength={100}
            />
          </div>
          <button
            type="submit"
            className="btn btn--primary"
            disabled={creatingKey}
          >
            {creatingKey ? "Creating..." : "Create key"}
          </button>
        </form>

        {generatedKey && (
          <div className="api-key-display">
            <span className="api-key-display-notice">
              Copy this key now — it won't be shown again
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
                {copiedKey ? "Copied" : "Copy"}
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
                    {key.name || "Unnamed key"}
                  </span>
                  <span className="api-key-meta">
                    Created {new Date(key.createdAt).toLocaleDateString("en-GB")}
                    {key.lastUsedAt && ` \u00B7 Last used ${new Date(key.lastUsedAt).toLocaleDateString("en-GB")}`}
                  </span>
                </div>
                <button
                  type="button"
                  className="btn btn--danger btn--danger-sm"
                  onClick={() => handleDeleteAPIKey(key.id)}
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
          className="card settings-section"
        >
          <h2>Branding</h2>
          <p className="card-description">
            Customize how your shared video pages look to viewers.
          </p>

          <div className="form-field">
            <label className="form-label">Company name</label>
            <input
              type="text"
              className="form-input"
              value={branding.companyName ?? ""}
              onChange={(e) => setBranding({ ...branding, companyName: e.target.value || null })}
              placeholder="SendRec"
              maxLength={200}
            />
          </div>

          <div className="logo-section">
            <span className="logo-section-label">Logo</span>
            <div className="logo-section-controls">
              {branding.logoKey && branding.logoKey !== "none" ? (
                <>
                  <span className="logo-section-name">
                    {branding.logoKey.split("/").pop()}
                  </span>
                  <button
                    type="button"
                    className="btn btn--danger btn--danger-sm"
                    onClick={handleLogoRemove}
                  >
                    Remove
                  </button>
                </>
              ) : branding.logoKey === "none" ? (
                <>
                  <span className="logo-section-status">Logo hidden</span>
                  <button
                    type="button"
                    className="btn btn--secondary"
                    onClick={handleLogoRemove}
                  >
                    Show default logo
                  </button>
                </>
              ) : (
                <>
                  <label className="btn btn--secondary" style={{ cursor: uploadingLogo ? "default" : "pointer" }}>
                    {uploadingLogo ? "Uploading..." : "Upload logo (PNG or SVG, max 512KB)"}
                    <input
                      type="file"
                      accept="image/png,image/svg+xml"
                      className="sr-only"
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
                    className="btn btn--secondary"
                    onClick={() => setBranding((prev) => ({ ...prev, logoKey: "none" }))}
                  >
                    Hide logo
                  </button>
                </>
              )}
            </div>
          </div>

          <div className="form-field">
            <label className="form-label">Footer text</label>
            <textarea
              className="form-input"
              value={branding.footerText ?? ""}
              onChange={(e) => setBranding({ ...branding, footerText: e.target.value || null })}
              placeholder="Custom footer message"
              maxLength={500}
              rows={2}
            />
          </div>

          <div className="color-grid">
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
                <div key={key} className="color-field">
                  <span className="form-label">{labels[key]}</span>
                  <div className="color-row">
                    <input
                      type="color"
                      className="color-swatch"
                      value={branding[key] ?? defaults[key]}
                      onChange={(e) => setBranding({ ...branding, [key]: e.target.value })}
                    />
                    <input
                      type="text"
                      className="form-input"
                      value={branding[key] ?? ""}
                      onChange={(e) => setBranding({ ...branding, [key]: e.target.value || null })}
                      placeholder={defaults[key]}
                      style={{ flex: 1 }}
                    />
                  </div>
                </div>
              );
            })}
          </div>

          <div
            className="branding-preview"
            style={{ background: branding.colorBackground ?? "#0a1628" }}
          >
            <p className="branding-preview-label">Preview</p>
            <div className="branding-preview-title" style={{ color: branding.colorAccent ?? "#00b67a" }}>
              {branding.companyName || "SendRec"}
            </div>
            <div className="branding-preview-card" style={{ background: branding.colorSurface ?? "#1e293b" }}>
              <span style={{ color: branding.colorText ?? "#ffffff", fontSize: 14 }}>Sample video title</span>
            </div>
          </div>

          <div className="form-field">
            <label className="form-label">Custom CSS</label>
            <textarea
              className="form-input form-input--mono"
              value={branding.customCss ?? ""}
              onChange={(e) => setBranding({ ...branding, customCss: e.target.value || null })}
              placeholder={"/* Override watch page styles */\nbody { font-family: 'Inter', sans-serif; }\n.download-btn { border-radius: 20px; }\n.comment-submit { border-radius: 20px; }"}
              maxLength={10240}
              rows={6}
            />
            <span className="form-hint">
              Injected into the watch page &lt;style&gt; tag. Max 10KB. No @import url() or closing style tags.
            </span>
            <details className="settings-details">
              <summary>Available CSS selectors</summary>
              <pre>{`/* CSS Variables */
:root {
  --brand-bg;       /* Page background */
  --brand-surface;  /* Cards, panels */
  --brand-text;     /* Primary text */
  --brand-accent;   /* Buttons, links */
  --player-accent;  /* Seek bar, progress */
}

/* Layout */
body                /* Background, font, text color */
.container          /* Max-width wrapper (960px) */
.video-title        /* Video heading */
.video-meta         /* Creator info row */
.video-meta-avatar  /* Creator avatar */
.video-meta-name    /* Creator name */

/* Header & Footer */
.logo               /* Logo + name link */
.logo img           /* Logo image */
.branding           /* "Shared via SendRec" footer */
.branding a         /* Footer link */

/* Video Player */
.player-container   /* Player wrapper */
.player-overlay     /* Play button overlay */
.play-overlay-btn   /* Large play button */
.player-controls    /* Control bar */
.ctrl-btn           /* Control buttons */
.time-display       /* Current / duration */
.seek-bar           /* Seek bar wrapper */
.seek-track         /* Track background */
.seek-progress      /* Play progress */
.seek-buffered      /* Buffered range */
.seek-thumb         /* Draggable handle */
.volume-group       /* Volume control */
.volume-slider      /* Volume slider */
.speed-dropdown     /* Speed selector */
.speed-menu         /* Speed options dropdown */
.speed-menu button.active /* Selected speed */

/* Seek Bar Overlays */
.seek-chapters      /* Chapter markers */
.seek-chapter       /* Single chapter */
.seek-markers       /* Comment markers */
.seek-marker        /* Single marker dot */

/* Actions */
.actions            /* Download + controls */
.download-btn       /* Download button */

/* Comments */
.comments-section   /* Full comments area */
.comments-header    /* Heading */
.comment            /* Single comment */
.comment-meta       /* Author + badges */
.comment-author     /* Commenter name */
.comment-body       /* Comment text */
.comment-owner-badge   /* "Owner" badge */
.comment-private-badge /* "Private" badge */
.comment-timestamp  /* Timestamp link */
.comment-form       /* New comment form */
.comment-form input /* Name + email fields */
.comment-form textarea /* Text area */
.comment-submit     /* "Post comment" button */
.no-comments        /* Empty state text */

/* Reactions */
.reaction-bar       /* Quick-reaction buttons */
.reaction-btn       /* Single reaction */

/* Emoji Picker */
.emoji-trigger      /* Emoji button */
.emoji-grid         /* Dropdown panel */
.emoji-btn          /* Single emoji */

/* Transcript & Summary */
.transcript-section /* Full panel area */
.panel-tabs         /* Summary/Transcript tabs */
.panel-tab          /* Tab button */
.panel-tab--active  /* Active tab */
.summary-text       /* Summary paragraph */
.chapter-list       /* Chapters container */
.chapter-item       /* Single chapter */
.chapter-item.active /* Playing chapter */
.chapter-timestamp  /* Chapter time */
.chapter-title      /* Chapter name */
.transcript-header  /* Heading */
.transcript-segment /* Single line */
.transcript-segment.active /* Playing line */
.transcript-timestamp /* Time link */
.transcript-text    /* Segment text */

/* Call to Action */
.cta-card           /* CTA container */
.cta-title          /* CTA heading */
.cta-desc           /* CTA description */
.cta-btn            /* CTA button */

/* Utilities */
.hidden             /* display: none */

/* Responsive & Accessibility */
@media (max-width: 640px) { ... }
@media (prefers-reduced-motion: reduce) { ... }`}</pre>
            </details>
          </div>

          {brandingError && (
            <p className="status-message status-message--error">{brandingError}</p>
          )}
          {brandingMessage && (
            <p className="status-message status-message--success">{brandingMessage}</p>
          )}

          <div className="btn-row">
            <button
              type="submit"
              className="btn btn--primary"
              disabled={savingBranding}
            >
              {savingBranding ? "Saving..." : "Save branding"}
            </button>
            <button
              type="button"
              className="btn btn--secondary"
              onClick={handleBrandingReset}
            >
              Reset to defaults
            </button>
          </div>
        </form>
      )}

      <form
        onSubmit={handlePasswordSubmit}
        className="card settings-section"
      >
        <h2>Change Password</h2>

        <div className="form-field">
          <label className="form-label" htmlFor="current-password">Current password</label>
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
          <label className="form-label" htmlFor="new-password">New password</label>
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
          <span className="form-hint">Must be at least 8 characters</span>
        </div>

        <div className="form-field">
          <label className="form-label" htmlFor="confirm-password">Confirm new password</label>
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
            {savingPassword ? "Updating..." : "Change password"}
          </button>
        </div>
      </form>

      <DangerZone />
    </div>
  );
}

type RecordingMode = "camera" | "screen" | "screen-camera";

function RecordingDefaults() {
  const [mode, setModeState] = useState<RecordingMode>(() => {
    const stored = localStorage.getItem("recording-mode");
    if (stored === "camera" || stored === "screen" || stored === "screen-camera") return stored;
    return "screen-camera";
  });
  const [countdown, setCountdownState] = useState(() => localStorage.getItem("recording-countdown") !== "false");
  const [systemAudio, setSystemAudioState] = useState(() => localStorage.getItem("recording-audio") !== "false");

  function setMode(m: RecordingMode) {
    setModeState(m);
    localStorage.setItem("recording-mode", m);
  }
  function setCountdown(v: boolean) {
    setCountdownState(v);
    localStorage.setItem("recording-countdown", String(v));
  }
  function setSystemAudio(v: boolean) {
    setSystemAudioState(v);
    localStorage.setItem("recording-audio", String(v));
  }

  const modes: { value: RecordingMode; label: string }[] = [
    { value: "camera", label: "Camera" },
    { value: "screen", label: "Screen" },
    { value: "screen-camera", label: "Screen + Camera" },
  ];

  return (
    <div className="card settings-section">
      <h2>Recording Defaults</h2>
      <p className="card-description">
        Set your preferred recording mode and options.
      </p>

      <div className="form-field">
        <label className="form-label">Default recording mode</label>
        <fieldset className="btn-row" style={{ border: "none", padding: 0, margin: 0 }}>
          <legend className="sr-only">Recording mode</legend>
          {modes.map((m) => (
            <label
              key={m.value}
              className={`theme-option${mode === m.value ? " theme-option--active" : ""}`}
            >
              <input
                type="radio"
                name="recording-mode"
                value={m.value}
                checked={mode === m.value}
                onChange={() => setMode(m.value)}
                className="sr-only"
              />
              {m.label}
            </label>
          ))}
        </fieldset>
      </div>

      <div className="form-field" style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}>
        <label className="form-label" style={{ margin: 0 }}>Countdown timer</label>
        <button
          type="button"
          className={`toggle-track${countdown ? " active" : ""}`}
          onClick={() => setCountdown(!countdown)}
          role="switch"
          aria-checked={countdown}
        >
          <span className="toggle-thumb" />
        </button>
      </div>

      <div className="form-field" style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}>
        <label className="form-label" style={{ margin: 0 }}>System audio capture</label>
        <button
          type="button"
          className={`toggle-track${systemAudio ? " active" : ""}`}
          onClick={() => setSystemAudio(!systemAudio)}
          role="switch"
          aria-checked={systemAudio}
        >
          <span className="toggle-thumb" />
        </button>
      </div>
    </div>
  );
}

function DangerZone() {
  const navigate = useNavigate();
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");

  async function handleSignOut() {
    await fetch("/api/auth/logout", { method: "POST", credentials: "include" }).catch(() => {});
    setAccessToken(null);
    navigate("/login");
  }

  async function handleDeleteAccount() {
    if (!confirm("Are you sure you want to delete your account? This action cannot be undone. All your videos and data will be permanently deleted.")) return;
    setDeleting(true);
    setDeleteError("");
    try {
      await apiFetch("/api/user", { method: "DELETE" });
      setAccessToken(null);
      navigate("/login");
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : "Failed to delete account");
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="card settings-section card--danger">
      <h2 style={{ color: "var(--color-error)" }}>Danger Zone</h2>

      <div className="form-field" style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}>
        <div>
          <p className="form-label" style={{ margin: 0 }}>Sign out</p>
          <p className="form-hint">Sign out of your account on this device.</p>
        </div>
        <button type="button" className="btn btn--secondary" onClick={handleSignOut}>
          Sign out
        </button>
      </div>

      <div className="form-field" style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}>
        <div>
          <p className="form-label" style={{ margin: 0 }}>Delete account</p>
          <p className="form-hint">Permanently delete your account and all data.</p>
        </div>
        <button
          type="button"
          className="btn"
          style={{ background: "var(--color-error)", color: "#fff", borderColor: "var(--color-error)" }}
          onClick={handleDeleteAccount}
          disabled={deleting}
        >
          {deleting ? "Deleting..." : "Delete account"}
        </button>
      </div>
      {deleteError && (
        <p className="status-message status-message--error">{deleteError}</p>
      )}
    </div>
  );
}
