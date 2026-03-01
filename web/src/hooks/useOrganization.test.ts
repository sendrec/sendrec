import { renderHook, act, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const mockApiFetch = vi.fn();
vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

function makeOrg(overrides: Record<string, unknown> = {}) {
  return {
    id: "org-1",
    name: "Acme Corp",
    slug: "acme-corp",
    subscriptionPlan: "free",
    role: "owner",
    memberCount: 3,
    ...overrides,
  };
}

describe("useOrganization", () => {
  beforeEach(() => {
    localStorage.clear();
    mockApiFetch.mockReset();
    vi.resetModules();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  async function loadHook() {
    const mod = await import("./useOrganization");
    return mod.useOrganization;
  }

  it("fetches organizations on mount", async () => {
    const orgList = [makeOrg(), makeOrg({ id: "org-2", name: "Beta Inc" })];
    mockApiFetch.mockResolvedValueOnce(orgList);

    const useOrganization = await loadHook();
    const { result } = renderHook(() => useOrganization());

    expect(result.current.loading).toBe(true);

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.orgs).toHaveLength(2);
    expect(result.current.orgs[0].name).toBe("Acme Corp");
    expect(result.current.orgs[1].name).toBe("Beta Inc");
    expect(mockApiFetch).toHaveBeenCalledWith("/api/organizations");
  });

  it("returns empty array when fetch fails", async () => {
    mockApiFetch.mockRejectedValueOnce(new Error("network error"));

    const useOrganization = await loadHook();
    const { result } = renderHook(() => useOrganization());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.orgs).toEqual([]);
    expect(result.current.selectedOrg).toBeNull();
  });

  it("switchOrg updates selectedOrgId", async () => {
    const orgList = [makeOrg()];
    mockApiFetch.mockResolvedValueOnce(orgList);

    const useOrganization = await loadHook();
    const { result } = renderHook(() => useOrganization());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.selectedOrgId).toBeNull();

    act(() => {
      result.current.switchOrg("org-1");
    });

    expect(result.current.selectedOrgId).toBe("org-1");
    expect(result.current.selectedOrg).toEqual(orgList[0]);
    expect(localStorage.getItem("sendrec-org-id")).toBe("org-1");
  });

  it("clears stored org when it no longer exists in fetched list", async () => {
    localStorage.setItem("sendrec-org-id", "deleted-org");
    mockApiFetch.mockResolvedValueOnce([makeOrg()]);

    const useOrganization = await loadHook();
    const { result } = renderHook(() => useOrganization());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.selectedOrgId).toBeNull();
    expect(localStorage.getItem("sendrec-org-id")).toBeNull();
  });

  it("switchOrg to null clears localStorage", async () => {
    mockApiFetch.mockResolvedValueOnce([makeOrg()]);

    const useOrganization = await loadHook();
    const { result } = renderHook(() => useOrganization());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    act(() => {
      result.current.switchOrg("org-1");
    });

    expect(localStorage.getItem("sendrec-org-id")).toBe("org-1");

    act(() => {
      result.current.switchOrg(null);
    });

    expect(result.current.selectedOrgId).toBeNull();
    expect(result.current.selectedOrg).toBeNull();
    expect(localStorage.getItem("sendrec-org-id")).toBeNull();
  });

  it("restores selected org from localStorage on mount", async () => {
    localStorage.setItem("sendrec-org-id", "org-1");
    mockApiFetch.mockResolvedValueOnce([makeOrg()]);

    const useOrganization = await loadHook();
    const { result } = renderHook(() => useOrganization());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.selectedOrgId).toBe("org-1");
    expect(result.current.selectedOrg?.name).toBe("Acme Corp");
  });
});
