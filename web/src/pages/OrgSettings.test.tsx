import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, MemoryRouter, Route, RouterProvider, Routes } from "react-router-dom";
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

  it("shows upgrade CTA when workspace plan is free", async () => {
    mockApiFetch
      .mockResolvedValueOnce(mockOrg)
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ plan: "free" });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Upgrade to Pro")).toBeInTheDocument();
    });
  });

  it("shows manage subscription when workspace has Pro plan", async () => {
    mockApiFetch
      .mockResolvedValueOnce(mockOrg)
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ plan: "pro", portalUrl: "https://portal.example.com" });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Manage subscription")).toBeInTheDocument();
    });

    expect(screen.getByText("Cancel subscription")).toBeInTheDocument();
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


  it("hides upgrade cards for paid plan", async () => {
    mockApiFetch
      .mockResolvedValueOnce(mockOrg)
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ plan: "pro" });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Corp")).toBeInTheDocument();
    });

    expect(screen.queryByText("Upgrade to Pro")).not.toBeInTheDocument();
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
      .mockResolvedValueOnce({ plan: "business" })
      .mockResolvedValueOnce({ issuerUrl: "", clientId: "", configured: false, enforceSso: false });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Single Sign-On")).toBeInTheDocument();
    });
  });

  it("SSO form shows protocol toggle with OIDC and SAML options", async () => {
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
      .mockResolvedValueOnce({ plan: "business" })
      .mockResolvedValueOnce({ provider: "oidc", issuerUrl: "", clientId: "", configured: false, enforceSso: false });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Single Sign-On")).toBeInTheDocument();
    });

    const oidcRadio = screen.getByRole("radio", { name: "OIDC" });
    const samlRadio = screen.getByRole("radio", { name: "SAML" });
    expect(oidcRadio).toBeInTheDocument();
    expect(samlRadio).toBeInTheDocument();
  });

  it("SSO form shows SAML fields when SAML is selected", async () => {
    const user = userEvent.setup();
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
      .mockResolvedValueOnce({ plan: "business" })
      .mockResolvedValueOnce({ provider: "oidc", issuerUrl: "", clientId: "", configured: false, enforceSso: false });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Single Sign-On")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("radio", { name: "SAML" }));

    expect(screen.getByLabelText("Metadata URL")).toBeInTheDocument();
    expect(screen.getByLabelText("Or paste metadata XML")).toBeInTheDocument();
  });

  it("SSO form shows OIDC fields when OIDC is selected", async () => {
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
      .mockResolvedValueOnce({ plan: "business" })
      .mockResolvedValueOnce({ provider: "oidc", issuerUrl: "", clientId: "", configured: false, enforceSso: false });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Single Sign-On")).toBeInTheDocument();
    });

    expect(screen.getByLabelText("Issuer URL")).toBeInTheDocument();
    expect(screen.getByLabelText("Client ID")).toBeInTheDocument();
    expect(screen.getByLabelText("Client Secret")).toBeInTheDocument();
  });

  it("SSO form loads existing SAML config", async () => {
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
      .mockRejectedValueOnce(new Error("Not Found"))
      .mockResolvedValueOnce({
        provider: "saml",
        configured: true,
        enforceSso: false,
        samlMetadataUrl: "https://idp.example.com/metadata",
        samlEntityId: "https://idp.example.com/entity",
        samlSsoUrl: "https://idp.example.com/sso",
        spMetadataUrl: "https://app.sendrec.eu/saml/org-1/metadata",
      });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Single Sign-On")).toBeInTheDocument();
    });

    const samlRadio = screen.getByRole("radio", { name: "SAML" });
    expect(samlRadio).toBeChecked();

    expect(screen.getByDisplayValue("https://idp.example.com/entity")).toBeInTheDocument();
    expect(screen.getByDisplayValue("https://idp.example.com/sso")).toBeInTheDocument();
    expect(screen.getByDisplayValue("https://app.sendrec.eu/saml/org-1/metadata")).toBeInTheDocument();
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

  it("shows SCIM section for business plan", async () => {
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
      .mockResolvedValueOnce({ plan: "business" })
      .mockResolvedValueOnce({ issuerUrl: "", clientId: "", configured: false, enforceSso: false })
      .mockResolvedValueOnce({ configured: false });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("SCIM Provisioning")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "Generate SCIM Token" })).toBeInTheDocument();
  });

  it("shows SCIM token status when configured", async () => {
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
      .mockResolvedValueOnce({ plan: "business" })
      .mockResolvedValueOnce({ issuerUrl: "", clientId: "", configured: false, enforceSso: false })
      .mockResolvedValueOnce({ configured: true, createdAt: "2026-03-06T00:00:00Z" });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText(/Active/)).toBeInTheDocument();
    });
    expect(screen.getByText("SCIM Provisioning")).toBeInTheDocument();
    expect(screen.getByText("SCIM Base URL")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Regenerate Token" })).toBeInTheDocument();
    expect(screen.getByText("Setup Guide").closest("details")).toHaveClass("settings-details");
  });

  it("clears generated SCIM token when switching workspaces", async () => {
    const user = userEvent.setup();
    const org1Context = {
      orgs: [
        { id: "org-1", name: "Acme Corp", slug: "acme-corp", subscriptionPlan: "business", role: "owner", memberCount: 3 },
        { id: "org-2", name: "Beta Corp", slug: "beta-corp", subscriptionPlan: "business", role: "owner", memberCount: 2 },
      ],
      selectedOrg: { id: "org-1", name: "Acme Corp", slug: "acme-corp", subscriptionPlan: "business", role: "owner", memberCount: 3 },
      selectedOrgId: "org-1",
      switchOrg: vi.fn(),
      createOrg: vi.fn(),
      refreshOrgs: vi.fn(),
      loading: false,
    };
    const org2Context = {
      ...org1Context,
      selectedOrg: { id: "org-2", name: "Beta Corp", slug: "beta-corp", subscriptionPlan: "business", role: "owner", memberCount: 2 },
      selectedOrgId: "org-2",
    };
    let currentOrgContext = org1Context;
    mockUseOrganization.mockImplementation(() => currentOrgContext);

    mockApiFetch
      .mockResolvedValueOnce({ ...mockOrg, subscriptionPlan: "business" })
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ plan: "business" })
      .mockResolvedValueOnce({ issuerUrl: "", clientId: "", configured: false, enforceSso: false })
      .mockResolvedValueOnce({ configured: false })
      .mockResolvedValueOnce({ token: "scim_org_1_token" })
      .mockResolvedValueOnce({ ...mockOrg, id: "org-2", name: "Beta Corp", slug: "beta-corp", subscriptionPlan: "business" })
      .mockResolvedValueOnce([ownerMember])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ plan: "business" })
      .mockResolvedValueOnce({ issuerUrl: "", clientId: "", configured: false, enforceSso: false })
      .mockResolvedValueOnce({ configured: true, createdAt: "2026-03-07T00:00:00Z" });

    const router = createMemoryRouter(
      [
        { path: "/organizations/:id/settings", element: <OrgSettings /> },
      ],
      { initialEntries: ["/organizations/org-1/settings"] },
    );

    render(<RouterProvider router={router} />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Generate SCIM Token" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Generate SCIM Token" }));

    await waitFor(() => {
      expect(screen.getByDisplayValue("scim_org_1_token")).toBeInTheDocument();
    });

    currentOrgContext = org2Context;
    await act(async () => {
      await router.navigate("/organizations/org-2/settings");
    });

    await waitFor(() => {
      expect(screen.getByText("SCIM Base URL")).toBeInTheDocument();
    });

    expect(screen.queryByDisplayValue("scim_org_1_token")).not.toBeInTheDocument();
  });

  it("shows SCIM status load errors", async () => {
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
      .mockResolvedValueOnce({ plan: "business" })
      .mockResolvedValueOnce({ issuerUrl: "", clientId: "", configured: false, enforceSso: false })
      .mockRejectedValueOnce(new Error("SCIM status unavailable"));

    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Failed to load SCIM status")).toBeInTheDocument();
    });
  });

  it("hides SCIM section for non-business plan", async () => {
    mockOwnerResponses();
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Corp")).toBeInTheDocument();
    });

    expect(screen.queryByText("SCIM Provisioning")).not.toBeInTheDocument();
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
