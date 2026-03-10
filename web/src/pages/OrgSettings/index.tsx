import { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { apiFetch } from "../../api/client";
import { ConfirmDialog, type ConfirmDialogState } from "../../components/ConfirmDialog";
import { useOrganization } from "../../hooks/useOrganization";
import type { Invite, Member, OrgBilling, OrgDetail, SsoConfig } from "./types";
import { GeneralSection } from "./GeneralSection";
import { MembersSection } from "./MembersSection";
import { BillingSection } from "./BillingSection";
import { SSOSection } from "./SSOSection";

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

  const [retentionDays, setRetentionDays] = useState(0);

  const [ssoIssuerUrl, setSsoIssuerUrl] = useState("");
  const [ssoClientId, setSsoClientId] = useState("");
  const [ssoEnforce, setSsoEnforce] = useState(false);
  const [ssoConfigured, setSsoConfigured] = useState(false);
  const [ssoProvider, setSsoProvider] = useState<"oidc" | "saml">("oidc");
  const [samlMetadataUrl, setSamlMetadataUrl] = useState("");
  const [samlEntityId, setSamlEntityId] = useState("");
  const [samlSsoUrl, setSamlSsoUrl] = useState("");
  const [spMetadataUrl, setSpMetadataUrl] = useState("");

  const [scimConfigured, setScimConfigured] = useState(false);
  const [scimCreatedAt, setScimCreatedAt] = useState("");
  const [scimToken, setScimToken] = useState("");
  const [scimGenerating, setScimGenerating] = useState(false);
  const [scimError, setScimError] = useState("");
  const [scimMessage, setScimMessage] = useState("");

  const [confirmDialog, setConfirmDialog] = useState<ConfirmDialogState | null>(null);

  useEffect(() => {
    if (orgsLoading) return;
    if (!canManage || selectedOrgId !== orgId) {
      navigate("/", { replace: true });
    }
  }, [orgsLoading, canManage, selectedOrgId, orgId, navigate]);

  useEffect(() => {
    if (!orgId || !canManage || selectedOrgId !== orgId) return;

    setLoading(true);
    setError("");
    setScimConfigured(false);
    setScimCreatedAt("");
    setScimToken("");
    setScimError("");
    setScimMessage("");

    Promise.all([
      apiFetch<OrgDetail>(`/api/organizations/${orgId}`),
      apiFetch<Member[]>(`/api/organizations/${orgId}/members`),
      apiFetch<Invite[]>(`/api/organizations/${orgId}/invites`).catch(() => []),
      apiFetch<OrgBilling>(`/api/organizations/${orgId}/billing`).catch(() => null),
    ])
      .then(async ([orgData, memberData, inviteData, billingData]) => {
        if (orgData) {
          setOrg(orgData);
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

          try {
            const scimData = await apiFetch<{ configured: boolean; createdAt?: string }>(
              `/api/organizations/${orgId}/scim-token`
            );
            if (scimData) {
              setScimConfigured(scimData.configured);
              setScimCreatedAt(scimData.createdAt || "");
              setScimToken("");
            }
          } catch {
            setScimConfigured(false);
            setScimCreatedAt("");
            setScimToken("");
            setScimError("Failed to load SCIM status");
          }
        }
      })
      .catch(() => setError("Failed to load workspace"))
      .finally(() => setLoading(false));
  }, [orgId, canManage, selectedOrgId]);

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

  if (!org || !orgId) {
    return (
      <div className="page-container page-container--centered">
        <p className="status-message status-message--error">Workspace not found</p>
      </div>
    );
  }

  const showSso = canManage && (billing?.plan === "business" || org.subscriptionPlan === "business");

  return (
    <div className="page-container">
      <h1 className="page-title">Workspace Settings</h1>

      {error && (
        <p className="status-message status-message--error">{error}</p>
      )}

      <GeneralSection
        orgId={orgId}
        org={org}
        setOrg={setOrg}
        canManage={canManage}
        isOwner={isOwner}
        retentionDays={retentionDays}
        setRetentionDays={setRetentionDays}
        setConfirmDialog={setConfirmDialog}
        setError={setError}
      />

      <MembersSection
        orgId={orgId}
        members={members}
        setMembers={setMembers}
        invites={invites}
        setInvites={setInvites}
        canManage={canManage}
        isOwner={isOwner}
        setConfirmDialog={setConfirmDialog}
        setError={setError}
      />

      {isOwner && billing && (
        <BillingSection
          orgId={orgId}
          billing={billing}
          setBilling={setBilling}
          setConfirmDialog={setConfirmDialog}
          setError={setError}
        />
      )}

      {showSso && (
        <SSOSection
          orgId={orgId}
          setConfirmDialog={setConfirmDialog}
          setError={setError}
          ssoConfigured={ssoConfigured}
          setSsoConfigured={setSsoConfigured}
          ssoProvider={ssoProvider}
          setSsoProvider={setSsoProvider}
          ssoIssuerUrl={ssoIssuerUrl}
          setSsoIssuerUrl={setSsoIssuerUrl}
          ssoClientId={ssoClientId}
          setSsoClientId={setSsoClientId}
          ssoEnforce={ssoEnforce}
          setSsoEnforce={setSsoEnforce}
          samlMetadataUrl={samlMetadataUrl}
          setSamlMetadataUrl={setSamlMetadataUrl}
          samlEntityId={samlEntityId}
          setSamlEntityId={setSamlEntityId}
          samlSsoUrl={samlSsoUrl}
          setSamlSsoUrl={setSamlSsoUrl}
          spMetadataUrl={spMetadataUrl}
          setSpMetadataUrl={setSpMetadataUrl}
          scimConfigured={scimConfigured}
          setScimConfigured={setScimConfigured}
          scimCreatedAt={scimCreatedAt}
          setScimCreatedAt={setScimCreatedAt}
          scimToken={scimToken}
          setScimToken={setScimToken}
          scimGenerating={scimGenerating}
          setScimGenerating={setScimGenerating}
          scimError={scimError}
          setScimError={setScimError}
          scimMessage={scimMessage}
          setScimMessage={setScimMessage}
        />
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
