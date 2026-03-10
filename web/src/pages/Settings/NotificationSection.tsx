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

  return (
    <>
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
    </>
  );
}
