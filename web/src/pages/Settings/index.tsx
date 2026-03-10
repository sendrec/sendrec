import { useEffect, useState } from "react";
import { apiFetch } from "../../api/client";
import { LimitsResponse } from "../../types/limits";
import {
  UserProfile,
  APIKeyItem,
  BrandingSettings,
  IntegrationConfig,
  LinkedIdentity,
  IdentitiesResponse,
  BillingData,
} from "./types";
import { BillingSection } from "./BillingSection";
import { ProfileSection } from "./ProfileSection";
import { SecuritySection } from "./SecuritySection";
import { NotificationSection } from "./NotificationSection";
import { WebhookSection } from "./WebhookSection";
import { IntegrationSection } from "./IntegrationSection";
import { BrandingSection } from "./BrandingSection";

interface LoadedState {
  profile: UserProfile;
  notificationMode: string;
  slackWebhookUrl: string;
  savedSlackUrl: string;
  webhookUrl: string;
  savedWebhookUrl: string;
  webhookSecret: string;
  limits: LimitsResponse | null;
  apiKeys: APIKeyItem[];
  brandingEnabled: boolean;
  branding: BrandingSettings;
  billingEnabled: boolean;
  billing: BillingData | null;
  transcriptionEnabled: boolean;
  noiseReductionEnabled: boolean;
  transcriptionLanguage: string;
  noiseReduction: boolean;
  retentionDays: number;
  integrations: IntegrationConfig[];
  ghToken: string;
  ghOwner: string;
  ghRepo: string;
  jiraBaseUrl: string;
  jiraEmail: string;
  jiraApiToken: string;
  jiraProjectKey: string;
  identities: LinkedIdentity[];
  identityHasPassword: boolean;
}

