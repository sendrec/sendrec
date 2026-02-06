import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { AuthForm } from "./AuthForm";

function renderAuthForm(props: Partial<Parameters<typeof AuthForm>[0]> = {}) {
  const defaults = {
    title: "Sign in",
    submitLabel: "Sign in",
    onSubmit: vi.fn().mockResolvedValue(undefined),
    footer: <span>footer content</span>,
  };
  return render(
    <MemoryRouter>
      <AuthForm {...defaults} {...props} />
    </MemoryRouter>
  );
}

describe("AuthForm", () => {
  it("renders title and submit button", () => {
    renderAuthForm({ title: "Sign in", submitLabel: "Sign in" });
    expect(screen.getByRole("heading", { name: "Sign in" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sign in" })).toBeInTheDocument();
  });

  it("renders email and password fields", () => {
    renderAuthForm();
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
    expect(screen.getByLabelText("Password")).toBeInTheDocument();
  });

  it("does not render name field by default", () => {
    renderAuthForm();
    expect(screen.queryByLabelText("Name")).not.toBeInTheDocument();
  });

  it("renders name field when showName is true", () => {
    renderAuthForm({ showName: true });
    expect(screen.getByLabelText("Name")).toBeInTheDocument();
  });

  it("renders confirm password when showPasswordConfirm is true", () => {
    renderAuthForm({ showPasswordConfirm: true });
    expect(screen.getByLabelText("Confirm password")).toBeInTheDocument();
  });

  it("shows error when passwords do not match", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    renderAuthForm({ showPasswordConfirm: true, onSubmit });

    await user.type(screen.getByLabelText("Email"), "test@example.com");
    await user.type(screen.getByLabelText(/^Password/), "password123");
    await user.type(screen.getByLabelText("Confirm password"), "different123");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(screen.getByText("Passwords do not match")).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("calls onSubmit with form data", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    renderAuthForm({ onSubmit });

    await user.type(screen.getByLabelText("Email"), "test@example.com");
    await user.type(screen.getByLabelText("Password"), "password123");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(onSubmit).toHaveBeenCalledWith({
      email: "test@example.com",
      password: "password123",
      name: "",
    });
  });

  it("shows error message when onSubmit throws", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockRejectedValue(new Error("Invalid credentials"));
    renderAuthForm({ onSubmit });

    await user.type(screen.getByLabelText("Email"), "test@example.com");
    await user.type(screen.getByLabelText("Password"), "password123");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(screen.getByText("Invalid credentials")).toBeInTheDocument();
  });

  it("shows loading state during submit", async () => {
    const user = userEvent.setup();
    let resolveSubmit: () => void;
    const onSubmit = vi.fn().mockReturnValue(
      new Promise<void>((resolve) => {
        resolveSubmit = resolve;
      })
    );
    renderAuthForm({ onSubmit });

    await user.type(screen.getByLabelText("Email"), "test@example.com");
    await user.type(screen.getByLabelText("Password"), "password123");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(screen.getByRole("button", { name: "Loading..." })).toBeDisabled();
    resolveSubmit!();
  });

  it("renders footer content", () => {
    renderAuthForm({ footer: <span>Go to register</span> });
    expect(screen.getByText("Go to register")).toBeInTheDocument();
  });
});
