import { useState, useEffect, useCallback } from "react";
import { apiFetch } from "../api/client";
import { getCurrentOrgId, setCurrentOrgId, subscribeToOrgChanges, subscribeToOrgUpdates } from "../api/orgContext";

export interface Organization {
  id: string;
  name: string;
  slug: string;
  subscriptionPlan: string;
  role: string;
  memberCount: number;
  icon?: string | null;
}

export function useOrganization() {
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [selectedOrgId, setSelectedOrgId] = useState<string | null>(
    getCurrentOrgId()
  );
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    return subscribeToOrgChanges(() => {
      setSelectedOrgId(getCurrentOrgId());
    });
  }, []);

  useEffect(() => {
    apiFetch<Organization[]>("/api/organizations")
      .then((result) => {
        const list = result ?? [];
        setOrgs(list);
        const stored = getCurrentOrgId();
        if (stored && !list.some((o) => o.id === stored)) {
          setCurrentOrgId(null);
        }
      })
      .catch(() => setOrgs([]))
      .finally(() => setLoading(false));
  }, []);

  const switchOrg = useCallback((orgId: string | null) => {
    setCurrentOrgId(orgId);
  }, []);

  const selectedOrg = orgs.find((o) => o.id === selectedOrgId) ?? null;

  const createOrg = useCallback(async (name: string): Promise<Organization | null> => {
    const result = await apiFetch<Organization>("/api/organizations", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    });
    if (result) {
      setOrgs((prev) => [...prev, { ...result, role: "owner", memberCount: 1 }]);
      setCurrentOrgId(result.id);
    }
    return result ?? null;
  }, []);

  const refreshOrgs = useCallback(() => {
    apiFetch<Organization[]>("/api/organizations")
      .then((result) => {
        const list = result ?? [];
        setOrgs(list);
        const stored = getCurrentOrgId();
        if (stored && !list.some((o) => o.id === stored)) {
          setCurrentOrgId(null);
        }
      })
      // A refresh tops up a list the page already relies on. On a transient
      // error keep what is there: an empty list drops the selected workspace
      // and sends the user out of its settings.
      .catch(() => {});
  }, []);

  useEffect(() => subscribeToOrgUpdates(refreshOrgs), [refreshOrgs]);

  return { orgs, selectedOrg, selectedOrgId, switchOrg, createOrg, refreshOrgs, loading };
}
