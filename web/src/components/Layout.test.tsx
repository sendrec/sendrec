import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Layout } from "./Layout";
import { expectNoA11yViolations } from "../test-utils/a11y";

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

const mockSetAccessToken = vi.fn();
const mockApiFetch = vi.fn();
vi.mock("../api/client", () => ({
  setAccessToken: (...args: unknown[]) => mockSetAccessToken(...args),
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

const mockSwitchOrg = vi.fn();
const mockCreateOrg = vi.fn();
const mockRefreshOrgs = vi.fn();
const mockUseOrganization = vi.fn().mockReturnValue({
  orgs: [],
  selectedOrg: null,
  selectedOrgId: null,
  switchOrg: mockSwitchOrg,
  createOrg: mockCreateOrg,
  refreshOrgs: mockRefreshOrgs,
  loading: false,
});
vi.mock("../hooks/useOrganization", () => ({
  useOrganization: (...args: unknown[]) => mockUseOrganization(...args),
}));

function renderLayout(path = "/") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Layout>
        <div>Page content</div>
      </Layout>
    </MemoryRouter>
  );
}

describe("Layout", () => {
  beforeEach(() => {
    mockNavigate.mockReset();
    mockSetAccessToken.mockReset();
    mockApiFetch.mockReset();
    mockApiFetch.mockRejectedValue(new Error("not available"));
    mockSwitchOrg.mockReset();
    mockCreateOrg.mockReset();
    mockRefreshOrgs.mockReset();
    mockUseOrganization.mockReturnValue({
      orgs: [],
      selectedOrg: null,
      selectedOrgId: null,
      switchOrg: mockSwitchOrg,
      createOrg: mockCreateOrg,
      refreshOrgs: mockRefreshOrgs,
      loading: false,
    });
    globalThis.fetch = vi.fn().mockResolvedValue({});
  });

  it("renders navigation links", () => {
    renderLayout();
    expect(screen.getByRole("link", { name: /SendRec/ })).toHaveAttribute("href", "/");
    expect(screen.getByRole("link", { name: "Record" })).toHaveAttribute("href", "/");
    expect(screen.getByRole("link", { name: "Library" })).toHaveAttribute("href", "/library");
    expect(screen.getByRole("link", { name: "Playlists" })).toHaveAttribute("href", "/playlists");
    expect(screen.getByRole("link", { name: "Analytics" })).toHaveAttribute("href", "/analytics");
    expect(screen.getByRole("link", { name: "Settings" })).toHaveAttribute("href", "/settings");
    expect(screen.queryByRole("link", { name: "Upload" })).not.toBeInTheDocument();
  });

  it("renders logo image in nav", () => {
    renderLayout();
    const logo = document.querySelector('img[src="/images/logo.png"]') as HTMLImageElement;
    expect(logo).toBeInTheDocument();
    expect(logo).toHaveAttribute("alt", "");
  });

  it("renders children in main element", () => {
    renderLayout();
    expect(screen.getByText("Page content")).toBeInTheDocument();
    expect(screen.getByRole("main")).toContainElement(screen.getByText("Page content"));
  });

  it("highlights active link for Record on /", () => {
    renderLayout("/");
    const recordLink = screen.getByRole("link", { name: "Record" });
    expect(recordLink).toHaveClass("nav-link--active");
  });

  it("highlights active link for Library on /library", () => {
    renderLayout("/library");
    const libraryLink = screen.getByRole("link", { name: "Library" });
    expect(libraryLink).toHaveClass("nav-link--active");
    const recordLink = screen.getByRole("link", { name: "Record" });
    expect(recordLink).not.toHaveClass("nav-link--active");
  });

  it("highlights active link for Analytics on /analytics", () => {
    renderLayout("/analytics");
    const analyticsLink = screen.getByRole("link", { name: "Analytics" });
    expect(analyticsLink).toHaveClass("nav-link--active");
    const recordLink = screen.getByRole("link", { name: "Record" });
    expect(recordLink).not.toHaveClass("nav-link--active");
  });

  it("highlights Analytics link for per-video analytics", () => {
    renderLayout("/videos/123/analytics");
    const analyticsLink = screen.getByRole("link", { name: "Analytics" });
    expect(analyticsLink).toHaveClass("nav-link--active");
  });

  it("signs out on button click", async () => {
    const user = userEvent.setup();
    renderLayout();

    await user.click(screen.getByRole("button", { name: "Sign out" }));

    expect(globalThis.fetch).toHaveBeenCalledWith("/api/auth/logout", {
      method: "POST",
      credentials: "include",
    });
    expect(mockSetAccessToken).toHaveBeenCalledWith(null);
    expect(mockNavigate).toHaveBeenCalledWith("/login");
  });

  it("renders hamburger menu button", () => {
    renderLayout();
    expect(screen.getByRole("button", { name: "Toggle menu" })).toBeInTheDocument();
  });

  it("toggles mobile menu on hamburger click", async () => {
    const user = userEvent.setup();
    renderLayout();

    const hamburger = screen.getByRole("button", { name: "Toggle menu" });
    const navLinks = screen.getByRole("link", { name: "Record" }).closest(".nav-links");
    expect(navLinks).not.toHaveClass("nav-links--open");

    await user.click(hamburger);
    expect(navLinks).toHaveClass("nav-links--open");

    await user.click(hamburger);
    expect(navLinks).not.toHaveClass("nav-links--open");
  });

  it("closes mobile menu when a nav link is clicked", async () => {
    const user = userEvent.setup();
    renderLayout();

    const hamburger = screen.getByRole("button", { name: "Toggle menu" });
    await user.click(hamburger);

    const navLinks = screen.getByRole("link", { name: "Library" }).closest(".nav-links");
    expect(navLinks).toHaveClass("nav-links--open");

    await user.click(screen.getByRole("link", { name: "Library" }));
    expect(navLinks).not.toHaveClass("nav-links--open");
  });

  it("renders nav with nav-bar class", () => {
    renderLayout();
    const nav = screen.getByRole("navigation");
    expect(nav).toHaveClass("nav-bar");
  });

  it("shows Free badge for free plan", async () => {
    mockApiFetch.mockResolvedValueOnce({ plan: "free" });
    renderLayout();

    await waitFor(() => {
      expect(screen.getByText("Free")).toBeInTheDocument();
    });
    expect(screen.getByText("Free")).toHaveClass("plan-badge");
  });

  it("shows Pro badge for pro plan", async () => {
    mockApiFetch.mockResolvedValueOnce({ plan: "pro" });
    renderLayout();

    await waitFor(() => {
      expect(screen.getByText("Pro")).toBeInTheDocument();
    });
    expect(screen.getByText("Pro")).toHaveClass("plan-badge", "plan-badge--pro");
  });

  it("shows Free badge when billing API fails", async () => {
    mockApiFetch.mockRejectedValueOnce(new Error("not available"));
    renderLayout();

    await waitFor(() => {
      expect(screen.getByText("Free")).toBeInTheDocument();
    });
    expect(screen.queryByText("Pro")).not.toBeInTheDocument();
  });

  it("always renders org switcher with New Workspace option", () => {
    renderLayout();
    const switcher = screen.getByLabelText("Switch workspace");
    expect(switcher).toBeInTheDocument();
    expect(switcher).toHaveValue("personal");
    expect(screen.getByText("+ New Workspace")).toBeInTheDocument();
  });

  it("renders org switcher with orgs listed", () => {
    mockUseOrganization.mockReturnValue({
      orgs: [
        { id: "org-1", name: "Acme Corp", slug: "acme", subscriptionPlan: "free", role: "owner", memberCount: 3 },
      ],
      selectedOrg: null,
      selectedOrgId: null,
      switchOrg: mockSwitchOrg,
      createOrg: mockCreateOrg,
      refreshOrgs: mockRefreshOrgs,
      loading: false,
    });
    renderLayout();
    const switcher = screen.getByLabelText("Switch workspace");
    expect(switcher).toBeInTheDocument();
    expect(switcher).toHaveValue("personal");
    expect(screen.getByText("Acme Corp")).toBeInTheDocument();
  });

  it("calls switchOrg when selecting an organization", async () => {
    const user = userEvent.setup();
    mockUseOrganization.mockReturnValue({
      orgs: [
        { id: "org-1", name: "Acme Corp", slug: "acme", subscriptionPlan: "free", role: "owner", memberCount: 3 },
      ],
      selectedOrg: null,
      selectedOrgId: null,
      switchOrg: mockSwitchOrg,
      createOrg: mockCreateOrg,
      refreshOrgs: mockRefreshOrgs,
      loading: false,
    });
    renderLayout();

    await user.selectOptions(screen.getByLabelText("Switch workspace"), "org-1");
    expect(mockSwitchOrg).toHaveBeenCalledWith("org-1");
  });

  it("calls switchOrg with null when selecting personal", async () => {
    const user = userEvent.setup();
    mockUseOrganization.mockReturnValue({
      orgs: [
        { id: "org-1", name: "Acme Corp", slug: "acme", subscriptionPlan: "free", role: "owner", memberCount: 3 },
      ],
      selectedOrg: { id: "org-1", name: "Acme Corp", slug: "acme", subscriptionPlan: "free", role: "owner", memberCount: 3 },
      selectedOrgId: "org-1",
      switchOrg: mockSwitchOrg,
      createOrg: mockCreateOrg,
      refreshOrgs: mockRefreshOrgs,
      loading: false,
    });
    renderLayout();

    await user.selectOptions(screen.getByLabelText("Switch workspace"), "personal");
    expect(mockSwitchOrg).toHaveBeenCalledWith(null);
  });

  it("shows Org Settings link when an org is selected", () => {
    mockUseOrganization.mockReturnValue({
      orgs: [
        { id: "org-1", name: "Acme Corp", slug: "acme", subscriptionPlan: "free", role: "owner", memberCount: 3 },
      ],
      selectedOrg: { id: "org-1", name: "Acme Corp", slug: "acme", subscriptionPlan: "free", role: "owner", memberCount: 3 },
      selectedOrgId: "org-1",
      switchOrg: mockSwitchOrg,
      createOrg: mockCreateOrg,
      refreshOrgs: mockRefreshOrgs,
      loading: false,
    });
    renderLayout();
    const orgSettingsLink = screen.getByRole("link", { name: "Org Settings" });
    expect(orgSettingsLink).toHaveAttribute("href", "/organizations/org-1/settings");
  });

  it("hides Org Settings link when no org is selected", () => {
    renderLayout();
    expect(screen.queryByRole("link", { name: "Org Settings" })).not.toBeInTheDocument();
  });

  it("has no accessibility violations", async () => {
    const { container } = renderLayout();
    await expectNoA11yViolations(container);
  });
});
