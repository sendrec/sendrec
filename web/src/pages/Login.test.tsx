import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Login } from "./Login";
import { expectNoA11yViolations } from "../test-utils/a11y";

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

const { MockApiError, mockApiFetch, mockSetAccessToken } = vi.hoisted(() => {
  class MockApiError extends Error {
    public readonly data: Record<string, unknown>;
    constructor(
      public readonly status: number,
      message: string,
      data: Record<string, unknown> = {}
    ) {
      super(message);
      this.name = "ApiError";
      this.data = data;
    }
  }
  return {
    MockApiError,
    mockApiFetch: vi.fn(),
    mockSetAccessToken: vi.fn(),
  };
});

vi.mock("../api/client", () => ({
  ApiError: MockApiError,
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
  setAccessToken: (...args: unknown[]) => mockSetAccessToken(...args),
}));

function mockHealthResponse(registrationEnabled: boolean) {
  vi.spyOn(global, "fetch").mockResolvedValueOnce(
    new Response(JSON.stringify({ registrationEnabled }), { status: 200 })
  );
}

function mockHealthAndProviders(registrationEnabled: boolean, providers: string[]) {
  vi.spyOn(global, "fetch")
    .mockResolvedValueOnce(
      new Response(JSON.stringify({ registrationEnabled }), { status: 200 })
    )
    .mockResolvedValueOnce(
      new Response(JSON.stringify({ providers }), { status: 200 })
    );
}

function renderLogin(initialEntries: string[] = ["/"]) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <Login />
    </MemoryRouter>
  );
}

