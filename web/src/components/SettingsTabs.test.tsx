import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { SettingsTabs } from "./SettingsTabs";
import type { Organization } from "../hooks/useOrganization";

const rocket: Organization = { id: "org-1", name: "Rocket Team", slug: "rocket", subscriptionPlan: "free", role: "owner", memberCount: 3, icon: "🚀" };

function renderAt(path: string, workspace: Organization | null = null) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <SettingsTabs workspace={workspace} />
    </MemoryRouter>,
  );
}

// One Settings page with a tab per scope: the account, always, and the selected
// workspace for those who can manage it. #262.
describe("SettingsTabs", () => {
  it("shows only the account tab outside a workspace", () => {
    renderAt("/settings");
    expect(screen.getByRole("link", { name: "Account" })).toHaveAttribute("href", "/settings");
    expect(screen.getAllByRole("link")).toHaveLength(1);
  });

  it("adds the selected workspace, with its icon, for an owner or admin", () => {
    renderAt("/settings", rocket);
    const tab = screen.getByRole("link", { name: /Rocket Team/ });
    expect(tab).toHaveAttribute("href", "/organizations/org-1/settings");
    expect(tab).toHaveTextContent("🚀");
  });

  // Account settings — security, sign-in methods, API keys — stay reachable
  // from inside a workspace.
  it("keeps the account tab inside a workspace", () => {
    renderAt("/organizations/org-1/settings", rocket);
    expect(screen.getByRole("link", { name: "Account" })).toBeInTheDocument();
  });

  it.each(["member", "viewer"])("leaves the workspace tab out for a %s", (role) => {
    renderAt("/settings", { ...rocket, role });
    expect(screen.queryByRole("link", { name: /Rocket Team/ })).not.toBeInTheDocument();
  });

  it("marks the tab for the current page", () => {
    renderAt("/organizations/org-1/settings", rocket);
    expect(screen.getByRole("link", { name: /Rocket Team/ })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Account" })).not.toHaveAttribute("aria-current");
  });

  // React Router serves /settings/ as /settings; the tab should agree.
  it.each([
    ["/settings/", "Account"],
    ["/organizations/org-1/settings/", /Rocket Team/],
  ])("marks the current tab behind a trailing slash at %s", (path, name) => {
    renderAt(path, rocket);
    expect(screen.getByRole("link", { name })).toHaveAttribute("aria-current", "page");
  });
});
