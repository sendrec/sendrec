import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Register } from "./Register";

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

const mockApiFetch = vi.fn();
const mockSetAccessToken = vi.fn();
vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
  setAccessToken: (...args: unknown[]) => mockSetAccessToken(...args),
}));

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
    mockSetAccessToken.mockReset();
    mockNavigate.mockReset();
  });

  it("renders create account form with name and password confirm fields", () => {
    renderRegister();
    expect(screen.getByRole("heading", { name: "Create account" })).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toBeInTheDocument();
    expect(screen.getByLabelText("Confirm password")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create account" })).toBeInTheDocument();
  });

  it("renders sign in link", () => {
    renderRegister();
    expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
  });

  it("registers and navigates to home on success", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce({ accessToken: "tok-456" });
    renderRegister();

    await user.type(screen.getByLabelText("Name"), "Alice");
    await user.type(screen.getByLabelText("Email"), "alice@example.com");
    await user.type(screen.getByLabelText(/^Password/), "password123");
    await user.type(screen.getByLabelText("Confirm password"), "password123");
    await user.click(screen.getByRole("button", { name: "Create account" }));

    expect(mockApiFetch).toHaveBeenCalledWith("/api/auth/register", {
      method: "POST",
      body: JSON.stringify({ email: "alice@example.com", password: "password123", name: "Alice" }),
    });
    expect(mockSetAccessToken).toHaveBeenCalledWith("tok-456");
    expect(mockNavigate).toHaveBeenCalledWith("/");
  });

  it("shows error on failed registration", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockRejectedValueOnce(new Error("could not create account"));
    renderRegister();

    await user.type(screen.getByLabelText("Name"), "Alice");
    await user.type(screen.getByLabelText("Email"), "alice@example.com");
    await user.type(screen.getByLabelText(/^Password/), "password123");
    await user.type(screen.getByLabelText("Confirm password"), "password123");
    await user.click(screen.getByRole("button", { name: "Create account" }));

    expect(screen.getByText("could not create account")).toBeInTheDocument();
  });
});
