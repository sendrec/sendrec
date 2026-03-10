import { LimitsResponse } from "../../types/limits";

export interface UserProfile {
  name: string;
  email: string;
  transcriptionLanguage?: string;
  noiseReduction?: boolean;
  retentionDays?: number;
}

export interface APIKeyItem {
  id: string;
  name: string;
  createdAt: string;
  lastUsedAt: string | null;
}

export interface WebhookDelivery {
  id: string;
  event: string;
  payload: string;
  statusCode: number;
  responseBody: string;
  attempt: number;
  createdAt: string;
}

export interface BrandingSettings {
  companyName: string | null;
  logoKey: string | null;
  colorBackground: string | null;
  colorSurface: string | null;
  colorText: string | null;
  colorAccent: string | null;
  footerText: string | null;
  customCss: string | null;
}

export interface IntegrationConfig {
  id?: string;
  provider: string;
  config: Record<string, string>;
}

export interface LinkedIdentity {
  provider: string;
  email: string;
}

export interface IdentitiesResponse {
  identities: LinkedIdentity[];
  hasPassword: boolean;
}

export interface BillingData {
  plan: string;
  subscriptionId: string | null;
  subscriptionStatus: string | null;
  portalUrl: string | null;
}

export interface ConfirmDialogSetter {
  (state: import("../../components/ConfirmDialog").ConfirmDialogState | null): void;
}

export interface SharedSettingsProps {
  limits: LimitsResponse | null;
}
