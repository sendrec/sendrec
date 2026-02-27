import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { ConfirmEmail } from "./ConfirmEmail";
import { expectNoA11yViolations } from "../test-utils/a11y";

function renderConfirmEmail(token?: string) {
  const path = token ? `/confirm-email?token=${token}` : "/confirm-email";
  return render(
    <MemoryRouter initialEntries={[path]}>
      <ConfirmEmail />
    </MemoryRouter>
  );
}

describe("ConfirmEmail", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("shows loading state then success on valid token", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ message: "Email confirmed." }), { status: 200 })
    );
    renderConfirmEmail("valid-token");

    expect(screen.getByRole("heading", { name: "Confirming your email..." })).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Email confirmed" })).toBeInTheDocument();
    });

    expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
    expect(globalThis.fetch).toHaveBeenCalledWith("/api/auth/confirm-email", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token: "valid-token" }),
    });
  });

  it("shows error on invalid token", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "invalid or expired token" }), { status: 400 })
    );
    renderConfirmEmail("bad-token");

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Confirmation failed" })).toBeInTheDocument();
    });

    expect(screen.getByText("invalid or expired token")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Try again" })).toHaveAttribute("href", "/register");
  });

  it("has no accessibility violations", async () => {
    const { container } = renderConfirmEmail();
    await expectNoA11yViolations(container);
  });

  it("shows error when token is missing", () => {
    renderConfirmEmail();
    expect(screen.getByRole("heading", { name: "Confirmation failed" })).toBeInTheDocument();
    expect(screen.getByText("Missing confirmation token.")).toBeInTheDocument();
  });
});
