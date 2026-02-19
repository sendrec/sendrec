import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Settings } from "./Settings";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

function renderSettings() {
  return render(
    <MemoryRouter>
      <Settings />
    </MemoryRouter>
  );
}

describe("Settings", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows loading state initially", () => {
    mockApiFetch.mockReturnValue(new Promise(() => {})); // never resolves
    renderSettings();
    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("loads and displays profile", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([]);
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("alice@example.com")).toBeInTheDocument();
    });
    expect(screen.getByDisplayValue("Alice")).toBeInTheDocument();
  });

  it("disables email field", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([]);
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("alice@example.com")).toBeDisabled();
    });
  });

  it("disables save button when name is unchanged", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([]);
    renderSettings();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Save name" })).toBeDisabled();
    });
  });

  it("enables save button when name changes", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([]);
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Alice")).toBeInTheDocument();
    });

    await user.clear(screen.getByDisplayValue("Alice"));
    await user.type(screen.getByLabelText("Name"), "Bob");

    expect(screen.getByRole("button", { name: "Save name" })).toBeEnabled();
  });

  it("updates name successfully", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([]) // api-keys
      .mockResolvedValueOnce(undefined); // PATCH response
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Alice")).toBeInTheDocument();
    });

    await user.clear(screen.getByDisplayValue("Alice"));
    await user.type(screen.getByLabelText("Name"), "Bob");
    await user.click(screen.getByRole("button", { name: "Save name" }));

    await waitFor(() => {
      expect(screen.getByText("Name updated")).toBeInTheDocument();
    });

    expect(mockApiFetch).toHaveBeenCalledWith("/api/user", {
      method: "PATCH",
      body: JSON.stringify({ name: "Bob" }),
    });
  });

  it("shows error when name update fails", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Failed to update name"));
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Alice")).toBeInTheDocument();
    });

    await user.clear(screen.getByDisplayValue("Alice"));
    await user.type(screen.getByLabelText("Name"), "Bob");
    await user.click(screen.getByRole("button", { name: "Save name" }));

    await waitFor(() => {
      expect(screen.getByText("Failed to update name")).toBeInTheDocument();
    });
  });

  it("shows error when passwords do not match", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([]);
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Alice")).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText("Current password"), "oldpass123");
    await user.type(screen.getByLabelText(/^New password/), "newpass123");
    await user.type(screen.getByLabelText("Confirm new password"), "different1");
    await user.click(screen.getByRole("button", { name: "Change password" }));

    expect(screen.getByText("Passwords do not match")).toBeInTheDocument();
  });

  it("updates password successfully", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([]) // api-keys
      .mockResolvedValueOnce(undefined); // PATCH response
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Alice")).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText("Current password"), "oldpass123");
    await user.type(screen.getByLabelText(/^New password/), "newpass456");
    await user.type(screen.getByLabelText("Confirm new password"), "newpass456");
    await user.click(screen.getByRole("button", { name: "Change password" }));

    await waitFor(() => {
      expect(screen.getByText("Password updated")).toBeInTheDocument();
    });

    expect(mockApiFetch).toHaveBeenCalledWith("/api/user", {
      method: "PATCH",
      body: JSON.stringify({ currentPassword: "oldpass123", newPassword: "newpass456" }),
    });
  });

  it("clears password fields after successful change", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([]) // api-keys
      .mockResolvedValueOnce(undefined); // PATCH response
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Alice")).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText("Current password"), "oldpass123");
    await user.type(screen.getByLabelText(/^New password/), "newpass456");
    await user.type(screen.getByLabelText("Confirm new password"), "newpass456");
    await user.click(screen.getByRole("button", { name: "Change password" }));

    await waitFor(() => {
      expect(screen.getByText("Password updated")).toBeInTheDocument();
    });

    expect(screen.getByLabelText("Current password")).toHaveValue("");
    expect(screen.getByLabelText(/^New password/)).toHaveValue("");
    expect(screen.getByLabelText("Confirm new password")).toHaveValue("");
  });

  it("loads and displays notification preference", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "views_and_comments" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([]);
    renderSettings();

    await waitFor(() => {
      const select = screen.getByLabelText("Notifications") as HTMLSelectElement;
      expect(select.value).toBe("views_and_comments");
    });
  });

  it("defaults notification preference to off", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([]);
    renderSettings();

    await waitFor(() => {
      const select = screen.getByLabelText("Notifications") as HTMLSelectElement;
      expect(select.value).toBe("off");
    });
  });

  it("updates notification preference on change", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([]) // api-keys
      .mockResolvedValueOnce(undefined); // PUT response
    renderSettings();

    await waitFor(() => {
      expect(screen.getByLabelText("Notifications")).toBeInTheDocument();
    });

    await user.selectOptions(screen.getByLabelText("Notifications"), "digest");

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/settings/notifications", {
        method: "PUT",
        body: JSON.stringify({ notificationMode: "digest" }),
      });
    });
  });

  it("shows saved confirmation after notification change", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([]) // api-keys
      .mockResolvedValueOnce(undefined); // PUT response
    renderSettings();

    await waitFor(() => {
      expect(screen.getByLabelText("Notifications")).toBeInTheDocument();
    });

    await user.selectOptions(screen.getByLabelText("Notifications"), "views_only");

    await waitFor(() => {
      expect(screen.getByText("Preference saved")).toBeInTheDocument();
    });
  });

  it("shows branding section when enabled", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: true })
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ companyName: null, colorBackground: null, colorSurface: null, colorText: null, colorAccent: null, footerText: null, logoKey: null, customCss: null });
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText("Branding")).toBeInTheDocument();
    });
  });

  it("hides branding section when disabled", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([]);
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("alice@example.com")).toBeInTheDocument();
    });
    expect(screen.queryByText("Branding")).not.toBeInTheDocument();
  });

  it("loads existing branding settings", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: true })
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ companyName: "Acme Corp", colorBackground: "#112233", colorSurface: null, colorText: null, colorAccent: null, footerText: null, logoKey: null, customCss: null });
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Corp")).toBeInTheDocument();
    });
  });

  it("saves branding settings", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: true })
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ companyName: null, colorBackground: null, colorSurface: null, colorText: null, colorAccent: null, footerText: null, logoKey: null, customCss: null })
      .mockResolvedValueOnce(undefined); // PUT response
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText("Branding")).toBeInTheDocument();
    });

    await user.type(screen.getByPlaceholderText("SendRec"), "My Company");
    await user.click(screen.getByRole("button", { name: "Save branding" }));

    await waitFor(() => {
      expect(screen.getByText("Branding saved")).toBeInTheDocument();
    });
  });

  it("resets branding to defaults", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: true })
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ companyName: "Acme Corp", colorBackground: "#112233", colorSurface: null, colorText: null, colorAccent: null, footerText: null, logoKey: null, customCss: null });
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Corp")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Reset to defaults" }));

    expect(screen.queryByDisplayValue("Acme Corp")).not.toBeInTheDocument();
  });

  it("shows API Keys section", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([]);
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText("API Keys")).toBeInTheDocument();
    });
  });

  it("displays existing API keys", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([
        { id: "key-1", name: "My Nextcloud", createdAt: "2026-02-16T10:00:00Z", lastUsedAt: "2026-02-16T12:00:00Z" },
        { id: "key-2", name: "", createdAt: "2026-02-15T10:00:00Z", lastUsedAt: null },
      ]);
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText("My Nextcloud")).toBeInTheDocument();
    });
    expect(screen.getByText("Unnamed key")).toBeInTheDocument();
  });

  it("creates a new API key and shows it", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ id: "new-key-1", key: "sr_abc123", name: "Test Key", createdAt: "2026-02-16T10:00:00Z" });
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText("API Keys")).toBeInTheDocument();
    });

    await user.type(screen.getByPlaceholderText("e.g. My Nextcloud"), "Test Key");
    await user.click(screen.getByRole("button", { name: "Create key" }));

    await waitFor(() => {
      expect(screen.getByText("sr_abc123")).toBeInTheDocument();
    });
    expect(screen.getByText("Copy this key now — it won't be shown again")).toBeInTheDocument();
    expect(screen.getByText("Test Key")).toBeInTheDocument();
  });

  it("deletes an API key", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([
        { id: "key-1", name: "My Key", createdAt: "2026-02-16T10:00:00Z", lastUsedAt: null },
      ])
      .mockResolvedValueOnce(undefined); // DELETE response
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText("My Key")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(screen.queryByText("My Key")).not.toBeInTheDocument();
    });

    expect(mockApiFetch).toHaveBeenCalledWith("/api/settings/api-keys/key-1", { method: "DELETE" });
  });

  it("shows custom CSS textarea when branding enabled", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: true })
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ companyName: null, colorBackground: null, colorSurface: null, colorText: null, colorAccent: null, footerText: null, logoKey: null, customCss: null });
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText("Custom CSS")).toBeInTheDocument();
    });
  });

  it("includes custom CSS in save payload", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: true })
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ companyName: null, colorBackground: null, colorSurface: null, colorText: null, colorAccent: null, footerText: null, logoKey: null, customCss: null })
      .mockResolvedValueOnce(undefined); // PUT response
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText("Custom CSS")).toBeInTheDocument();
    });

    const textarea = screen.getByPlaceholderText(/Override watch page styles/);
    fireEvent.change(textarea, { target: { value: "body { color: red; }" } });
    await user.click(screen.getByRole("button", { name: "Save branding" }));

    await waitFor(() => {
      expect(screen.getByText("Branding saved")).toBeInTheDocument();
    });

    const saveCall = mockApiFetch.mock.calls.find(
      (call: unknown[]) => call[0] === "/api/settings/branding" && (call[1] as { method: string })?.method === "PUT"
    );
    expect(saveCall).toBeDefined();
    const payload = JSON.parse((saveCall![1] as { body: string }).body);
    expect(payload.customCss).toBe("body { color: red; }");
  });

  it("shows error when API key creation fails", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("maximum number of API keys reached"));
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText("API Keys")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Create key" }));

    await waitFor(() => {
      expect(screen.getByText("maximum number of API keys reached")).toBeInTheDocument();
    });
  });
});
