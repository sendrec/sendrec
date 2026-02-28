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

  it("member cannot see invite form or delete section", async () => {
    mockMemberResponses();
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Corp")).toBeInTheDocument();
    });

    expect(screen.queryByText("Danger Zone")).not.toBeInTheDocument();
    expect(screen.queryByText("Delete workspace")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Email")).not.toBeInTheDocument();
    expect(screen.queryByText("Send invite")).not.toBeInTheDocument();
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
      .mockResolvedValueOnce({ plan: "free", effectivePlan: "pro" });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Pro")).toBeInTheDocument();
    });

    expect(screen.getByText(/Pro features through your personal subscription/)).toBeInTheDocument();
    expect(screen.queryByText("Upgrade to Pro")).not.toBeInTheDocument();
  });

  it("shows upgrade CTA when both workspace and effective plan are free", async () => {
    mockApiFetch
      .mockResolvedValueOnce(mockOrg)
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ plan: "free", effectivePlan: "free" });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Upgrade to Pro")).toBeInTheDocument();
    });

    expect(screen.queryByText(/personal subscription/)).not.toBeInTheDocument();
  });

  it("shows manage subscription when workspace has its own Pro plan", async () => {
    mockApiFetch
      .mockResolvedValueOnce(mockOrg)
      .mockResolvedValueOnce([ownerMember, regularMember])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ plan: "pro", effectivePlan: "pro", portalUrl: "https://portal.example.com" });
    renderOrgSettings();

    await waitFor(() => {
      expect(screen.getByText("Manage subscription")).toBeInTheDocument();
    });

    expect(screen.getByText("Cancel subscription")).toBeInTheDocument();
    expect(screen.queryByText(/personal subscription/)).not.toBeInTheDocument();
  });
});
