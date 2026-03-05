import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { TransferDialog } from "./TransferDialog";

const mockApiFetch = vi.fn();
vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

const mockUseOrganization = vi.fn();
vi.mock("../hooks/useOrganization", () => ({
  useOrganization: () => mockUseOrganization(),
}));

function renderDialog(props: Partial<React.ComponentProps<typeof TransferDialog>> = {}) {
  const defaultProps = {
    videoId: "vid-1",
    videoTitle: "Test Video",
    onTransferred: vi.fn(),
    onCancel: vi.fn(),
    ...props,
  };
  return render(
    <MemoryRouter>
      <TransferDialog {...defaultProps} />
    </MemoryRouter>
  );
}

describe("TransferDialog", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    mockUseOrganization.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("lists workspaces excluding current scope", () => {
    mockUseOrganization.mockReturnValue({
      orgs: [
        { id: "org-1", name: "Workspace A", slug: "a", subscriptionPlan: "free", role: "member", memberCount: 2 },
        { id: "org-2", name: "Workspace B", slug: "b", subscriptionPlan: "free", role: "admin", memberCount: 3 },
      ],
      selectedOrgId: "org-1",
    });

    renderDialog();

    expect(screen.getByText("Personal")).toBeInTheDocument();
    expect(screen.queryByText("Workspace A")).not.toBeInTheDocument();
    expect(screen.getByText("Workspace B")).toBeInTheDocument();
  });

  it("excludes viewer-only workspaces", () => {
    mockUseOrganization.mockReturnValue({
      orgs: [
        { id: "org-1", name: "My Workspace", slug: "mine", subscriptionPlan: "free", role: "member", memberCount: 2 },
        { id: "org-2", name: "View Only", slug: "view", subscriptionPlan: "free", role: "viewer", memberCount: 3 },
      ],
      selectedOrgId: null,
    });

    renderDialog();

    expect(screen.getByText("My Workspace")).toBeInTheDocument();
    expect(screen.queryByText("View Only")).not.toBeInTheDocument();
  });

  it("shows Personal option when in a workspace", () => {
    mockUseOrganization.mockReturnValue({
      orgs: [
        { id: "org-1", name: "Current Workspace", slug: "cur", subscriptionPlan: "free", role: "member", memberCount: 2 },
      ],
      selectedOrgId: "org-1",
    });

    renderDialog();

    expect(screen.getByText("Personal")).toBeInTheDocument();
  });

  it("calls transfer API and onTransferred on success", async () => {
    const user = userEvent.setup();
    const onTransferred = vi.fn();
    mockUseOrganization.mockReturnValue({
      orgs: [
        { id: "org-1", name: "Target Workspace", slug: "target", subscriptionPlan: "free", role: "member", memberCount: 2 },
      ],
      selectedOrgId: null,
    });
    mockApiFetch.mockResolvedValueOnce({ id: "vid-1", title: "Test Video", organizationId: "org-1" });

    renderDialog({ onTransferred });

    await user.click(screen.getByText("Target Workspace"));
    await user.click(screen.getByRole("button", { name: "Move" }));

    expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/vid-1/transfer", {
      method: "POST",
      body: JSON.stringify({ organizationId: "org-1" }),
    });
    expect(onTransferred).toHaveBeenCalled();
  });

  it("shows error message on transfer failure", async () => {
    const user = userEvent.setup();
    mockUseOrganization.mockReturnValue({
      orgs: [
        { id: "org-1", name: "Target Workspace", slug: "target", subscriptionPlan: "free", role: "member", memberCount: 2 },
      ],
      selectedOrgId: null,
    });
    mockApiFetch.mockRejectedValueOnce(new Error("Server error"));

    renderDialog();

    await user.click(screen.getByText("Target Workspace"));
    await user.click(screen.getByRole("button", { name: "Move" }));

    expect(screen.getByText("Failed to transfer video")).toBeInTheDocument();
  });

  it("calls onCancel when Cancel button clicked", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    mockUseOrganization.mockReturnValue({
      orgs: [],
      selectedOrgId: null,
    });

    renderDialog({ onCancel });

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onCancel).toHaveBeenCalled();
  });
});
