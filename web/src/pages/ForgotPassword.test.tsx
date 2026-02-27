import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { ForgotPassword } from "./ForgotPassword";
import { expectNoA11yViolations } from "../test-utils/a11y";

const originalFetch = globalThis.fetch;

function renderForgotPassword() {
  return render(
    <MemoryRouter>
      <ForgotPassword />
    </MemoryRouter>
  );
}

describe("ForgotPassword", () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it("renders form with email input and submit button", () => {
    renderForgotPassword();

    expect(
      screen.getByRole("heading", { name: "Reset password" })
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Send reset link" })
    ).toBeInTheDocument();
  });

  it("has no accessibility violations", async () => {
    const { container } = renderForgotPassword();
    await expectNoA11yViolations(container);
  });

  it("submits email and shows success message", async () => {
    const user = userEvent.setup();
    vi.mocked(globalThis.fetch).mockResolvedValueOnce({
      ok: true,
      json: async () => ({}),
    } as Response);

    renderForgotPassword();

    await user.type(screen.getByLabelText("Email"), "alice@example.com");
    await user.click(screen.getByRole("button", { name: "Send reset link" }));

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Check your email" })
      ).toBeInTheDocument();
    });

    expect(globalThis.fetch).toHaveBeenCalledWith(
      "/api/auth/forgot-password",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: "alice@example.com" }),
      }
    );
  });

  it("shows error on failed request", async () => {
    const user = userEvent.setup();
    vi.mocked(globalThis.fetch).mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: "User not found" }),
    } as Response);

    renderForgotPassword();

    await user.type(screen.getByLabelText("Email"), "unknown@example.com");
    await user.click(screen.getByRole("button", { name: "Send reset link" }));

    await waitFor(() => {
      expect(screen.getByText("User not found")).toBeInTheDocument();
    });
  });

  it("shows loading state during submit", async () => {
    const user = userEvent.setup();
    vi.mocked(globalThis.fetch).mockReturnValueOnce(
      new Promise(() => {}) // never resolves
    );

    renderForgotPassword();

    await user.type(screen.getByLabelText("Email"), "alice@example.com");
    await user.click(screen.getByRole("button", { name: "Send reset link" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Sending..." })).toBeDisabled();
    });
  });

  it("has link back to login", () => {
    renderForgotPassword();

    expect(
      screen.getByRole("link", { name: "Back to sign in" })
    ).toHaveAttribute("href", "/login");
  });
});
