import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Layout } from "./Layout";

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

const mockSetAccessToken = vi.fn();
vi.mock("../api/client", () => ({
  setAccessToken: (...args: unknown[]) => mockSetAccessToken(...args),
}));

function renderLayout(path = "/") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Layout>
        <div>Page content</div>
      </Layout>
    </MemoryRouter>
  );
}

describe("Layout", () => {
  beforeEach(() => {
    mockNavigate.mockReset();
    mockSetAccessToken.mockReset();
    globalThis.fetch = vi.fn().mockResolvedValue({});
  });

  it("renders navigation links", () => {
    renderLayout();
    expect(screen.getByRole("link", { name: /SendRec/ })).toHaveAttribute("href", "/");
    expect(screen.getByRole("link", { name: "Record" })).toHaveAttribute("href", "/");
    expect(screen.getByRole("link", { name: "Library" })).toHaveAttribute("href", "/library");
    expect(screen.getByRole("link", { name: "Settings" })).toHaveAttribute("href", "/settings");
  });

  it("renders logo image in nav", () => {
    renderLayout();
    const logo = screen.getByAltText("SendRec");
    expect(logo).toBeInTheDocument();
    expect(logo).toHaveAttribute("src", "/images/logo.png");
  });

  it("renders children in main element", () => {
    renderLayout();
    expect(screen.getByText("Page content")).toBeInTheDocument();
    expect(screen.getByRole("main")).toContainElement(screen.getByText("Page content"));
  });

  it("highlights active link for Record on /", () => {
    renderLayout("/");
    const recordLink = screen.getByRole("link", { name: "Record" });
    expect(recordLink).toHaveStyle({ fontWeight: 700 });
  });

  it("highlights active link for Library on /library", () => {
    renderLayout("/library");
    const libraryLink = screen.getByRole("link", { name: "Library" });
    expect(libraryLink).toHaveStyle({ fontWeight: 700 });
    const recordLink = screen.getByRole("link", { name: "Record" });
    expect(recordLink).toHaveStyle({ fontWeight: 400 });
  });

  it("signs out on button click", async () => {
    const user = userEvent.setup();
    renderLayout();

    await user.click(screen.getByRole("button", { name: "Sign out" }));

    expect(globalThis.fetch).toHaveBeenCalledWith("/api/auth/logout", {
      method: "POST",
      credentials: "include",
    });
    expect(mockSetAccessToken).toHaveBeenCalledWith(null);
    expect(mockNavigate).toHaveBeenCalledWith("/login");
  });
});
