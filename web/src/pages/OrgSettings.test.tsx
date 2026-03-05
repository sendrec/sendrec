import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { OrgSettings } from "./OrgSettings";
import { expectNoA11yViolations } from "../test-utils/a11y";

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

const mockUseOrganization = vi.fn();
vi.mock("../hooks/useOrganization", () => ({
  useOrganization: () => mockUseOrganization(),
}));

const mockOrg = {
  id: "org-1",
  name: "Acme Corp",
  slug: "acme-corp",
  subscriptionPlan: "free",
  createdAt: "2026-01-15T00:00:00Z",
};

const ownerMember = {
  userId: "u1",
  email: "owner@acme.com",
  name: "Alice Owner",
  role: "owner",
  joinedAt: "2026-01-15T00:00:00Z",
};

const adminMember = {
  userId: "u2",
  email: "admin@acme.com",
  name: "Bob Admin",
  role: "admin",
  joinedAt: "2026-01-20T00:00:00Z",
};

const regularMember = {
  userId: "u3",
  email: "member@acme.com",
  name: "Charlie Member",
  role: "member",
  joinedAt: "2026-02-01T00:00:00Z",
};

const mockInvite = {
  id: "inv-1",
  email: "new@acme.com",
  role: "member",
  expiresAt: "2026-03-01T00:00:00Z",
  createdAt: "2026-02-20T00:00:00Z",
};

function renderOrgSettings() {
  return render(
    <MemoryRouter initialEntries={["/organizations/org-1/settings"]}>
      <Routes>
        <Route path="/organizations/:id/settings" element={<OrgSettings />} />
      </Routes>
    </MemoryRouter>,
  );
}

function mockOwnerResponses() {
  mockApiFetch
    .mockResolvedValueOnce(mockOrg)
    .mockResolvedValueOnce([ownerMember, adminMember, regularMember])
    .mockResolvedValueOnce([mockInvite])
    .mockRejectedValueOnce(new Error("Not Found"));
}

function mockMemberResponses() {
  mockApiFetch
    .mockResolvedValueOnce(mockOrg)
    .mockResolvedValueOnce([regularMember, ownerMember])
    .mockResolvedValueOnce([])
    .mockRejectedValueOnce(new Error("Not Found"));
}

