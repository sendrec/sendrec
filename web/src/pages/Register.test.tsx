import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Register } from "./Register";
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

function mockHealthResponse(registrationEnabled: boolean) {
  vi.spyOn(global, "fetch").mockResolvedValueOnce(
    new Response(JSON.stringify({ registrationEnabled }), { status: 200 })
  );
}

function renderRegister() {
  return render(
    <MemoryRouter>
      <Register />
    </MemoryRouter>
  );
}

describe("Register", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    mockNavigate.mockReset();
    vi.restoreAllMocks();
  });

  it("renders create account form with name and password confirm fields", async () => {
    mockHealthResponse(true);
    renderRegister();
    expect(await screen.findByRole("heading", { name: "Create account" })).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toBeInTheDocument();
    expect(screen.getByLabelText("Confirm password")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create account" })).toBeInTheDocument();
  });

  it("renders sign in link", async () => {
    mockHealthResponse(true);
    renderRegister();
    expect(await screen.findByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
  });

  it("registers and navigates to check-email on success", async () => {
    mockHealthResponse(true);
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce({ message: "Account created. Check your email to confirm." });
    renderRegister();

    await user.type(await screen.findByLabelText("Name"), "Alice");
    await user.type(screen.getByLabelText("Email"), "alice@example.com");
    await user.type(screen.getByLabelText(/^Password/), "password123");
    await user.type(screen.getByLabelText("Confirm password"), "password123");
    await user.click(screen.getByRole("button", { name: "Create account" }));

    expect(mockApiFetch).toHaveBeenCalledWith("/api/auth/register", {
      method: "POST",
      body: JSON.stringify({ email: "alice@example.com", password: "password123", name: "Alice" }),
    });
    expect(mockNavigate).toHaveBeenCalledWith("/check-email", { state: { email: "alice@example.com" } });
  });

  it("has no accessibility violations", async () => {
    mockHealthResponse(true);
    const { container } = renderRegister();
    await screen.findByRole("heading", { name: "Create account" });
    await expectNoA11yViolations(container);
  });

  it("shows error on failed registration", async () => {
    mockHealthResponse(true);
    const user = userEvent.setup();
    mockApiFetch.mockRejectedValueOnce(new Error("could not create account"));
    renderRegister();

    await user.type(await screen.findByLabelText("Name"), "Alice");
    await user.type(screen.getByLabelText("Email"), "alice@example.com");
    await user.type(screen.getByLabelText(/^Password/), "password123");
    await user.type(screen.getByLabelText("Confirm password"), "password123");
    await user.click(screen.getByRole("button", { name: "Create account" }));

    expect(screen.getByText("could not create account")).toBeInTheDocument();
  });

  it("redirects to login when registration is disabled", async () => {
    mockHealthResponse(false);
    renderRegister();

    await vi.waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/login", { replace: true });
    });
  });

  it("shows form when registration is enabled", async () => {
    mockHealthResponse(true);
    renderRegister();

    await vi.waitFor(() => {
      expect(screen.getByRole("heading", { name: "Create account" })).toBeInTheDocument();
    });
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
