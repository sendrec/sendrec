import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AcceptInvite } from "./AcceptInvite";
import { expectNoA11yViolations } from "../test-utils/a11y";

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

const { mockApiFetch, mockGetAccessToken, mockTryRefreshToken } = vi.hoisted(() => ({
  mockApiFetch: vi.fn(),
  mockGetAccessToken: vi.fn(),
  mockTryRefreshToken: vi.fn(),
}));

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
  getAccessToken: () => mockGetAccessToken(),
  tryRefreshToken: () => mockTryRefreshToken(),
}));

function renderAcceptInvite(token?: string) {
  const path = token
    ? `/invites/accept?token=${encodeURIComponent(token)}`
    : "/invites/accept";
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AcceptInvite />
    </MemoryRouter>
  );
}

describe("AcceptInvite", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    mockGetAccessToken.mockReset();
    mockTryRefreshToken.mockReset();
    mockNavigate.mockReset();
  });

  it("shows login and register links when not authenticated", async () => {
    mockGetAccessToken.mockReturnValue(null);
    mockTryRefreshToken.mockResolvedValue(false);

    renderAcceptInvite("test-token");

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "You've been invited" })).toBeInTheDocument();
    });

    const signInLink = screen.getByRole("link", { name: "Sign in" });
    expect(signInLink).toBeInTheDocument();
    const signInHref = signInLink.getAttribute("href") ?? "";
    expect(signInHref).toContain("/login?redirect=");
    expect(decodeURIComponent(signInHref)).toContain("token=test-token");

    const createAccountLink = screen.getByRole("link", { name: "Create account" });
    expect(createAccountLink).toBeInTheDocument();
    const createAccountHref = createAccountLink.getAttribute("href") ?? "";
    expect(createAccountHref).toContain("/register?redirect=");
    expect(decodeURIComponent(createAccountHref)).toContain("token=test-token");
  });

  it("calls accept API when authenticated", async () => {
    mockGetAccessToken.mockReturnValue("valid-token");
    mockApiFetch.mockResolvedValueOnce(undefined);

    renderAcceptInvite("invite-token");

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/invites/accept", {
        method: "POST",
        body: JSON.stringify({ token: "invite-token" }),
      });
    });
  });

  it("shows error message for failed accept", async () => {
    mockGetAccessToken.mockReturnValue("valid-token");
    mockApiFetch.mockRejectedValueOnce(new Error("invite expired or already used"));

    renderAcceptInvite("expired-token");

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Invite failed" })).toBeInTheDocument();
    });

    expect(screen.getByText("invite expired or already used")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Go to dashboard" })).toHaveAttribute("href", "/");
  });

  it("shows success message after accepting", async () => {
    mockGetAccessToken.mockReturnValue("valid-token");
    mockApiFetch.mockResolvedValueOnce(undefined);

    renderAcceptInvite("valid-token");

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Invite accepted" })).toBeInTheDocument();
    });

    expect(screen.getByText(/You have joined the organization/)).toBeInTheDocument();
  });

  it("shows error when token is missing", () => {
    mockGetAccessToken.mockReturnValue(null);

    renderAcceptInvite();

    expect(screen.getByRole("heading", { name: "Invite failed" })).toBeInTheDocument();
    expect(screen.getByText("Missing invite token.")).toBeInTheDocument();
  });

  it("attempts token refresh when not immediately authenticated", async () => {
    mockGetAccessToken.mockReturnValue(null);
    mockTryRefreshToken.mockResolvedValue(true);
    mockApiFetch.mockResolvedValueOnce(undefined);

    renderAcceptInvite("some-token");

    await waitFor(() => {
      expect(mockTryRefreshToken).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/invites/accept", {
        method: "POST",
        body: JSON.stringify({ token: "some-token" }),
      });
    });
  });

  it("has no accessibility violations", async () => {
    mockGetAccessToken.mockReturnValue(null);
    mockTryRefreshToken.mockResolvedValue(false);

    const { container } = renderAcceptInvite("test-token");

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "You've been invited" })).toBeInTheDocument();
    });

    await expectNoA11yViolations(container);
  });
});