describe("OrgSettings", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    mockNavigate.mockReset();
    mockUseOrganization.mockReturnValue({
      orgs: [{ id: "org-1", name: "Acme Corp", slug: "acme-corp", subscriptionPlan: "free", role: "owner", memberCount: 3 }],
      selectedOrg: { id: "org-1", name: "Acme Corp", slug: "acme-corp", subscriptionPlan: "free", role: "owner", memberCount: 3 },
      selectedOrgId: "org-1",
      switchOrg: vi.fn(),
      createOrg: vi.fn(),
      refreshOrgs: vi.fn(),
      loading: false,
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("has no accessibility violations", async () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}));
    const { container } = renderOrgSettings();
    await expectNoA11yViolations(container);
  });

  it("renders org name and members list", async () => {
    mockOwnerResponses();
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Corp")).toBeInTheDocument();
    });

    expect(screen.getByText("Alice Owner")).toBeInTheDocument();
    expect(screen.getByText("Bob Admin")).toBeInTheDocument();
    expect(screen.getByText("Charlie Member")).toBeInTheDocument();
    expect(screen.getByText(/3 members/)).toBeInTheDocument();
  });

  it("owner can see delete button and member role change", async () => {
    mockOwnerResponses();
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Corp")).toBeInTheDocument();
    });

    expect(screen.getByRole("button", { name: "Delete workspace" })).toBeInTheDocument();
    expect(screen.getByText("Danger Zone")).toBeInTheDocument();

    const roleSelects = screen.getAllByRole("combobox").filter(
      (el) => el.getAttribute("aria-label")?.startsWith("Role for")
    );
    expect(roleSelects.length).toBeGreaterThan(0);
  });

  it("member is redirected away from settings", async () => {
    mockUseOrganization.mockReturnValue({
      orgs: [{ id: "org-1", name: "Acme Corp", slug: "acme-corp", subscriptionPlan: "free", role: "member", memberCount: 3 }],
      selectedOrg: { id: "org-1", name: "Acme Corp", slug: "acme-corp", subscriptionPlan: "free", role: "member", memberCount: 3 },
      selectedOrgId: "org-1",
      switchOrg: vi.fn(),
      createOrg: vi.fn(),
      refreshOrgs: vi.fn(),
      loading: false,
    });
    renderOrgSettings();

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/", { replace: true });
    });
  });

  it("redirects when switched to personal context", async () => {
    mockUseOrganization.mockReturnValue({
      orgs: [{ id: "org-1", name: "Acme Corp", slug: "acme-corp", subscriptionPlan: "free", role: "owner", memberCount: 3 }],
      selectedOrg: null,
      selectedOrgId: null,
      switchOrg: vi.fn(),
      createOrg: vi.fn(),
      refreshOrgs: vi.fn(),
      loading: false,
    });
    renderOrgSettings();

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/", { replace: true });
    });
  });

  it("send invite form submits correctly", async () => {
    const user = userEvent.setup();
    mockOwnerResponses();
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Corp")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "inv-2",
      email: "colleague@acme.com",
      role: "member",
      expiresAt: "2026-03-15T00:00:00Z",
      createdAt: "2026-02-28T00:00:00Z",
    });

    await user.type(screen.getByLabelText("Email"), "colleague@acme.com");
    await user.click(screen.getByRole("button", { name: "Send invite" }));

    await waitFor(() => {
      expect(screen.getByText("Invite sent")).toBeInTheDocument();
    });

    expect(mockApiFetch).toHaveBeenCalledWith("/api/organizations/org-1/invites", {
      method: "POST",
      body: JSON.stringify({ email: "colleague@acme.com", role: "member" }),
    });
  });

  it("remove member shows confirmation", async () => {
    const user = userEvent.setup();
    mockOwnerResponses();
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Corp")).toBeInTheDocument();
    });

    const removeButtons = screen.getAllByRole("button", { name: "Remove" });
    expect(removeButtons.length).toBeGreaterThan(0);

    await user.click(removeButtons[0]);

    await waitFor(() => {
      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    });
  });

  it("shows loading state initially", () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}));
    renderOrgSettings();
    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("updates org name on save", async () => {
    const user = userEvent.setup();
    mockOwnerResponses();
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Corp")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce(undefined);

    await user.clear(screen.getByDisplayValue("Acme Corp"));
    await user.type(screen.getByLabelText("Workspace name"), "New Corp");
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      expect(screen.getByText("Workspace updated")).toBeInTheDocument();
    });

    expect(mockApiFetch).toHaveBeenCalledWith("/api/organizations/org-1", {
      method: "PATCH",
      body: JSON.stringify({ name: "New Corp", slug: "acme-corp" }),
    });
  });

  it("displays pending invites", async () => {
    mockOwnerResponses();
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("new@acme.com")).toBeInTheDocument();
    });

    expect(screen.getByText("Pending invites")).toBeInTheDocument();
    expect(screen.getByText(/Revoke/)).toBeInTheDocument();
  });

  it("shows Pro badge and owner plan message when effective plan comes from owner", async () => {
    mockApiFetch
      .mockResolvedValueOnce(mockOrg)
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ plan: "pro", planInherited: true });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Pro")).toBeInTheDocument();
    });

    expect(screen.getByText(/Inherited from your personal plan/)).toBeInTheDocument();
    expect(screen.queryByText("Upgrade to Pro")).not.toBeInTheDocument();
  });

  it("shows upgrade CTA when both workspace and effective plan are free", async () => {
    mockApiFetch
      .mockResolvedValueOnce(mockOrg)
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ plan: "free", planInherited: false });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Upgrade to Pro")).toBeInTheDocument();
    });

    expect(screen.queryByText(/Inherited from your personal plan/)).not.toBeInTheDocument();
  });

  it("shows manage subscription when workspace has its own Pro plan", async () => {
    mockApiFetch
      .mockResolvedValueOnce(mockOrg)
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ plan: "pro", planInherited: false, portalUrl: "https://portal.example.com" });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Manage subscription")).toBeInTheDocument();
    });

    expect(screen.getByText("Cancel subscription")).toBeInTheDocument();
    expect(screen.queryByText(/Inherited from your personal plan/)).not.toBeInTheDocument();
  });

  it("renders Data Retention select", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ ...mockOrg, retentionDays: 0 })
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found"));
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByLabelText("Auto-delete after")).toBeInTheDocument();
    });
    expect(screen.getByLabelText("Auto-delete after")).toHaveValue("0");
  });

  it("renders Data Retention with saved value", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ ...mockOrg, retentionDays: 60 })
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found"));
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByLabelText("Auto-delete after")).toHaveValue("60");
    });
  });

  it("changing retention days calls PATCH /api/organizations/:id", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ ...mockOrg, retentionDays: 0 })
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")) // billing
      .mockResolvedValueOnce({ message: "Settings updated" }); // PATCH response
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByLabelText("Auto-delete after")).toBeInTheDocument();
    });

    await user.selectOptions(screen.getByLabelText("Auto-delete after"), "90");

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/organizations/org-1", expect.objectContaining({
        method: "PATCH",
        body: JSON.stringify({ retentionDays: 90 }),
      }));
    });
  });

  it("renders SSO card for business plan admin", async () => {
    mockUseOrganization.mockReturnValue({
      orgs: [{ id: "org-1", name: "Acme Corp", slug: "acme-corp", subscriptionPlan: "business", role: "admin", memberCount: 3 }],
      selectedOrg: { id: "org-1", name: "Acme Corp", slug: "acme-corp", subscriptionPlan: "business", role: "admin", memberCount: 3 },
      selectedOrgId: "org-1",
      switchOrg: vi.fn(),
      createOrg: vi.fn(),
      refreshOrgs: vi.fn(),
      loading: false,
    });
    mockApiFetch
      .mockResolvedValueOnce({ ...mockOrg, subscriptionPlan: "business" })
      .mockResolvedValueOnce([adminMember, ownerMember])
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")) // billing
      .mockResolvedValueOnce({ issuerUrl: "https://accounts.google.com", clientId: "abc123", configured: true, enforceSso: false }); // SSO config
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Single Sign-On")).toBeInTheDocument();
    });
    expect(screen.getByLabelText("Issuer URL")).toHaveValue("https://accounts.google.com");
    expect(screen.getByLabelText("Client ID")).toHaveValue("abc123");
    expect(screen.getByRole("button", { name: "Save SSO settings" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove SSO" })).toBeInTheDocument();
  });

  it("hides SSO card for free plan", async () => {
    mockOwnerResponses();
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Corp")).toBeInTheDocument();
    });

    expect(screen.queryByText("Single Sign-On")).not.toBeInTheDocument();
  });

  it("hides SSO card for member role", async () => {
    mockUseOrganization.mockReturnValue({
      orgs: [{ id: "org-1", name: "Acme Corp", slug: "acme-corp", subscriptionPlan: "business", role: "member", memberCount: 3 }],
      selectedOrg: { id: "org-1", name: "Acme Corp", slug: "acme-corp", subscriptionPlan: "business", role: "member", memberCount: 3 },
      selectedOrgId: "org-1",
      switchOrg: vi.fn(),
      createOrg: vi.fn(),
      refreshOrgs: vi.fn(),
      loading: false,
    });
    renderOrgSettings();

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/", { replace: true });
    });

    expect(screen.queryByText("Single Sign-On")).not.toBeInTheDocument();
  });

  it("shows Inherited badge when planInherited is true", async () => {
    mockApiFetch
      .mockResolvedValueOnce(mockOrg)
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ plan: "pro", planInherited: true });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText(/Inherited from your personal plan/)).toBeInTheDocument();
    });

    expect(screen.getByText("Pro")).toBeInTheDocument();
  });

  it("hides upgrade cards for paid non-inherited plan", async () => {
    mockApiFetch
      .mockResolvedValueOnce(mockOrg)
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ plan: "pro", planInherited: false });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Corp")).toBeInTheDocument();
    });

    expect(screen.queryByText("Upgrade to Pro")).not.toBeInTheDocument();
    expect(screen.queryByText(/Inherited/)).not.toBeInTheDocument();
  });

  it("gates SSO on billing.plan === business", async () => {
    mockUseOrganization.mockReturnValue({
      orgs: [{ id: "org-1", name: "Acme Corp", slug: "acme-corp", subscriptionPlan: "business", role: "owner", memberCount: 3 }],
      selectedOrg: { id: "org-1", name: "Acme Corp", slug: "acme-corp", subscriptionPlan: "business", role: "owner", memberCount: 3 },
      selectedOrgId: "org-1",
      switchOrg: vi.fn(),
      createOrg: vi.fn(),
      refreshOrgs: vi.fn(),
      loading: false,
    });
    mockApiFetch
      .mockResolvedValueOnce({ ...mockOrg, subscriptionPlan: "business" })
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ plan: "business", planInherited: false })
      .mockResolvedValueOnce({ issuerUrl: "", clientId: "", configured: false, enforceSso: false });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Single Sign-On")).toBeInTheDocument();
    });
  });

  it("invite dropdown includes Viewer option", async () => {
    mockOwnerResponses();
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Corp")).toBeInTheDocument();
    });

    const roleSelect = screen.getByLabelText("Role");
    const viewerOption = Array.from(roleSelect.querySelectorAll("option")).find(
      (opt) => opt.textContent === "Viewer"
    );
    expect(viewerOption).toBeDefined();
    expect(viewerOption!.getAttribute("value")).toBe("viewer");
  });

  it("role dropdown includes viewer for non-owner members", async () => {
    mockOwnerResponses();
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Corp")).toBeInTheDocument();
    });

    const roleSelects = screen.getAllByRole("combobox").filter(
      (el) => el.getAttribute("aria-label")?.startsWith("Role for")
    );
    expect(roleSelects.length).toBeGreaterThan(0);

    const firstRoleSelect = roleSelects[0];
    const options = Array.from(firstRoleSelect.querySelectorAll("option")).map(
      (opt) => opt.getAttribute("value")
    );
    expect(options).toContain("viewer");
  });
});
