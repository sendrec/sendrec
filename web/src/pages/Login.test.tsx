import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Login } from "./Login";

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

const { MockApiError, mockApiFetch, mockSetAccessToken } = vi.hoisted(() => {
  class MockApiError extends Error {
    constructor(
      public readonly status: number,
      message: string
    ) {
      super(message);
      this.name = "ApiError";
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

function renderLogin() {
  return render(
    <MemoryRouter>
      <Login />
    </MemoryRouter>
  );
}

describe("Login", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    mockSetAccessToken.mockReset();
    mockNavigate.mockReset();
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

  it("redirects to check-email on unverified email error", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockRejectedValueOnce(new MockApiError(403, "email_not_verified"));
    renderLogin();

    await user.type(screen.getByLabelText("Email"), "alice@example.com");
    await user.type(screen.getByLabelText("Password"), "password123");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(mockNavigate).toHaveBeenCalledWith("/check-email", { state: { email: "alice@example.com" } });
  });
});
