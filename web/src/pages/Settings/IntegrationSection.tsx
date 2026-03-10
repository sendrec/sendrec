import { useState } from "react";
import { apiFetch } from "../../api/client";
import { IntegrationConfig } from "./types";

interface IntegrationSectionProps {
  initialIntegrations: IntegrationConfig[];
  initialGhToken: string;
  initialGhOwner: string;
  initialGhRepo: string;
  initialJiraBaseUrl: string;
  initialJiraEmail: string;
  initialJiraApiToken: string;
  initialJiraProjectKey: string;
}

export function IntegrationSection({
  initialIntegrations,
  initialGhToken,
  initialGhOwner,
  initialGhRepo,
  initialJiraBaseUrl,
  initialJiraEmail,
  initialJiraApiToken,
  initialJiraProjectKey,
}: IntegrationSectionProps) {
  const [integrations, setIntegrations] = useState(initialIntegrations);
  const [intgExpanded, setIntgExpanded] = useState<string | null>(null);
  const [intgSaving, setIntgSaving] = useState(false);
  const [intgMessage, setIntgMessage] = useState("");
  const [intgError, setIntgError] = useState("");
  const [ghToken, setGhToken] = useState(initialGhToken);
  const [ghOwner, setGhOwner] = useState(initialGhOwner);
  const [ghRepo, setGhRepo] = useState(initialGhRepo);
  const [jiraBaseUrl, setJiraBaseUrl] = useState(initialJiraBaseUrl);
  const [jiraEmail, setJiraEmail] = useState(initialJiraEmail);
  const [jiraApiToken, setJiraApiToken] = useState(initialJiraApiToken);
  const [jiraProjectKey, setJiraProjectKey] = useState(initialJiraProjectKey);

  async function saveIntegration(provider: string) {
    setIntgSaving(true);
    setIntgError("");
    setIntgMessage("");
    try {
      const config = provider === "github"
        ? { token: ghToken, owner: ghOwner, repo: ghRepo }
        : { base_url: jiraBaseUrl, email: jiraEmail, api_token: jiraApiToken, project_key: jiraProjectKey };
      await apiFetch(`/api/settings/integrations/${provider}`, { method: "PUT", body: JSON.stringify(config) });
      setIntgMessage("Saved");
      const data = await apiFetch<IntegrationConfig[]>("/api/settings/integrations");
      if (data) setIntegrations(data);
    } catch (err) {
      setIntgError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setIntgSaving(false);
    }
  }

  async function testIntegration(provider: string) {
    setIntgError("");
    setIntgMessage("");
    try {
      await apiFetch(`/api/settings/integrations/${provider}/test`, { method: "POST" });
      setIntgMessage("Connected successfully");
    } catch (err) {
      setIntgError(err instanceof Error ? err.message : "Connection failed");
    }
  }

  async function deleteIntegration(provider: string) {
    setIntgError("");
    setIntgMessage("");
    try {
      await apiFetch(`/api/settings/integrations/${provider}`, { method: "DELETE" });
      setIntegrations((prev) => prev.filter((i) => i.provider !== provider));
      setIntgMessage("Disconnected");
    } catch (err) {
      setIntgError(err instanceof Error ? err.message : "Failed to disconnect");
    }
  }

  return (
    <div className="card settings-section">
      <h2>Integrations</h2>
      <p className="card-description">Connect external services to create issues from videos.</p>
      {intgMessage && <p className="status-message status-message--success">{intgMessage}</p>}
      {intgError && <p className="status-message status-message--error">{intgError}</p>}

      {["github", "jira"].map((provider) => {
        const connected = integrations.some((i) => i.provider === provider);
        const expanded = intgExpanded === provider;
        return (
          <div key={provider} className="form-field">
            <div
              style={{ display: "flex", justifyContent: "space-between", alignItems: "center", cursor: "pointer" }}
              onClick={() => setIntgExpanded(expanded ? null : provider)}
            >
              <span style={{ fontWeight: 500 }}>{provider === "github" ? "GitHub" : "Jira"}</span>
              <span style={{ fontSize: "0.85rem", color: connected ? "var(--color-success, #22c55e)" : "var(--color-text-muted)" }}>
                {connected ? "Connected" : "Not connected"}
              </span>
            </div>
            {expanded && (
              <div style={{ marginTop: "0.75rem" }}>
                {provider === "github" ? (
                  <>
                    <div className="form-field">
                      <label className="form-label">Personal access token</label>
                      <input className="form-input" type="password" value={ghToken} onChange={(e) => setGhToken(e.target.value)} placeholder="ghp_..." />
                    </div>
                    <div className="form-field">
                      <label className="form-label">Owner</label>
                      <input className="form-input" value={ghOwner} onChange={(e) => setGhOwner(e.target.value)} placeholder="org-or-user" />
                    </div>
                    <div className="form-field">
                      <label className="form-label">Repository</label>
                      <input className="form-input" value={ghRepo} onChange={(e) => setGhRepo(e.target.value)} placeholder="repo-name" />
                    </div>
                  </>
                ) : (
                  <>
                    <div className="form-field">
                      <label className="form-label">Jira URL</label>
                      <input className="form-input" value={jiraBaseUrl} onChange={(e) => setJiraBaseUrl(e.target.value)} placeholder="https://yourteam.atlassian.net" />
                    </div>
                    <div className="form-field">
                      <label className="form-label">Email</label>
                      <input className="form-input" type="email" value={jiraEmail} onChange={(e) => setJiraEmail(e.target.value)} />
                    </div>
                    <div className="form-field">
                      <label className="form-label">API Token</label>
                      <input className="form-input" type="password" value={jiraApiToken} onChange={(e) => setJiraApiToken(e.target.value)} />
                    </div>
                    <div className="form-field">
                      <label className="form-label">Project Key</label>
                      <input className="form-input" value={jiraProjectKey} onChange={(e) => setJiraProjectKey(e.target.value)} placeholder="PROJ" />
                    </div>
                  </>
                )}
                <div style={{ display: "flex", gap: "0.5rem", marginTop: "0.5rem" }}>
                  <button className="btn btn--primary" onClick={() => saveIntegration(provider)} disabled={intgSaving}>
                    {intgSaving ? "Saving..." : "Save"}
                  </button>
                  <button className="btn btn--secondary" onClick={() => testIntegration(provider)}>Test Connection</button>
                  {connected && (
                    <button className="btn btn--danger" onClick={() => deleteIntegration(provider)}>Disconnect</button>
                  )}
                </div>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