export function Settings() {
  const [loaded, setLoaded] = useState<LoadedState | null>(null);

  useEffect(() => {
    async function fetchProfile() {
      const state: Partial<LoadedState> = {
        notificationMode: "off",
        slackWebhookUrl: "",
        savedSlackUrl: "",
        webhookUrl: "",
        savedWebhookUrl: "",
        webhookSecret: "",
        apiKeys: [],
        brandingEnabled: false,
        branding: {
          companyName: null, logoKey: null,
          colorBackground: null, colorSurface: null, colorText: null, colorAccent: null,
          footerText: null, customCss: null,
        },
        billingEnabled: false,
        billing: null,
        transcriptionEnabled: false,
        noiseReductionEnabled: false,
        transcriptionLanguage: "auto",
        noiseReduction: true,
        retentionDays: 0,
        integrations: [],
        ghToken: "",
        ghOwner: "",
        ghRepo: "",
        jiraBaseUrl: "",
        jiraEmail: "",
        jiraApiToken: "",
        jiraProjectKey: "",
        identities: [],
        identityHasPassword: false,
      };

      try {
        const [result, notifPrefs, limits, keys] = await Promise.all([
          apiFetch<UserProfile>("/api/user"),
          apiFetch<{ notificationMode: string; slackWebhookUrl: string | null; webhookUrl: string | null; webhookSecret: string | null }>("/api/settings/notifications"),
          apiFetch<LimitsResponse>("/api/videos/limits"),
          apiFetch<APIKeyItem[]>("/api/settings/api-keys"),
        ]);
        if (result) {
          state.profile = result;
          if (result.transcriptionLanguage) {
            state.transcriptionLanguage = result.transcriptionLanguage;
          }
          if (result.noiseReduction !== undefined) {
            state.noiseReduction = result.noiseReduction;
          }
          if (result.retentionDays !== undefined) {
            state.retentionDays = result.retentionDays;
          }
        }
        if (notifPrefs) {
          state.notificationMode = notifPrefs.notificationMode;
          if (notifPrefs.slackWebhookUrl) {
            state.slackWebhookUrl = notifPrefs.slackWebhookUrl;
            state.savedSlackUrl = notifPrefs.slackWebhookUrl;
          }
          if (notifPrefs.webhookUrl) {
            state.webhookUrl = notifPrefs.webhookUrl;
            state.savedWebhookUrl = notifPrefs.webhookUrl;
          }
          if (notifPrefs.webhookSecret) {
            state.webhookSecret = notifPrefs.webhookSecret;
          }
        }
        if (keys) {
          state.apiKeys = keys;
        }
        state.limits = limits ?? null;
        if (limits?.transcriptionEnabled) {
          state.transcriptionEnabled = true;
        }
        state.noiseReductionEnabled = limits?.noiseReductionEnabled ?? false;
        if (limits?.brandingEnabled) {
          state.brandingEnabled = true;
          const brandingData = await apiFetch<BrandingSettings>("/api/settings/branding");
          if (brandingData) {
            state.branding = brandingData;
          }
        }
      } catch {
        // stay on page, fields will be empty
      }

      if (!state.profile) return;

      try {
        const billingData = await apiFetch<BillingData>("/api/settings/billing");
        if (billingData) {
          state.billing = billingData;
          state.billingEnabled = true;
        }
      } catch {
        state.billingEnabled = false;
      }

      try {
        const [intgData, identityData] = await Promise.all([
          apiFetch<IntegrationConfig[]>("/api/settings/integrations").catch(() => null),
          apiFetch<IdentitiesResponse>("/api/user/identities").catch(() => null),
        ]);
        if (intgData) {
          state.integrations = intgData;
          for (const ig of intgData) {
            if (ig.provider === "github") {
              state.ghToken = ig.config.token || "";
              state.ghOwner = ig.config.owner || "";
              state.ghRepo = ig.config.repo || "";
            } else if (ig.provider === "jira") {
              state.jiraBaseUrl = ig.config.base_url || "";
              state.jiraEmail = ig.config.email || "";
              state.jiraApiToken = ig.config.api_token || "";
              state.jiraProjectKey = ig.config.project_key || "";
            }
          }
        }
        if (identityData) {
          state.identities = identityData.identities;
          state.identityHasPassword = identityData.hasPassword;
        }
      } catch { /* integrations/identities not available */ }

      setLoaded(state as LoadedState);
    }
    fetchProfile();
  }, []);

  if (!loaded) {
    return (
      <div className="page-container page-container--centered">
        <p className="status-message status-message--success">Loading...</p>
      </div>
    );
  }

  return (
    <div className="page-container">
      <h1 className="page-title">Settings</h1>

      {loaded.billingEnabled && loaded.billing && (
        <BillingSection billing={loaded.billing} />
      )}

      <ProfileSection
        profile={loaded.profile}
        transcriptionEnabled={loaded.transcriptionEnabled}
        noiseReductionEnabled={loaded.noiseReductionEnabled}
        initialTranscriptionLanguage={loaded.transcriptionLanguage}
        initialNoiseReduction={loaded.noiseReduction}
        initialRetentionDays={loaded.retentionDays}
      />

      <IntegrationSection
        initialIntegrations={loaded.integrations}
        initialGhToken={loaded.ghToken}
        initialGhOwner={loaded.ghOwner}
        initialGhRepo={loaded.ghRepo}
        initialJiraBaseUrl={loaded.jiraBaseUrl}
        initialJiraEmail={loaded.jiraEmail}
        initialJiraApiToken={loaded.jiraApiToken}
        initialJiraProjectKey={loaded.jiraProjectKey}
      />

      <NotificationSection
        initialNotificationMode={loaded.notificationMode}
        initialSlackWebhookUrl={loaded.slackWebhookUrl}
        initialSavedSlackUrl={loaded.savedSlackUrl}
      />

      <WebhookSection
        initialNotificationMode={loaded.notificationMode}
        initialWebhookUrl={loaded.webhookUrl}
        initialSavedWebhookUrl={loaded.savedWebhookUrl}
        initialWebhookSecret={loaded.webhookSecret}
        initialSavedSlackUrl={loaded.savedSlackUrl}
      />

      <SecuritySection
        limits={loaded.limits}
        initialApiKeys={loaded.apiKeys}
        initialIdentities={loaded.identities}
        initialIdentityHasPassword={loaded.identityHasPassword}
      />

      {loaded.brandingEnabled && (
        <BrandingSection
          initialBranding={loaded.branding}
          limits={loaded.limits}
        />
      )}
    </div>
  );
}
