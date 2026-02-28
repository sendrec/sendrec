import { useState, useEffect, useCallback } from "react";
import { apiFetch } from "../api/client";
import {
  getCurrentOrgId,
  setCurrentOrgId,
  subscribeToOrgChanges,
} from "../api/orgContext";

export interface Organization {
  id: string;
  name: string;
  slug: string;
  subscriptionPlan: string;
  role: string;
  memberCount: number;
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

  return { orgs, selectedOrg, selectedOrgId, switchOrg, loading };
}
