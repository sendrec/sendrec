import { type FormEvent, useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { apiFetch } from "../api/client";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { useOrganization } from "../hooks/useOrganization";

interface OrgDetail {
  id: string;
  name: string;
  slug: string;
  subscriptionPlan: string;
  createdAt: string;
  retentionDays?: number;
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
  acceptLink?: string;
  expiresAt: string;
  createdAt: string;
}

interface OrgBilling {
  plan: string;
  subscriptionStatus?: string;
  portalUrl?: string;
}

interface SsoConfig {
  provider: string;
  issuerUrl: string;
  clientId: string;
  configured: boolean;
  enforceSso: boolean;
  samlMetadataUrl?: string;
  samlEntityId?: string;
  samlSsoUrl?: string;
  spMetadataUrl?: string;
}

const ROLES = ["viewer", "member", "admin", "owner"] as const;

export function OrgSettings() {
  const { id: orgId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { orgs, selectedOrgId, loading: orgsLoading } = useOrganization();

  const [org, setOrg] = useState<OrgDetail | null>(null);
  const [members, setMembers] = useState<Member[]>([]);
  const [invites, setInvites] = useState<Invite[]>([]);
  const [billing, setBilling] = useState<OrgBilling | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const myOrg = orgs.find((o) => o.id === orgId);
  const currentUserRole = myOrg?.role ?? null;
  const isOwner = currentUserRole === "owner";
  const isAdmin = currentUserRole === "admin";
  const canManage = isOwner || isAdmin;

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

  const [retentionDays, setRetentionDays] = useState(0);

  const [ssoIssuerUrl, setSsoIssuerUrl] = useState("");
  const [ssoClientId, setSsoClientId] = useState("");
  const [ssoClientSecret, setSsoClientSecret] = useState("");
  const [ssoEnforce, setSsoEnforce] = useState(false);
  const [ssoConfigured, setSsoConfigured] = useState(false);
  const [ssoMessage, setSsoMessage] = useState("");
  const [ssoError, setSsoError] = useState("");
  const [savingSso, setSavingSso] = useState(false);
  const [removingSso, setRemovingSso] = useState(false);
  const [ssoProvider, setSsoProvider] = useState<"oidc" | "saml">("oidc");
  const [samlMetadataUrl, setSamlMetadataUrl] = useState("");
  const [samlMetadataXml, setSamlMetadataXml] = useState("");
  const [samlEntityId, setSamlEntityId] = useState("");
  const [samlSsoUrl, setSamlSsoUrl] = useState("");
  const [spMetadataUrl, setSpMetadataUrl] = useState("");

  const [deleteError, setDeleteError] = useState("");
  const [deleting, setDeleting] = useState(false);

  const [confirmDialog, setConfirmDialog] = useState<{
    message: string;
    confirmLabel?: string;
    danger?: boolean;
    onConfirm: () => void;
  } | null>(null);

  useEffect(() => {
    if (orgsLoading) return;
    if (!canManage || selectedOrgId !== orgId) {
      navigate("/", { replace: true });
    }
  }, [orgsLoading, canManage, selectedOrgId, orgId, navigate]);

  useEffect(() => {
    if (!orgId || !canManage || selectedOrgId !== orgId) return;

    Promise.all([
      apiFetch<OrgDetail>(`/api/organizations/${orgId}`),
      apiFetch<Member[]>(`/api/organizations/${orgId}/members`),
      apiFetch<Invite[]>(`/api/organizations/${orgId}/invites`).catch(() => []),
      apiFetch<OrgBilling>(`/api/organizations/${orgId}/billing`).catch(() => null),
    ])
      .then(async ([orgData, memberData, inviteData, billingData]) => {
        if (orgData) {
          setOrg(orgData);
          setOrgName(orgData.name);
          setOrgSlug(orgData.slug);
          if (orgData.retentionDays !== undefined) {
            setRetentionDays(orgData.retentionDays);
          }
        }
        setMembers(memberData ?? []);
        setInvites((inviteData as Invite[]) ?? []);
        setBilling(billingData as OrgBilling | null);

        const billingPlan = (billingData as OrgBilling | null)?.plan ?? orgData?.subscriptionPlan;
        if (billingPlan === "business") {
          try {
            const ssoData = await apiFetch<SsoConfig>(`/api/organizations/${orgId}/sso`);
            if (ssoData) {
              setSsoConfigured(ssoData.configured);
              setSsoEnforce(ssoData.enforceSso);
              if (ssoData.configured) {
                setSsoProvider(ssoData.provider === "saml" ? "saml" : "oidc");
                if (ssoData.provider === "saml") {
                  setSamlMetadataUrl(ssoData.samlMetadataUrl || "");
                  setSamlEntityId(ssoData.samlEntityId || "");
                  setSamlSsoUrl(ssoData.samlSsoUrl || "");
                  setSpMetadataUrl(ssoData.spMetadataUrl || "");
                } else {
                  setSsoIssuerUrl(ssoData.issuerUrl || "");
                  setSsoClientId(ssoData.clientId || "");
                }
              }
            }
          } catch { /* SSO not available */ }
        }
      })
      .catch(() => setError("Failed to load workspace"))
      .finally(() => setLoading(false));
  }, [orgId, canManage]);

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

  async function doUpgrade(plan: string) {
    setUpgrading(true);
    setBillingMessage("");
    try {
      const resp = await apiFetch<{ checkoutUrl?: string; upgraded?: string }>(
        `/api/organizations/${orgId}/billing/checkout`,
        {
          method: "POST",
          body: JSON.stringify({ plan }),
        }
      );
      if (resp?.upgraded) {
        window.location.reload();
      } else if (resp?.checkoutUrl) {
        window.location.href = resp.checkoutUrl;
      }
    } catch (err) {
      setBillingMessage(err instanceof Error ? err.message : "Failed to start checkout");
    } finally {
      setUpgrading(false);
    }
  }

  function handleUpgrade(plan: string) {
    if (billing?.subscriptionStatus && billing.subscriptionStatus !== "canceled") {
      const label = plan === "business" ? "Business" : "Pro";
      setConfirmDialog({
        message: `Upgrade to ${label}? Your remaining credit will be prorated.`,
        confirmLabel: `Upgrade to ${label}`,
        onConfirm: () => {
          setConfirmDialog(null);
          doUpgrade(plan);
        },
      });
    } else {
      doUpgrade(plan);
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

  async function handleSsoSave(event: FormEvent) {
    event.preventDefault();
    setSsoError("");
    setSsoMessage("");
    setSavingSso(true);
    try {
      const body = ssoProvider === "saml"
        ? {
            provider: "saml",
            samlMetadataUrl: samlMetadataUrl.trim() || undefined,
            samlMetadataXml: samlMetadataXml.trim() || undefined,
            enforceSso: ssoEnforce,
          }
        : {
            provider: "oidc",
            issuerUrl: ssoIssuerUrl.trim(),
            clientId: ssoClientId.trim(),
            clientSecret: ssoClientSecret || undefined,
            enforceSso: ssoEnforce,
          };
      await apiFetch(`/api/organizations/${orgId}/sso`, {
        method: "PUT",
        body: JSON.stringify(body),
      });
      setSsoMessage("SSO settings saved");
      setSsoConfigured(true);
      setSsoClientSecret("");
      setSamlMetadataXml("");
      // Reload to get parsed fields
      const ssoData = await apiFetch<SsoConfig>(`/api/organizations/${orgId}/sso`);
      if (ssoData?.provider === "saml") {
        setSamlEntityId(ssoData.samlEntityId || "");
        setSamlSsoUrl(ssoData.samlSsoUrl || "");
        setSpMetadataUrl(ssoData.spMetadataUrl || "");
      }
    } catch (err) {
      setSsoError(err instanceof Error ? err.message : "Failed to save SSO settings");
    } finally {
      setSavingSso(false);
    }
  }

  function handleRemoveSso() {
    setConfirmDialog({
      message: "Remove SSO configuration? Members will need to use password login.",
      confirmLabel: "Remove SSO",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        setRemovingSso(true);
        setSsoError("");
        setSsoMessage("");
        try {
          await apiFetch(`/api/organizations/${orgId}/sso`, { method: "DELETE" });
          setSsoIssuerUrl("");
          setSsoClientId("");
          setSsoClientSecret("");
          setSsoEnforce(false);
          setSsoConfigured(false);
          setSsoProvider("oidc");
          setSamlMetadataUrl("");
          setSamlMetadataXml("");
          setSamlEntityId("");
          setSamlSsoUrl("");
          setSpMetadataUrl("");
          setSsoMessage("SSO configuration removed");
        } catch (err) {
          setSsoError(err instanceof Error ? err.message : "Failed to remove SSO");
        } finally {
          setRemovingSso(false);
        }
      },
    });
  }

  if (orgsLoading || !canManage || loading) {
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
                <option value="viewer">Viewer</option>
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
                    {invite.acceptLink && (
                      <button
                        type="button"
                        className="btn btn--secondary btn--danger-sm"
                        onClick={() => {
                          navigator.clipboard.writeText(invite.acceptLink!);
                        }}
                      >
                        Copy link
                      </button>
                    )}
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
            <span className={`plan-badge ${billing.plan !== "free" ? "plan-badge--pro" : ""}`}>
              {billing.plan === "business" ? "Business" : billing.plan === "pro" ? "Pro" : "Free"}
            </span>
          </div>

          {billing.plan === "free" && !billing.subscriptionStatus && (
            <>
              <p className="card-description">
                Upgrade for unlimited videos and recording duration.
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
                    onClick={() => handleUpgrade("pro")}
                    disabled={upgrading}
                  >
                    {upgrading ? "Redirecting..." : "Upgrade to Pro"}
                  </button>
                </div>
              </div>
              <div className="upgrade-card">
                <div className="upgrade-card-info">
                  <span className="upgrade-card-plan">Business</span>
                  <span className="upgrade-card-desc">Everything in Pro, plus SSO and workspace access controls</span>
                </div>
                <div className="upgrade-card-actions">
                  <span className="upgrade-card-price">&euro;12/mo</span>
                  <button
                    type="button"
                    className="btn btn--primary"
                    onClick={() => handleUpgrade("business")}
                    disabled={upgrading}
                  >
                    {upgrading ? "Redirecting..." : "Upgrade to Business"}
                  </button>
                </div>
              </div>
            </>
          )}

          {billing.plan === "pro" && billing.subscriptionStatus !== "canceled" && (
            <div className="upgrade-card">
              <div className="upgrade-card-info">
                <span className="upgrade-card-plan">Business</span>
                <span className="upgrade-card-desc">Everything in Pro, plus SSO and workspace access controls</span>
              </div>
              <div className="upgrade-card-actions">
                <span className="upgrade-card-price">&euro;12/mo</span>
                <button
                  type="button"
                  className="btn btn--primary"
                  onClick={() => handleUpgrade("business")}
                  disabled={upgrading}
                >
                  {upgrading ? "Redirecting..." : "Upgrade to Business"}
                </button>
              </div>
            </div>
          )}

          {billing.subscriptionStatus === "canceled" && (
            <p className="card-description">
              Subscription canceled. Access continues until the end of the billing period.
            </p>
          )}

          {(billing.plan === "pro" || billing.plan === "business") && billing.subscriptionStatus !== "canceled" && (
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

      {canManage && (billing?.plan === "business" || org.subscriptionPlan === "business") && (
        <form onSubmit={handleSsoSave} className="card settings-section">
          <h2>Single Sign-On</h2>
          <p className="card-description">
            Configure single sign-on for your workspace. Members can sign in using your identity provider.
          </p>

          <div className="form-field">
            <label className="form-label">Protocol</label>
            <div style={{ display: "flex", gap: "1rem" }}>
              <label style={{ display: "flex", alignItems: "center", gap: "0.25rem" }}>
                <input
                  type="radio"
                  name="sso-protocol"
                  value="oidc"
                  checked={ssoProvider === "oidc"}
                  onChange={() => setSsoProvider("oidc")}
                />
                OIDC
              </label>
              <label style={{ display: "flex", alignItems: "center", gap: "0.25rem" }}>
                <input
                  type="radio"
                  name="sso-protocol"
                  value="saml"
                  checked={ssoProvider === "saml"}
                  onChange={() => setSsoProvider("saml")}
                />
                SAML
              </label>
            </div>
          </div>

          {ssoProvider === "oidc" ? (
            <>
              <div className="form-field">
                <label className="form-label" htmlFor="sso-issuer-url">Issuer URL</label>
                <input
                  id="sso-issuer-url"
                  type="url"
                  className="form-input"
                  value={ssoIssuerUrl}
                  onChange={(e) => setSsoIssuerUrl(e.target.value)}
                  placeholder="https://accounts.google.com"
                />
              </div>

              <div className="form-field">
                <label className="form-label" htmlFor="sso-client-id">Client ID</label>
                <input
                  id="sso-client-id"
                  type="text"
                  className="form-input"
                  value={ssoClientId}
                  onChange={(e) => setSsoClientId(e.target.value)}
                />
              </div>

              <div className="form-field">
                <label className="form-label" htmlFor="sso-client-secret">Client Secret</label>
                <input
                  id="sso-client-secret"
                  type="password"
                  className="form-input"
                  value={ssoClientSecret}
                  onChange={(e) => setSsoClientSecret(e.target.value)}
                  placeholder={ssoConfigured ? "Unchanged" : ""}
                />
              </div>
              <details className="settings-details">
                <summary>OIDC setup guide</summary>
                <pre>{`1. In your IdP (Google, Okta, Auth0, Azure AD, etc.),
   create an OAuth2/OpenID Connect application.

2. Set the redirect URI to:
   ${window.location.origin}/api/auth/sso/org/callback

3. Copy the following into the fields above:
   • Issuer URL — your IdP's OpenID discovery URL
     (e.g. https://accounts.google.com)
   • Client ID — from the application you created
   • Client Secret — from the application you created

4. Click "Save SSO settings" and test the login flow.

Common issuer URLs:
  Google:   https://accounts.google.com
  Okta:     https://your-org.okta.com
  Auth0:    https://your-tenant.auth0.com
  Azure AD: https://login.microsoftonline.com/{tenant}/v2.0`}</pre>
              </details>
            </>
          ) : (
            <>
              <div className="form-field">
                <label className="form-label" htmlFor="saml-metadata-url">Metadata URL</label>
                <input
                  id="saml-metadata-url"
                  type="url"
                  className="form-input"
                  placeholder="https://your-idp.com/saml/metadata"
                  value={samlMetadataUrl}
                  onChange={(e) => setSamlMetadataUrl(e.target.value)}
                />
              </div>
              <div className="form-field">
                <label className="form-label" htmlFor="saml-metadata-xml">Or paste metadata XML</label>
                <textarea
                  id="saml-metadata-xml"
                  className="form-input"
                  rows={4}
                  placeholder="<EntityDescriptor ...>"
                  value={samlMetadataXml}
                  onChange={(e) => setSamlMetadataXml(e.target.value)}
                />
              </div>
              {samlEntityId && (
                <>
                  <div className="form-field">
                    <label className="form-label">IdP Entity ID</label>
                    <input className="form-input" value={samlEntityId} readOnly />
                  </div>
                  <div className="form-field">
                    <label className="form-label">IdP SSO URL</label>
                    <input className="form-input" value={samlSsoUrl} readOnly />
                  </div>
                </>
              )}
              {spMetadataUrl && (
                <div className="form-field">
                  <label className="form-label">SP Metadata URL</label>
                  <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
                    <input className="form-input" value={spMetadataUrl} readOnly style={{ flex: 1 }} />
                    <button
                      type="button"
                      className="btn btn--secondary btn--sm"
                      onClick={() => navigator.clipboard.writeText(spMetadataUrl)}
                    >
                      Copy
                    </button>
                  </div>
                  <small className="form-hint">Provide this URL to your IdP administrator</small>
                </div>
              )}
              <details className="settings-details">
                <summary>SAML setup guide</summary>
                <pre>{`1. In your IdP (Okta, Auth0, Azure AD, OneLogin, etc.),
   create a SAML 2.0 application.

2. Configure the IdP with these SP values:
   • SP Entity ID / Audience:
     ${window.location.origin}/api/auth/saml/${orgId}/metadata
   • ACS URL (Assertion Consumer Service):
     ${window.location.origin}/api/auth/saml/${orgId}/acs
   • NameID format: Email address
   ${spMetadataUrl ? `\n   Or import the SP Metadata URL shown above — your
   IdP will auto-configure from it.` : `\n   After saving, the SP Metadata URL will be shown
   above — your IdP can auto-configure from it.`}

3. From your IdP, copy the metadata URL and paste it
   in "Metadata URL" above. Or download the metadata
   XML and paste it in the text area.

4. Click "Save SSO settings". The IdP Entity ID and
   SSO URL will be extracted automatically.

5. Test the login flow by signing in with SSO.

Attribute mapping (sent in SAML assertion):
  email — required (NameID or attribute)
  name  — optional (displayName or name)`}</pre>
              </details>
            </>
          )}

          <div className="form-field" style={{ flexDirection: "row", alignItems: "center", gap: 8 }}>
            <input
              id="sso-enforce"
              type="checkbox"
              checked={ssoEnforce}
              onChange={(e) => setSsoEnforce(e.target.checked)}
              style={{ width: "auto" }}
            />
            <label htmlFor="sso-enforce" className="form-label" style={{ margin: 0 }}>
              Enforce SSO for all members
            </label>
          </div>
          {ssoEnforce && (
            <p className="form-hint" style={{ color: "var(--color-warning)" }}>
              When enforced, members must sign in through your identity provider. Password login will be disabled for workspace members.
            </p>
          )}

          {ssoError && (
            <p className="status-message status-message--error">{ssoError}</p>
          )}
          {ssoMessage && (
            <p className="status-message status-message--success">{ssoMessage}</p>
          )}

          <div className="btn-row">
            <button
              type="submit"
              className="btn btn--primary"
              disabled={
                savingSso ||
                (ssoProvider === "oidc" && (!ssoIssuerUrl.trim() || !ssoClientId.trim())) ||
                (ssoProvider === "saml" && !samlMetadataUrl.trim() && !samlMetadataXml.trim())
              }
            >
              {savingSso ? "Saving..." : "Save SSO settings"}
            </button>
            {ssoConfigured && (
              <button
                type="button"
                className="btn btn--danger"
                onClick={handleRemoveSso}
                disabled={removingSso}
              >
                {removingSso ? "Removing..." : "Remove SSO"}
              </button>
            )}
          </div>
        </form>
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
