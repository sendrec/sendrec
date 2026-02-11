import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { ResetPassword } from "./ResetPassword";

const originalFetch = globalThis.fetch;

function renderResetPassword(initialEntries: string[] = ["/reset-password"]) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <ResetPassword />
    </MemoryRouter>
  );
}

describe("ResetPassword", () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it("shows invalid link message when no token", () => {
    renderResetPassword(["/reset-password"]);

    expect(
      screen.getByRole("heading", { name: "Invalid reset link" })
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Request new reset link" })
    ).toHaveAttribute("href", "/forgot-password");
  });

  it("renders password form when token present", () => {
    renderResetPassword(["/reset-password?token=abc123"]);

    expect(
      screen.getByRole("heading", { name: "Set new password" })
    ).toBeInTheDocument();
    expect(screen.getByLabelText("New password")).toBeInTheDocument();
    expect(screen.getByLabelText("Confirm password")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Reset password" })
    ).toBeInTheDocument();
  });

  it("shows error when passwords don't match", async () => {
    const user = userEvent.setup();
    renderResetPassword(["/reset-password?token=abc123"]);

    await user.type(screen.getByLabelText("New password"), "password123");
    await user.type(screen.getByLabelText("Confirm password"), "different99");
    await user.click(screen.getByRole("button", { name: "Reset password" }));

    expect(screen.getByText("Passwords do not match")).toBeInTheDocument();
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("shows error when password too short", async () => {
    const user = userEvent.setup();
    renderResetPassword(["/reset-password?token=abc123"]);

    await user.type(screen.getByLabelText("New password"), "short");
    await user.type(screen.getByLabelText("Confirm password"), "short");
    await user.click(screen.getByRole("button", { name: "Reset password" }));

    expect(
      screen.getByText("Password must be at least 8 characters")
    ).toBeInTheDocument();
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("submits and shows success on valid request", async () => {
    const user = userEvent.setup();
    vi.mocked(globalThis.fetch).mockResolvedValueOnce({
      ok: true,
      json: async () => ({}),
    } as Response);

    renderResetPassword(["/reset-password?token=abc123"]);

    await user.type(screen.getByLabelText("New password"), "newpass456");
    await user.type(screen.getByLabelText("Confirm password"), "newpass456");
    await user.click(screen.getByRole("button", { name: "Reset password" }));

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Password updated" })
      ).toBeInTheDocument();
    });

    expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute(
      "href",
      "/login"
    );

    expect(globalThis.fetch).toHaveBeenCalledWith("/api/auth/reset-password", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token: "abc123", password: "newpass456" }),
    });
  });

  it("shows error on failed API request", async () => {
    const user = userEvent.setup();
    vi.mocked(globalThis.fetch).mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: "Token expired" }),
    } as Response);

    renderResetPassword(["/reset-password?token=expired-token"]);

    await user.type(screen.getByLabelText("New password"), "newpass456");
    await user.type(screen.getByLabelText("Confirm password"), "newpass456");
    await user.click(screen.getByRole("button", { name: "Reset password" }));

    await waitFor(() => {
      expect(screen.getByText("Token expired")).toBeInTheDocument();
    });
  });
});
