import { type FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { apiFetch } from "../../api/client";
import type { OrgDetail, SharedSectionProps } from "./types";

interface GeneralSectionProps extends SharedSectionProps {
  org: OrgDetail;
  setOrg: React.Dispatch<React.SetStateAction<OrgDetail | null>>;
  canManage: boolean;
  isOwner: boolean;
  retentionDays: number;
  setRetentionDays: React.Dispatch<React.SetStateAction<number>>;
}

export function GeneralSection({
  orgId,
  org,
  setOrg,
  canManage,
  isOwner,
  retentionDays,
  setRetentionDays,
  setConfirmDialog,
}: GeneralSectionProps) {
  const navigate = useNavigate();

  const [orgName, setOrgName] = useState(org.name);
  const [orgSlug, setOrgSlug] = useState(org.slug);
  const [nameMessage, setNameMessage] = useState("");
  const [nameError, setNameError] = useState("");
  const [savingName, setSavingName] = useState(false);

  const [deleteError, setDeleteError] = useState("");
  const [deleting, setDeleting] = useState(false);

  async function handleGeneralSave(event: FormEvent) {
    event.preventDefault();
    setNameError("");
    setNameMessage("");

    if (!orgName.trim()) {
      setNameError("Workspace name is required");
      return;
    }

    setSavingName(true);
    try {
      await apiFetch(`/api/organizations/${orgId}`, {
        method: "PATCH",
        body: JSON.stringify({ name: orgName.trim(), slug: orgSlug.trim() }),
      });
      setNameMessage("Workspace updated");
      setOrg((prev) => prev ? { ...prev, name: orgName.trim(), slug: orgSlug.trim() } : prev);
    } catch (err) {
      setNameError(err instanceof Error ? err.message : "Failed to update workspace");
    } finally {
      setSavingName(false);
    }
  }

  async function handleRetentionDaysChange(value: number) {
    const previous = retentionDays;
    setRetentionDays(value);
    try {
      await apiFetch(`/api/organizations/${orgId}`, {
        method: "PATCH",
        body: JSON.stringify({ retentionDays: value }),
      });
    } catch {
      setRetentionDays(previous);
    }
  }

  function handleDeleteOrg() {
    setConfirmDialog({
      message: "Are you sure you want to delete this workspace? This action cannot be undone. All workspace data will be permanently deleted.",
      confirmLabel: "Delete workspace",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        setDeleting(true);
        setDeleteError("");
        try {
          await apiFetch(`/api/organizations/${orgId}`, { method: "DELETE" });
          navigate("/settings");
        } catch (err) {
          setDeleteError(err instanceof Error ? err.message : "Failed to delete workspace");
        } finally {
          setDeleting(false);
        }
      },
    });
  }

  return (
    <>
      <form onSubmit={handleGeneralSave} className="card settings-section">
        <h2>General</h2>

        <div className="form-field">
          <label className="form-label" htmlFor="org-name">Workspace name</label>
          <input
            id="org-name"
            type="text"
            className="form-input"
            value={orgName}
            onChange={(e) => setOrgName(e.target.value)}
            disabled={!canManage}
            required
          />
        </div>

        <div className="form-field">
          <label className="form-label" htmlFor="org-slug">Slug</label>
          <input
            id="org-slug"
            type="text"
            className="form-input"
            value={orgSlug}
            onChange={(e) => setOrgSlug(e.target.value)}
            disabled={!canManage}
            required
          />
        </div>

        {nameError && (
          <p className="status-message status-message--error">{nameError}</p>
        )}
        {nameMessage && (
          <p className="status-message status-message--success">{nameMessage}</p>
        )}

        {canManage && (
          <div className="btn-row">
            <button
              type="submit"
              className="btn btn--primary"
              disabled={savingName || (orgName.trim() === org.name && orgSlug.trim() === org.slug)}
            >
              {savingName ? "Saving..." : "Save"}
            </button>
          </div>
        )}
      </form>

      {canManage && (
        <div className="card settings-section">
          <h2>Data Retention</h2>
          <p className="card-description">
            Automatically delete workspace videos after a set number of days. Pinned videos are excluded.
          </p>
          <div className="form-field">
            <label className="form-label" htmlFor="org-retention-days">Auto-delete after</label>
            <select
              id="org-retention-days"
              className="form-input"
              value={retentionDays}
              onChange={(e) => handleRetentionDaysChange(Number(e.target.value))}
            >
              <option value={0}>Off</option>
              <option value={30}>30 days</option>
              <option value={60}>60 days</option>
              <option value={90}>90 days</option>
              <option value={180}>180 days</option>
              <option value={365}>365 days</option>
            </select>
          </div>
        </div>
      )}

      {isOwner && (
        <div className="card settings-section card--danger">
          <h2 style={{ color: "var(--color-error)" }}>Danger Zone</h2>

          <div className="form-field" style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}>
            <div>
              <p className="form-label" style={{ margin: 0 }}>Delete workspace</p>
              <p className="form-hint">Permanently delete this workspace and all its data.</p>
            </div>
            <button
              type="button"
              className="btn"
              style={{ background: "var(--color-error)", color: "#fff", borderColor: "var(--color-error)" }}
              onClick={handleDeleteOrg}
              disabled={deleting}
            >
              {deleting ? "Deleting..." : "Delete workspace"}
            </button>
          </div>
          {deleteError && (
            <p className="status-message status-message--error">{deleteError}</p>
          )}
        </div>
      )}
    </>
  );
}
