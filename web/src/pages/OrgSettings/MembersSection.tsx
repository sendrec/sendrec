import { type FormEvent, useState } from "react";
import { apiFetch } from "../../api/client";
import type { Invite, Member, SharedSectionProps } from "./types";
import { ROLES } from "./types";

interface MembersSectionProps extends SharedSectionProps {
  members: Member[];
  setMembers: React.Dispatch<React.SetStateAction<Member[]>>;
  invites: Invite[];
  setInvites: React.Dispatch<React.SetStateAction<Invite[]>>;
  canManage: boolean;
  isOwner: boolean;
}

export function MembersSection({
  orgId,
  members,
  setMembers,
  invites,
  setInvites,
  canManage,
  isOwner,
  setConfirmDialog,
  setError,
}: MembersSectionProps) {
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState("member");
  const [inviteMessage, setInviteMessage] = useState("");
  const [inviteError, setInviteError] = useState("");
  const [sendingInvite, setSendingInvite] = useState(false);

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

  return (
    <>
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
    </>
  );
}
