import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, act, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { CheckEmail } from "./CheckEmail";

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

function renderCheckEmail(email?: string) {
  return render(
    <MemoryRouter initialEntries={[{ pathname: "/check-email", state: email ? { email } : undefined }]}>
      <CheckEmail />
    </MemoryRouter>
  );
}

describe("CheckEmail", () => {
  beforeEach(() => {
    mockNavigate.mockReset();
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders heading and email address", () => {
    renderCheckEmail("alice@example.com");
    expect(screen.getByRole("heading", { name: "Check your email" })).toBeInTheDocument();
    expect(screen.getByText(/alice@example\.com/)).toBeInTheDocument();
  });

  it("renders resend button and sign in link", () => {
    renderCheckEmail("alice@example.com");
    expect(screen.getByRole("button", { name: "Resend confirmation email" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to sign in" })).toHaveAttribute("href", "/login");
  });

  it("redirects to register when no email in state", () => {
    renderCheckEmail();
    expect(mockNavigate).toHaveBeenCalledWith("/register", { replace: true });
  });

  it("calls resend API and shows success message", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(new Response(JSON.stringify({}), { status: 200 }));
    renderCheckEmail("alice@example.com");

    await user.click(screen.getByRole("button", { name: "Resend confirmation email" }));

    expect(globalThis.fetch).toHaveBeenCalledWith("/api/auth/resend-confirmation", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email: "alice@example.com" }),
    });
    expect(screen.getByText("Confirmation email resent.")).toBeInTheDocument();
  });

  it("shows cooldown after resend", async () => {
    vi.useFakeTimers();
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({}),
    } as unknown as Response);
    renderCheckEmail("alice@example.com");

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Resend confirmation email" }));
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(screen.getByRole("button", { name: "Resend in 60s" })).toBeDisabled();

    for (let i = 0; i < 60; i++) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1000);
      });
    }

    expect(screen.getByRole("button", { name: "Resend confirmation email" })).not.toBeDisabled();
  });

  it("shows error on resend failure", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "rate limited" }), { status: 429 })
    );
    renderCheckEmail("alice@example.com");

    await user.click(screen.getByRole("button", { name: "Resend confirmation email" }));

    expect(screen.getByText("rate limited")).toBeInTheDocument();
  });
});
