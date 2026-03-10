import type { ConfirmDialogState } from "../../components/ConfirmDialog";

export interface OrgDetail {
  id: string;
  name: string;
  slug: string;
  subscriptionPlan: string;
  createdAt: string;
  retentionDays?: number;
}

export interface Member {
  userId: string;
  email: string;
  name: string;
  role: string;
  joinedAt: string;
}

export interface Invite {
  id: string;
  email: string;
  role: string;
  acceptLink?: string;
  expiresAt: string;
  createdAt: string;
}

export interface OrgBilling {
  plan: string;
  subscriptionStatus?: string;
  portalUrl?: string;
}

export interface SsoConfig {
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

export const ROLES = ["viewer", "member", "admin", "owner"] as const;

export interface SharedSectionProps {
  orgId: string;
  setConfirmDialog: (state: ConfirmDialogState | null) => void;
  setError: (error: string) => void;
}
