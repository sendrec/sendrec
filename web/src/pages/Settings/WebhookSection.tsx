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
      setWebhookError(err instanceof Error ? err.message : "Speichern fehlgeschlagen");
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
      setWebhookMessage("Testereignis gesendet");
      const data = await apiFetch<WebhookDelivery[]>("/api/settings/notifications/webhook-deliveries");
      setWebhookDeliveries(data ?? []);
    } catch (err) {
      setWebhookError(err instanceof Error ? err.message : "Test konnte nicht gesendet werden");
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
      setWebhookMessage("Secret neu erzeugt");
    } catch (err) {
      setWebhookError(err instanceof Error ? err.message : "Neuerzeugung fehlgeschlagen");
    } finally {
      setRegeneratingSecret(false);
    }
  }

  return (
    <div className="card settings-section">
      <h2>Webhooks</h2>
      <p className="card-description">
        Empfange HTTP-POST-Benachrichtigungen zu Video-Ereignissen. Nutzbar mit n8n, Zapier oder eigenen Integrationen.
      </p>

      <div className="form-field">
        <label className="form-label">Webhook-URL</label>
        <input
          type="url"
          className="form-input"
          value={webhookUrl}
          onChange={(e) => setWebhookUrl(e.target.value)}
          placeholder="https://example.com/webhook"
        />
        <span className="form-hint">HTTP-POST-Benachrichtigungen für Video-Ereignisse empfangen (n8n, Zapier, eigene Systeme).</span>
      </div>

      {webhookSecret && (
        <div className="form-field">
          <span className="form-label">Signatur-Secret</span>
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
              {copiedSecret ? "Copied" : "Kopieren"}
            </button>
            <button
              type="button"
              className="btn btn--secondary"
              onClick={handleRegenerateSecret}
              disabled={regeneratingSecret}
            >
              {regeneratingSecret ? "Wird neu erzeugt..." : "Neu erstellen"}
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
          {savingWebhook ? "Wird gespeichert..." : "Webhook speichern"}
        </button>
        <button
          type="button"
          className="btn btn--secondary"
          onClick={handleWebhookTest}
          disabled={testingWebhook || !savedWebhookUrl}
        >
          {testingWebhook ? "Wird gesendet..." : "Testereignis senden"}
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
            <h3 className="delivery-list-title">Letzte Zustellungen</h3>
            <div className="delivery-toolbar">
              <input
                type="text"
                className="delivery-search"
                placeholder="Nach Ereignis filtern..."
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
                    {f === "all" ? "Alle" : f === "success" ? "Erfolgreich" : "Fehler"}
                  </button>
                ))}
              </div>
            </div>
            <div className="delivery-scroll">
              {filtered.length === 0 ? (
                <p className="delivery-empty">Keine passenden Zustellungen</p>
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
                            <span className="delivery-detail-label">Nutzdaten</span>
                            <pre className="delivery-detail-pre">
                              {formatJson(delivery.payload)}
                            </pre>
                          </div>
                          {delivery.responseBody && (
                            <div>
                              <span className="delivery-detail-label">Antwort</span>
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
        <summary>Unterstützte Ereignisse</summary>
        <ul>
          <li><code>video.viewed</code> — Ein Zuschauer hat ein Video angesehen</li>
          <li><code>video.comment.created</code> — Ein neuer Kommentar wurde veröffentlicht</li>
          <li><code>video.reaction.created</code> — Eine Emoji-Reaktion wurde hinzugefügt</li>
          <li><code>video.transcription.ready</code> — Transkription abgeschlossen</li>
          <li><code>video.summary.ready</code> — KI-Zusammenfassung abgeschlossen</li>
          <li><code>video.cta.clicked</code> — Ein CTA-Button wurde angeklickt</li>
          <li><code>test</code> — Testereignis aus den Einstellungen</li>
        </ul>
      </details>
    </div>
  );
}
