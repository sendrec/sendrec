import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SettingsTabs } from "./SettingsTabs";

const mockUseOrganization = vi.fn();
vi.mock("../hooks/useOrganization", () => ({
  useOrganization: (...args: unknown[]) => mockUseOrganization(...args),
}));

const rocket = { id: "org-1", name: "Rocket Team", slug: "rocket", subscriptionPlan: "free", role: "owner", memberCount: 3, icon: "🚀" };

function withWorkspace(org: typeof rocket | null) {
  mockUseOrganization.mockReturnValue({ orgs: org ? [org] : [], selectedOrg: org, selectedOrgId: org?.id ?? null, loading: false });
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <SettingsTabs />
    </MemoryRouter>,
  );
}

// One Settings page with a tab per scope: the account, always, and the selected
// workspace for those who can manage it. #262.
describe("SettingsTabs", () => {
  beforeEach(() => withWorkspace(null));

  it("shows only the account tab outside a workspace", () => {
    renderAt("/settings");
    expect(screen.getByRole("link", { name: "Account" })).toHaveAttribute("href", "/settings");
    expect(screen.getAllByRole("link")).toHaveLength(1);
  });

  it("adds the selected workspace, with its icon, for an owner or admin", () => {
    withWorkspace(rocket);
    renderAt("/settings");
    const tab = screen.getByRole("link", { name: /Rocket Team/ });
    expect(tab).toHaveAttribute("href", "/organizations/org-1/settings");
    expect(tab).toHaveTextContent("🚀");
  });

  // Account settings — security, sign-in methods, API keys — stay reachable
  // from inside a workspace.
  it("keeps the account tab inside a workspace", () => {
    withWorkspace(rocket);
    renderAt("/organizations/org-1/settings");
    expect(screen.getByRole("link", { name: "Account" })).toBeInTheDocument();
  });

  it.each(["member", "viewer"])("leaves the workspace tab out for a %s", (role) => {
    withWorkspace({ ...rocket, role });
    renderAt("/settings");
    expect(screen.queryByRole("link", { name: /Rocket Team/ })).not.toBeInTheDocument();
  });

  it("marks the tab for the current page", () => {
    withWorkspace(rocket);
    renderAt("/organizations/org-1/settings");
    expect(screen.getByRole("link", { name: /Rocket Team/ })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Account" })).not.toHaveAttribute("aria-current");
  });
});
