import { useEffect, useState } from "react";
import { apiFetch } from "../../api/client";
import { WebhookDelivery } from "./types";

function formatJson(value: string): string {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

interface WebhookSectionProps {
  initialNotificationMode: string;
  initialWebhookUrl: string;
  initialSavedWebhookUrl: string;
  initialWebhookSecret: string;
  initialSavedSlackUrl: string;
}

export function WebhookSection({
  initialNotificationMode,
  initialWebhookUrl,
  initialSavedWebhookUrl,
  initialWebhookSecret,
  initialSavedSlackUrl,
}: WebhookSectionProps) {
  const [webhookUrl, setWebhookUrl] = useState(initialWebhookUrl);
  const [savedWebhookUrl, setSavedWebhookUrl] = useState(initialSavedWebhookUrl);
  const [webhookSecret, setWebhookSecret] = useState(initialWebhookSecret);
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

  useEffect(() => {
    if (!savedWebhookUrl) return;
    apiFetch<WebhookDelivery[]>("/api/settings/notifications/webhook-deliveries")
      .then((data) => setWebhookDeliveries(data ?? []))
      .catch(() => {});
  }, [savedWebhookUrl]);

  async function handleWebhookSave() {
    setSavingWebhook(true);
    setWebhookError("");
    setWebhookMessage("");
    try {
      await apiFetch("/api/settings/notifications", {
        method: "PUT",
        body: JSON.stringify({
          notificationMode: initialNotificationMode,
          slackWebhookUrl: initialSavedSlackUrl || undefined,
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

  return (
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
  );
}