describe("Login", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    mockSetAccessToken.mockReset();
    mockNavigate.mockReset();
    vi.restoreAllMocks();
  });

  it("renders sign in form", () => {
    renderLogin();
    expect(screen.getByRole("heading", { name: "Sign in" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sign in" })).toBeInTheDocument();
  });

  it("renders forgot password and register links", () => {
    renderLogin();
    expect(screen.getByRole("link", { name: "Forgot password?" })).toHaveAttribute("href", "/forgot-password");
    expect(screen.getByRole("link", { name: "Sign up" })).toHaveAttribute("href", "/register");
  });

  it("logs in and navigates to home on success", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce({ accessToken: "tok-123" });
    renderLogin();

    await user.type(screen.getByLabelText("Email"), "alice@example.com");
    await user.type(screen.getByLabelText("Password"), "password123");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(mockApiFetch).toHaveBeenCalledWith("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ email: "alice@example.com", password: "password123" }),
    });
    expect(mockSetAccessToken).toHaveBeenCalledWith("tok-123");
    expect(mockNavigate).toHaveBeenCalledWith("/");
  });

  it("shows error on failed login", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockRejectedValueOnce(new Error("Invalid credentials"));
    renderLogin();

    await user.type(screen.getByLabelText("Email"), "alice@example.com");
    await user.type(screen.getByLabelText("Password"), "wrongpass1");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(screen.getByText("Invalid credentials")).toBeInTheDocument();
  });

  it("has no accessibility violations", async () => {
    const { container } = renderLogin();
    await expectNoA11yViolations(container);
  });

  it("redirects to check-email on unverified email error", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockRejectedValueOnce(new MockApiError(403, "email_not_verified"));
    renderLogin();

    await user.type(screen.getByLabelText("Email"), "alice@example.com");
    await user.type(screen.getByLabelText("Password"), "password123");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(mockNavigate).toHaveBeenCalledWith("/check-email", { state: { email: "alice@example.com" } });
  });

  it("hides sign up link when registration is disabled", async () => {
    mockHealthResponse(false);
    renderLogin();

    await vi.waitFor(() => {
      expect(screen.queryByRole("link", { name: "Sign up" })).not.toBeInTheDocument();
    });
  });

  it("shows sign up link when registration is enabled", async () => {
    mockHealthResponse(true);
    renderLogin();

    await vi.waitFor(() => {
      expect(screen.getByRole("link", { name: "Sign up" })).toBeInTheDocument();
    });
  });

  it("renders social login buttons when providers API returns providers", async () => {
    mockHealthAndProviders(true, ["google", "github"]);
    renderLogin();

    await vi.waitFor(() => {
      expect(screen.getByText("Continue with Google")).toBeInTheDocument();
    });
    expect(screen.getByText("Continue with GitHub")).toBeInTheDocument();
    expect(screen.getByText("or")).toBeInTheDocument();

    const googleLink = screen.getByText("Continue with Google").closest("a");
    expect(googleLink).toHaveAttribute("href", "/api/auth/sso/google");

    const githubLink = screen.getByText("Continue with GitHub").closest("a");
    expect(githubLink).toHaveAttribute("href", "/api/auth/sso/github");
  });

  it("hides social login buttons when no providers", async () => {
    mockHealthAndProviders(true, []);
    renderLogin();

    await vi.waitFor(() => {
      expect(screen.getByRole("link", { name: "Sign up" })).toBeInTheDocument();
    });

    expect(screen.queryByText("or")).not.toBeInTheDocument();
    expect(screen.queryByText("Continue with Google")).not.toBeInTheDocument();
  });

  it("stores access token and navigates on sso_token param", () => {
    Object.defineProperty(window, "location", {
      value: {
        ...window.location,
        search: "?sso_token=sso-tok-abc",
        pathname: "/login",
      },
      writable: true,
    });
    const replaceStateSpy = vi.spyOn(window.history, "replaceState").mockImplementation(() => {});

    renderLogin();

    expect(mockSetAccessToken).toHaveBeenCalledWith("sso-tok-abc");
    expect(replaceStateSpy).toHaveBeenCalledWith({}, "", "/login");
    expect(mockNavigate).toHaveBeenCalledWith("/");
  });

  it("displays sso_error param as error message", async () => {
    Object.defineProperty(window, "location", {
      value: {
        ...window.location,
        search: "?sso_error=Account+not+found",
        pathname: "/login",
      },
      writable: true,
    });
    vi.spyOn(window.history, "replaceState").mockImplementation(() => {});

    renderLogin();

    await vi.waitFor(() => {
      expect(screen.getByText("Account not found")).toBeInTheDocument();
    });
  });

  it("shows workspace SSO button on sso_required error", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockRejectedValueOnce(
      new MockApiError(403, "sso_required", { orgId: "org-123", orgName: "Acme Corp" })
    );
    renderLogin();

    await user.type(screen.getByLabelText("Email"), "alice@example.com");
    await user.type(screen.getByLabelText("Password"), "password123");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(screen.getByText('"Acme Corp" requires SSO sign-in')).toBeInTheDocument();

    const ssoLink = screen.getByText("Sign in with SSO for Acme Corp");
    expect(ssoLink.closest("a")).toHaveAttribute(
      "href",
      "/api/auth/sso/org?email=alice%40example.com&org=org-123"
    );

    expect(screen.getByText("or")).toBeInTheDocument();
  });

  it("hides workspace SSO button after successful regular login", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockRejectedValueOnce(
        new MockApiError(403, "sso_required", { orgId: "org-123", orgName: "Acme Corp" })
      )
      .mockResolvedValueOnce({ accessToken: "tok-456" });
    renderLogin();

    await user.type(screen.getByLabelText("Email"), "alice@example.com");
    await user.type(screen.getByLabelText("Password"), "password123");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(screen.getByText("Sign in with SSO for Acme Corp")).toBeInTheDocument();

    await user.clear(screen.getByLabelText("Password"));
    await user.type(screen.getByLabelText("Password"), "correctpass1");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(mockSetAccessToken).toHaveBeenCalledWith("tok-456");
    expect(mockNavigate).toHaveBeenCalledWith("/");
    expect(screen.queryByText("Sign in with SSO for Acme Corp")).not.toBeInTheDocument();
  });
});
