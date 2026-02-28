import { type FormEvent, useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { apiFetch } from "../api/client";
import { ConfirmDialog } from "../components/ConfirmDialog";

interface OrgDetail {
  id: string;
  name: string;
  slug: string;
  subscriptionPlan: string;
  createdAt: string;
}

interface Member {
  userId: string;
  email: string;
  name: string;
  role: string;
  joinedAt: string;
}

interface Invite {
  id: string;
  email: string;
  role: string;
  expiresAt: string;
  createdAt: string;
}

interface OrgBilling {
  plan: string;
  effectivePlan: string;
  subscriptionStatus?: string;
  portalUrl?: string;
}

const ROLES = ["member", "admin", "owner"] as const;

export function OrgSettings() {
  const { id: orgId } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [org, setOrg] = useState<OrgDetail | null>(null);
  const [members, setMembers] = useState<Member[]>([]);
  const [invites, setInvites] = useState<Invite[]>([]);
  const [billing, setBilling] = useState<OrgBilling | null>(null);
  const [currentUserRole, setCurrentUserRole] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [orgName, setOrgName] = useState("");
  const [orgSlug, setOrgSlug] = useState("");
  const [nameMessage, setNameMessage] = useState("");
  const [nameError, setNameError] = useState("");
  const [savingName, setSavingName] = useState(false);

  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState("member");
  const [inviteMessage, setInviteMessage] = useState("");
  const [inviteError, setInviteError] = useState("");
  const [sendingInvite, setSendingInvite] = useState(false);

  const [billingMessage, setBillingMessage] = useState("");
  const [upgrading, setUpgrading] = useState(false);
  const [canceling, setCanceling] = useState(false);

  const [deleteError, setDeleteError] = useState("");
  const [deleting, setDeleting] = useState(false);

  const [confirmDialog, setConfirmDialog] = useState<{
    message: string;
    confirmLabel?: string;
    danger?: boolean;
    onConfirm: () => void;
  } | null>(null);

  const isOwner = currentUserRole === "owner";
  const isAdmin = currentUserRole === "admin";
  const canManage = isOwner || isAdmin;

  useEffect(() => {
    if (!orgId) return;

    Promise.all([
      apiFetch<OrgDetail>(`/api/organizations/${orgId}`),
      apiFetch<Member[]>(`/api/organizations/${orgId}/members`),
      apiFetch<Invite[]>(`/api/organizations/${orgId}/invites`).catch(() => []),
      apiFetch<OrgBilling>(`/api/organizations/${orgId}/billing`).catch(() => null),
    ])
      .then(([orgData, memberData, inviteData, billingData]) => {
        if (orgData) {
          setOrg(orgData);
          setOrgName(orgData.name);
          setOrgSlug(orgData.slug);
        }
        setMembers(memberData ?? []);
        setInvites((inviteData as Invite[]) ?? []);
        setBilling(billingData as OrgBilling | null);

        const currentMember = findCurrentUser(memberData ?? []);
        if (currentMember) {
          setCurrentUserRole(currentMember.role);
          if (currentMember.role !== "owner" && currentMember.role !== "admin") {
            navigate("/", { replace: true });
            return;
          }
        }
      })
      .catch(() => setError("Failed to load workspace"))
      .finally(() => setLoading(false));
  }, [orgId]);

  function findCurrentUser(memberList: Member[]): Member | null {
    return memberList.length > 0 ? memberList[0] : null;
  }

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

  async function handleRemoveMember(userId: string, memberName: string) {
    setConfirmDialog({
      message: `Remove ${memberName} from this workspace?`,
      confirmLabel: "Remove",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        try {
          await apiFetch(`/api/organizations/${orgId}/members/${userId}`, {
            method: "DELETE",
          });
          setMembers((prev) => prev.filter((m) => m.userId !== userId));
        } catch (err) {
          setError(err instanceof Error ? err.message : "Failed to remove member");
        }
      },
    });
  }

  async function handleRoleChange(userId: string, newRole: string) {
    try {
      await apiFetch(`/api/organizations/${orgId}/members/${userId}`, {
        method: "PATCH",
        body: JSON.stringify({ role: newRole }),
      });
      setMembers((prev) =>
        prev.map((m) => (m.userId === userId ? { ...m, role: newRole } : m))
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update role");
    }
  }

  async function handleSendInvite(event: FormEvent) {
    event.preventDefault();
    setInviteError("");
    setInviteMessage("");

    if (!inviteEmail.trim()) {
      setInviteError("Email is required");
      return;
    }

    setSendingInvite(true);
    try {
      const result = await apiFetch<Invite>(`/api/organizations/${orgId}/invites`, {
        method: "POST",
        body: JSON.stringify({ email: inviteEmail.trim(), role: inviteRole }),
      });
      if (result) {
        setInvites((prev) => [...prev, result]);
      }
      setInviteMessage("Invite sent");
      setInviteEmail("");
      setInviteRole("member");
    } catch (err) {
      setInviteError(err instanceof Error ? err.message : "Failed to send invite");
    } finally {
      setSendingInvite(false);
    }
  }

  async function handleRevokeInvite(inviteId: string) {
    try {
      await apiFetch(`/api/organizations/${orgId}/invites/${inviteId}`, {
        method: "DELETE",
      });
      setInvites((prev) => prev.filter((i) => i.id !== inviteId));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to revoke invite");
    }
  }

  async function handleUpgrade() {
    setUpgrading(true);
    setBillingMessage("");
    try {
      const resp = await apiFetch<{ checkoutUrl: string }>(
        `/api/organizations/${orgId}/billing/checkout`,
        {
          method: "POST",
          body: JSON.stringify({ plan: "pro" }),
        }
      );
      if (resp?.checkoutUrl) {
        window.location.href = resp.checkoutUrl;
      }
    } catch (err) {
      setBillingMessage(err instanceof Error ? err.message : "Failed to start checkout");
    } finally {
      setUpgrading(false);
    }
  }

  function handleCancelSubscription() {
    setConfirmDialog({
      message: "Cancel this workspace's Pro subscription? Access continues until the end of the billing period.",
      onConfirm: async () => {
        setConfirmDialog(null);
        setCanceling(true);
        setBillingMessage("");
        try {
          await apiFetch(`/api/organizations/${orgId}/billing`, {
            method: "DELETE",
          });
          setBillingMessage("Subscription canceled.");
          setBilling((b) => b ? { ...b, subscriptionStatus: "canceled" } : b);
        } catch (err) {
          setBillingMessage(err instanceof Error ? err.message : "Failed to cancel");
        } finally {
          setCanceling(false);
        }
      },
    });
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

  if (loading) {
    return (
      <div className="page-container page-container--centered">
        <p className="status-message status-message--success">Loading...</p>
      </div>
    );
  }

  if (error && !org) {
    return (
      <div className="page-container page-container--centered">
        <p className="status-message status-message--error">{error}</p>
      </div>
    );
  }

  if (!org) {
    return (
      <div className="page-container page-container--centered">
        <p className="status-message status-message--error">Workspace not found</p>
      </div>
    );
  }

  return (
    <div className="page-container">
      <h1 className="page-title">Workspace Settings</h1>

      {error && (
        <p className="status-message status-message--error">{error}</p>
      )}

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

      <div className="card settings-section">
        <h2>Members</h2>
        <p className="card-description">
          {members.length} {members.length === 1 ? "member" : "members"} in this workspace.
        </p>

        <div className="key-list">
          {members.map((member) => (
            <div key={member.userId} className="api-key-row">
              <div className="api-key-info">
                <span className="api-key-name">{member.name || member.email}</span>
                <span className="api-key-meta">
                  {member.email} — Joined {new Date(member.joinedAt).toLocaleDateString("en-GB")}
                </span>
              </div>
              <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                {isOwner && member.role !== "owner" ? (
                  <select
                    className="form-input"
                    value={member.role}
                    onChange={(e) => handleRoleChange(member.userId, e.target.value)}
                    aria-label={`Role for ${member.name || member.email}`}
                    style={{ width: "auto" }}
                  >
                    {ROLES.filter((r) => r !== "owner").map((r) => (
                      <option key={r} value={r}>{r}</option>
                    ))}
                  </select>
                ) : (
                  <span className="plan-badge">{member.role}</span>
                )}
                {canManage && member.role !== "owner" && (
                  <button
                    type="button"
                    className="btn btn--danger btn--danger-sm"
                    onClick={() => handleRemoveMember(member.userId, member.name || member.email)}
                  >
                    Remove
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>

      {canManage && (
        <div className="card settings-section">
          <h2>Invites</h2>
          <p className="card-description">
            Invite new members to this workspace by email.
          </p>

          <form onSubmit={handleSendInvite} className="api-key-form-row">
            <div className="form-field" style={{ flex: 1 }}>
              <label className="form-label" htmlFor="invite-email">Email</label>
              <input
                id="invite-email"
                type="email"
                className="form-input"
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                placeholder="colleague@example.com"
                required
              />
            </div>
            <div className="form-field">
              <label className="form-label" htmlFor="invite-role">Role</label>
              <select
                id="invite-role"
                className="form-input"
                value={inviteRole}
                onChange={(e) => setInviteRole(e.target.value)}
              >
                <option value="member">Member</option>
                <option value="admin">Admin</option>
              </select>
            </div>
            <button
              type="submit"
              className="btn btn--primary"
              disabled={sendingInvite}
            >
              {sendingInvite ? "Sending..." : "Send invite"}
            </button>
          </form>

          {inviteError && (
            <p className="status-message status-message--error">{inviteError}</p>
          )}
          {inviteMessage && (
            <p className="status-message status-message--success">{inviteMessage}</p>
          )}

          {invites.length > 0 && (
            <>
              <h3>Pending invites</h3>
              <div className="key-list">
                {invites.map((invite) => (
                  <div key={invite.id} className="api-key-row">
                    <div className="api-key-info">
                      <span className="api-key-name">{invite.email}</span>
                      <span className="api-key-meta">
                        Role: {invite.role} — Expires {new Date(invite.expiresAt).toLocaleDateString("en-GB")}
                      </span>
                    </div>
                    <button
                      type="button"
                      className="btn btn--danger btn--danger-sm"
                      onClick={() => handleRevokeInvite(invite.id)}
                    >
                      Revoke
                    </button>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      )}

      {isOwner && billing && (
        <div className="card settings-section">
          <div className="card-header">
            <h2>Billing</h2>
            <span className={`plan-badge ${billing.effectivePlan === "pro" || billing.effectivePlan === "business" ? "plan-badge--pro" : ""}`}>
              {billing.effectivePlan === "pro" || billing.effectivePlan === "business" ? "Pro" : "Free"}
            </span>
          </div>

          {billing.plan === "free" && billing.effectivePlan !== billing.plan && (
            <p className="card-description">
              This workspace has Pro features through your personal subscription. No separate workspace upgrade needed.
            </p>
          )}

          {billing.plan === "free" && billing.effectivePlan === billing.plan && !billing.subscriptionStatus && (
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
              Subscription canceled. Access continues until the end of the billing period.
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

      {confirmDialog && (
        <ConfirmDialog
          message={confirmDialog.message}
          confirmLabel={confirmDialog.confirmLabel}
          danger={confirmDialog.danger}
          onConfirm={confirmDialog.onConfirm}
          onCancel={() => setConfirmDialog(null)}
        />
      )}
    </div>
  );
}
