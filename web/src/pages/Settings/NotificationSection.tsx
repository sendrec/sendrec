import { useState } from "react";
import { apiFetch } from "../../api/client";

interface NotificationSectionProps {
  initialNotificationMode: string;
  initialSlackWebhookUrl: string;
  initialSavedSlackUrl: string;
}

export function NotificationSection({
  initialNotificationMode,
  initialSlackWebhookUrl,
  initialSavedSlackUrl,
}: NotificationSectionProps) {
  const [notificationMode, setNotificationMode] = useState(initialNotificationMode);
  const [notificationMessage, setNotificationMessage] = useState("");
  const [slackWebhookUrl, setSlackWebhookUrl] = useState(initialSlackWebhookUrl);
  const [savedSlackUrl, setSavedSlackUrl] = useState(initialSavedSlackUrl);
  const [slackMessage, setSlackMessage] = useState("");
  const [slackError, setSlackError] = useState("");
  const [savingSlack, setSavingSlack] = useState(false);
  const [testingSlack, setTestingSlack] = useState(false);

  async function handleNotificationChange(value: string) {
    setNotificationMessage("");
    const previous = notificationMode;
    setNotificationMode(value);
    try {
      await apiFetch("/api/settings/notifications", {
        method: "PUT",
        body: JSON.stringify({ notificationMode: value }),
      });
      setNotificationMessage("Einstellung gespeichert");
    } catch {
      setNotificationMode(previous);
      setNotificationMessage("Speichern fehlgeschlagen");
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
      setSlackMessage("Webhook-URL gespeichert");
    } catch (err) {
      setSlackError(err instanceof Error ? err.message : "Webhook-URL konnte nicht gespeichert werden");
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
      setSlackMessage("Testnachricht gesendet");
    } catch (err) {
      setSlackError(err instanceof Error ? err.message : "Testnachricht konnte nicht gesendet werden");
    } finally {
      setTestingSlack(false);
    }
  }

  return (
    <>
      <div className="card settings-section">
        <h2>E-Mail-Benachrichtigungen</h2>
        <p className="card-description">
          Lege fest, wann du E-Mail-Benachrichtigungen zu Aufrufen und Kommentaren erhältst.
        </p>

        <div className="form-field">
          <label className="form-label" htmlFor="notification-mode">Benachrichtigungen</label>
          <select
            id="notification-mode"
            className="form-input"
            value={notificationMode}
            onChange={(e) => handleNotificationChange(e.target.value)}
          >
            <option value="off">Aus</option>
            <option value="views_only">Nur Aufrufe</option>
            <option value="comments_only">Nur Kommentare</option>
            <option value="views_and_comments">Aufrufe + Kommentare</option>
            <option value="digest">Tägliche Zusammenfassung (Aufrufe + Kommentare)</option>
          </select>
        </div>

        {notificationMessage && (
          <p className={`status-message ${notificationMessage === "Speichern fehlgeschlagen" ? "status-message--error" : "status-message--success"}`}>{notificationMessage}</p>
        )}
      </div>

      <div className="card settings-section">
        <h2>Slack-Benachrichtigungen</h2>
        <p className="card-description">
          Sende Benachrichtigungen zu Videoaufrufen und Kommentaren an einen Slack-Kanal.
        </p>

        <div className="form-field">
          <label className="form-label">Slack-Webhook-URL</label>
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
            {savingSlack ? "Wird gespeichert..." : "Speichern"}
          </button>
          <button
            type="button"
            className="btn btn--secondary"
            onClick={handleSlackTest}
            disabled={testingSlack || !savedSlackUrl}
          >
            {testingSlack ? "Wird gesendet..." : "Testnachricht senden"}
          </button>
        </div>

        {slackError && (
          <p className="status-message status-message--error">{slackError}</p>
        )}
        {slackMessage && (
          <p className="status-message status-message--success">{slackMessage}</p>
        )}

        <details className="settings-details">
          <summary>So erhältst du eine Webhook-URL</summary>
          <ol>
            <li>Gehe zu <a href="https://api.slack.com/apps" target="_blank" rel="noopener noreferrer">api.slack.com/apps</a></li>
            <li>Klicke auf <strong>Create New App</strong> und wähle <strong>From scratch</strong></li>
            <li>Unter <strong>Features</strong>, wähle <strong>Incoming Webhooks</strong></li>
            <li>Aktiviere Webhooks und klicke auf <strong>Add New Webhook to Workspace</strong></li>
            <li>Wähle einen Kanal und kopiere die Webhook-URL</li>
          </ol>
        </details>
      </div>
    </>
  );
}
