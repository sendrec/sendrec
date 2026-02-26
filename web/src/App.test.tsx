import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { App } from "./App";

const mockGetAccessToken = vi.fn();
const mockTryRefreshToken = vi.fn();

vi.mock("./api/client", () => ({
  getAccessToken: () => mockGetAccessToken(),
  tryRefreshToken: () => mockTryRefreshToken(),
}));

vi.mock("./pages/Login", () => ({
  Login: () => <div>Login Page</div>,
}));

vi.mock("./pages/Register", () => ({
  Register: () => <div>Register Page</div>,
}));

vi.mock("./pages/ForgotPassword", () => ({
  ForgotPassword: () => <div>Forgot Password Page</div>,
}));

vi.mock("./pages/ResetPassword", () => ({
  ResetPassword: () => <div>Reset Password Page</div>,
}));

vi.mock("./pages/CheckEmail", () => ({
  CheckEmail: () => <div>Check Email Page</div>,
}));

vi.mock("./pages/ConfirmEmail", () => ({
  ConfirmEmail: () => <div>Confirm Email Page</div>,
}));

vi.mock("./pages/Record", () => ({
  Record: () => <div>Record Page</div>,
}));

vi.mock("./pages/Library", () => ({
  Library: () => <div>Library Page</div>,
}));

vi.mock("./pages/Settings", () => ({
  Settings: () => <div>Settings Page</div>,
}));

vi.mock("./pages/Playlists", () => ({
  Playlists: () => <div>Playlists Page</div>,
}));

vi.mock("./pages/Analytics", () => ({
  Analytics: () => <div>Analytics Page</div>,
}));

vi.mock("./pages/AnalyticsDashboard", () => ({
  AnalyticsDashboard: () => <div>Analytics Dashboard Page</div>,
}));

vi.mock("./pages/NotFound", () => ({
  NotFound: () => <div>Not Found Page</div>,
}));

vi.mock("./components/Layout", () => ({
  Layout: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="layout">{children}</div>
  ),
}));

function renderApp(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>
  );
}

describe("App", () => {
  beforeEach(() => {
    mockGetAccessToken.mockReset();
    mockTryRefreshToken.mockReset();
  });

  it("renders login page at /login", () => {
    renderApp("/login");
    expect(screen.getByText("Login Page")).toBeInTheDocument();
  });

  it("renders register page at /register", () => {
    renderApp("/register");
    expect(screen.getByText("Register Page")).toBeInTheDocument();
  });

  it("renders NotFound for unknown routes", () => {
    renderApp("/some-unknown-path");
    expect(screen.getByText("Not Found Page")).toBeInTheDocument();
  });

  it("redirects to login when not authenticated on protected route", async () => {
    mockGetAccessToken.mockReturnValue(null);
    mockTryRefreshToken.mockResolvedValue(false);

    renderApp("/");

    await waitFor(() => {
      expect(screen.getByText("Login Page")).toBeInTheDocument();
    });
  });

  it("renders protected content when authenticated", () => {
    mockGetAccessToken.mockReturnValue("valid-token");

    renderApp("/");

    expect(screen.getByTestId("layout")).toBeInTheDocument();
    expect(screen.getByText("Record Page")).toBeInTheDocument();
  });

  it("renders per-video analytics page for authenticated users", () => {
    mockGetAccessToken.mockReturnValue("valid-token");

    renderApp("/videos/v1/analytics");

    expect(screen.getByTestId("layout")).toBeInTheDocument();
    expect(screen.getByText("Analytics Page")).toBeInTheDocument();
  });

  it("renders analytics dashboard at /analytics", () => {
    mockGetAccessToken.mockReturnValue("valid-token");

    renderApp("/analytics");

    expect(screen.getByTestId("layout")).toBeInTheDocument();
    expect(screen.getByText("Analytics Dashboard Page")).toBeInTheDocument();
  });

  it("redirects /upload to Record page", () => {
    mockGetAccessToken.mockReturnValue("valid-token");

    renderApp("/upload");

    expect(screen.getByTestId("layout")).toBeInTheDocument();
    expect(screen.getByText("Record Page")).toBeInTheDocument();
  });

  it("renders Playlists page at /playlists", () => {
    mockGetAccessToken.mockReturnValue("valid-token");

    renderApp("/playlists");

    expect(screen.getByTestId("layout")).toBeInTheDocument();
    expect(screen.getByText("Playlists Page")).toBeInTheDocument();
  });
});
