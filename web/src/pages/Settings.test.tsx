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
    localStorage.clear();
    window.matchMedia = vi.fn().mockReturnValue({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    });
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
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
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
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
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
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
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
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
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
      .mockRejectedValueOnce(new Error("Not Found")) // billing
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
      .mockRejectedValueOnce(new Error("Not Found")) // billing
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
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
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
      .mockRejectedValueOnce(new Error("Not Found")) // billing
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
      .mockRejectedValueOnce(new Error("Not Found")) // billing
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
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
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
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
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
      .mockRejectedValueOnce(new Error("Not Found")) // billing
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
      .mockRejectedValueOnce(new Error("Not Found")) // billing
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
      .mockResolvedValueOnce({ companyName: null, colorBackground: null, colorSurface: null, colorText: null, colorAccent: null, footerText: null, logoKey: null, customCss: null })
      .mockRejectedValueOnce(new Error("Not Found")); // billing
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
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
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
      .mockResolvedValueOnce({ companyName: "Acme Corp", colorBackground: "#112233", colorSurface: null, colorText: null, colorAccent: null, footerText: null, logoKey: null, customCss: null })
      .mockRejectedValueOnce(new Error("Not Found")); // billing
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
      .mockRejectedValueOnce(new Error("Not Found")) // billing
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
      .mockResolvedValueOnce({ companyName: "Acme Corp", colorBackground: "#112233", colorSurface: null, colorText: null, colorAccent: null, footerText: null, logoKey: null, customCss: null })
      .mockRejectedValueOnce(new Error("Not Found")); // billing
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
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
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
      ])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
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
      .mockRejectedValueOnce(new Error("Not Found")) // billing
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
      .mockRejectedValueOnce(new Error("Not Found")) // billing
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
      .mockResolvedValueOnce({ companyName: null, colorBackground: null, colorSurface: null, colorText: null, colorAccent: null, footerText: null, logoKey: null, customCss: null })
      .mockRejectedValueOnce(new Error("Not Found")); // billing
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
      .mockRejectedValueOnce(new Error("Not Found")) // billing
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
      .mockRejectedValueOnce(new Error("Not Found")) // billing
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

  it("renders Slack webhook URL input", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off", slackWebhookUrl: null })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText("Slack Notifications")).toBeInTheDocument();
    });
    expect(screen.getByPlaceholderText("https://hooks.slack.com/services/...")).toBeInTheDocument();
  });

  it("saves Slack webhook URL", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off", slackWebhookUrl: null })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")) // billing
      .mockResolvedValueOnce(undefined); // PUT response
    renderSettings();

    await waitFor(() => {
      expect(screen.getByPlaceholderText("https://hooks.slack.com/services/...")).toBeInTheDocument();
    });

    await user.type(
      screen.getByPlaceholderText("https://hooks.slack.com/services/..."),
      "https://hooks.slack.com/services/T00/B00/xxx"
    );
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      expect(screen.getByText("Webhook URL saved")).toBeInTheDocument();
    });

    const saveCall = mockApiFetch.mock.calls.find(
      (call: unknown[]) =>
        call[0] === "/api/settings/notifications" &&
        (call[1] as { method: string })?.method === "PUT"
    );
    expect(saveCall).toBeDefined();
    const payload = JSON.parse((saveCall![1] as { body: string }).body);
    expect(payload.slackWebhookUrl).toBe("https://hooks.slack.com/services/T00/B00/xxx");
    expect(payload.notificationMode).toBe("off");
  });

  it("sends Slack test message", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off", slackWebhookUrl: "https://hooks.slack.com/services/T00/B00/xxx" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")) // billing
      .mockResolvedValueOnce(undefined); // POST test-slack response
    renderSettings();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Send test message" })).toBeEnabled();
    });

    await user.click(screen.getByRole("button", { name: "Send test message" }));

    await waitFor(() => {
      expect(screen.getByText("Test message sent")).toBeInTheDocument();
    });

    expect(mockApiFetch).toHaveBeenCalledWith("/api/settings/notifications/test-slack", {
      method: "POST",
    });
  });

  it("shows error for invalid Slack webhook URL", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off", slackWebhookUrl: null })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")) // billing
      .mockRejectedValueOnce(new Error("Invalid webhook URL"));
    renderSettings();

    await waitFor(() => {
      expect(screen.getByPlaceholderText("https://hooks.slack.com/services/...")).toBeInTheDocument();
    });

    await user.type(
      screen.getByPlaceholderText("https://hooks.slack.com/services/..."),
      "not-a-url"
    );
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      expect(screen.getByText("Invalid webhook URL")).toBeInTheDocument();
    });
  });

  it("clears Slack webhook URL", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off", slackWebhookUrl: "https://hooks.slack.com/services/T00/B00/xxx" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")) // billing
      .mockResolvedValueOnce(undefined); // PUT response for clear
    renderSettings();

    await waitFor(() => {
      expect(
        screen.getByDisplayValue("https://hooks.slack.com/services/T00/B00/xxx")
      ).toBeInTheDocument();
    });

    await user.clear(screen.getByPlaceholderText("https://hooks.slack.com/services/..."));
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      expect(screen.getByText("Webhook URL saved")).toBeInTheDocument();
    });

    const saveCall = mockApiFetch.mock.calls.find(
      (call: unknown[]) =>
        call[0] === "/api/settings/notifications" &&
        (call[1] as { method: string })?.method === "PUT"
    );
    expect(saveCall).toBeDefined();
    const payload = JSON.parse((saveCall![1] as { body: string }).body);
    expect(payload.slackWebhookUrl).toBe("");
  });

  it("shows Appearance section", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText("Appearance")).toBeInTheDocument();
    });
  });

  it("defaults to System theme for new users", async () => {
    localStorage.removeItem("theme");
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
    renderSettings();

    await waitFor(() => {
      expect(screen.getByRole("radio", { name: "System" })).toBeChecked();
    });
  });

  it("selects Dark theme after clicking Dark option", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
    renderSettings();

    await waitFor(() => {
      expect(screen.getByRole("radio", { name: "System" })).toBeChecked();
    });

    await user.click(screen.getByRole("radio", { name: "Dark" }));
    expect(screen.getByRole("radio", { name: "Dark" })).toBeChecked();
    expect(localStorage.getItem("theme")).toBe("dark");
  });

  it("switches to Light theme when clicked", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ notificationMode: "off" })
      .mockResolvedValueOnce({ brandingEnabled: false })
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Not Found")); // billing
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText("Appearance")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("radio", { name: "Light" }));
    expect(screen.getByRole("radio", { name: "Light" })).toBeChecked();
    expect(localStorage.getItem("theme")).toBe("light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
  });

  describe("Webhooks", () => {
    it("renders webhook section with URL input", async () => {
      mockApiFetch
        .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
        .mockResolvedValueOnce({ notificationMode: "off", slackWebhookUrl: null, webhookUrl: null, webhookSecret: null })
        .mockResolvedValueOnce({ brandingEnabled: false })
        .mockResolvedValueOnce([])
        .mockRejectedValueOnce(new Error("Not Found")); // billing
      renderSettings();

      await waitFor(() => {
        expect(screen.getByText("Webhooks")).toBeInTheDocument();
      });
      expect(screen.getByPlaceholderText("https://example.com/webhook")).toBeInTheDocument();
    });

    it("saves webhook URL and shows signing secret", async () => {
      const user = userEvent.setup();
      mockApiFetch
        .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
        .mockResolvedValueOnce({ notificationMode: "off", slackWebhookUrl: null, webhookUrl: null, webhookSecret: null })
        .mockResolvedValueOnce({ brandingEnabled: false })
        .mockResolvedValueOnce([])
        .mockRejectedValueOnce(new Error("Not Found")) // billing
        .mockResolvedValueOnce(undefined) // PUT notifications
        .mockResolvedValueOnce({ notificationMode: "off", webhookSecret: "whsec_abc123" }) // GET after save
        .mockResolvedValueOnce([]); // deliveries fetch triggered by savedWebhookUrl change
      renderSettings();

      await waitFor(() => {
        expect(screen.getByPlaceholderText("https://example.com/webhook")).toBeInTheDocument();
      });

      await user.type(
        screen.getByPlaceholderText("https://example.com/webhook"),
        "https://example.com/hook"
      );
      await user.click(screen.getByRole("button", { name: "Save webhook" }));

      await waitFor(() => {
        expect(screen.getByText("Saved")).toBeInTheDocument();
      });
      expect(screen.getByText("whsec_abc123")).toBeInTheDocument();

      const saveCall = mockApiFetch.mock.calls.find(
        (call: unknown[]) =>
          call[0] === "/api/settings/notifications" &&
          (call[1] as { method: string })?.method === "PUT"
      );
      expect(saveCall).toBeDefined();
      const payload = JSON.parse((saveCall![1] as { body: string }).body);
      expect(payload.webhookUrl).toBe("https://example.com/hook");
    });

    it("shows deliveries table when webhook is configured", async () => {
      mockApiFetch
        .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
        .mockResolvedValueOnce({
          notificationMode: "off",
          slackWebhookUrl: null,
          webhookUrl: "https://example.com/hook",
          webhookSecret: "whsec_abc123",
        })
        .mockResolvedValueOnce({ brandingEnabled: false })
        .mockResolvedValueOnce([])
        .mockRejectedValueOnce(new Error("Not Found")) // billing
        .mockResolvedValueOnce([
          {
            id: "del-1",
            event: "video.viewed",
            payload: '{"event":"video.viewed"}',
            statusCode: 200,
            responseBody: "ok",
            attempt: 1,
            createdAt: "2026-02-21T10:00:00Z",
          },
          {
            id: "del-2",
            event: "video.comment.created",
            payload: '{"event":"video.comment.created"}',
            statusCode: 500,
            responseBody: "error",
            attempt: 1,
            createdAt: "2026-02-21T09:00:00Z",
          },
        ]);
      renderSettings();

      await waitFor(() => {
        expect(screen.getByText("Recent deliveries")).toBeInTheDocument();
      });
      // Event names appear in both deliveries table and supported events list
      expect(screen.getAllByText("video.viewed").length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText("video.comment.created").length).toBeGreaterThanOrEqual(1);
      expect(screen.getByText("200")).toBeInTheDocument();
      expect(screen.getByText("500")).toBeInTheDocument();
    });

    it("sends test webhook event", async () => {
      const user = userEvent.setup();
      mockApiFetch
        .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
        .mockResolvedValueOnce({
          notificationMode: "off",
          slackWebhookUrl: null,
          webhookUrl: "https://example.com/hook",
          webhookSecret: "whsec_abc123",
        })
        .mockResolvedValueOnce({ brandingEnabled: false })
        .mockResolvedValueOnce([])
        .mockRejectedValueOnce(new Error("Not Found")) // billing
        .mockResolvedValueOnce([]) // initial deliveries fetch
        .mockResolvedValueOnce(undefined) // POST test-webhook
        .mockResolvedValueOnce([{ // refreshed deliveries
          id: "del-1",
          event: "test",
          payload: '{"event":"test"}',
          statusCode: 200,
          responseBody: "ok",
          attempt: 1,
          createdAt: "2026-02-21T10:00:00Z",
        }]);
      renderSettings();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "Send test event" })).toBeEnabled();
      });

      await user.click(screen.getByRole("button", { name: "Send test event" }));

      await waitFor(() => {
        expect(screen.getByText("Test event sent")).toBeInTheDocument();
      });

      expect(mockApiFetch).toHaveBeenCalledWith("/api/settings/notifications/test-webhook", {
        method: "POST",
      });
    });

    it("disables test button when no webhook URL saved", async () => {
      mockApiFetch
        .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
        .mockResolvedValueOnce({ notificationMode: "off", slackWebhookUrl: null, webhookUrl: null, webhookSecret: null })
        .mockResolvedValueOnce({ brandingEnabled: false })
        .mockResolvedValueOnce([])
        .mockRejectedValueOnce(new Error("Not Found")); // billing
      renderSettings();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "Send test event" })).toBeDisabled();
      });
    });

    it("shows signing secret with copy and regenerate buttons", async () => {
      mockApiFetch
        .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
        .mockResolvedValueOnce({
          notificationMode: "off",
          slackWebhookUrl: null,
          webhookUrl: "https://example.com/hook",
          webhookSecret: "whsec_secret123",
        })
        .mockResolvedValueOnce({ brandingEnabled: false })
        .mockResolvedValueOnce([])
        .mockRejectedValueOnce(new Error("Not Found")) // billing
        .mockResolvedValueOnce([]); // deliveries
      renderSettings();

      await waitFor(() => {
        expect(screen.getByText("whsec_secret123")).toBeInTheDocument();
      });
      expect(screen.getByText("Signing secret")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Copy" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Regenerate" })).toBeInTheDocument();
    });

    it("regenerates signing secret", async () => {
      const user = userEvent.setup();
      mockApiFetch
        .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
        .mockResolvedValueOnce({
          notificationMode: "off",
          slackWebhookUrl: null,
          webhookUrl: "https://example.com/hook",
          webhookSecret: "whsec_old",
        })
        .mockResolvedValueOnce({ brandingEnabled: false })
        .mockResolvedValueOnce([])
        .mockRejectedValueOnce(new Error("Not Found")) // billing
        .mockResolvedValueOnce([]) // deliveries
        .mockResolvedValueOnce({ webhookSecret: "whsec_new" }); // regenerate response
      renderSettings();

      await waitFor(() => {
        expect(screen.getByText("whsec_old")).toBeInTheDocument();
      });

      await user.click(screen.getByRole("button", { name: "Regenerate" }));

      await waitFor(() => {
        expect(screen.getByText("whsec_new")).toBeInTheDocument();
      });
      expect(screen.getByText("Secret regenerated")).toBeInTheDocument();

      expect(mockApiFetch).toHaveBeenCalledWith("/api/settings/notifications/regenerate-webhook-secret", {
        method: "POST",
      });
    });

    it("shows supported events in collapsible section", async () => {
      mockApiFetch
        .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
        .mockResolvedValueOnce({ notificationMode: "off", slackWebhookUrl: null, webhookUrl: null, webhookSecret: null })
        .mockResolvedValueOnce({ brandingEnabled: false })
        .mockResolvedValueOnce([])
        .mockRejectedValueOnce(new Error("Not Found")); // billing
      renderSettings();

      await waitFor(() => {
        expect(screen.getByText("Supported events")).toBeInTheDocument();
      });
    });
  });

  describe("Billing", () => {
    it("shows upgrade button for free users", async () => {
      mockApiFetch
        .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
        .mockResolvedValueOnce({ notificationMode: "off" })
        .mockResolvedValueOnce({ maxVideosPerMonth: 25, maxVideoDurationSeconds: 300, videosUsedThisMonth: 10, brandingEnabled: false, aiEnabled: false })
        .mockResolvedValueOnce([])
        .mockResolvedValueOnce({ plan: "free", subscriptionId: null, portalUrl: null });

      renderSettings();

      await waitFor(() => {
        expect(screen.getByText("Upgrade to Pro")).toBeInTheDocument();
      });
      expect(screen.getByText("Free")).toBeInTheDocument();
    });

    it("shows manage subscription for pro users", async () => {
      mockApiFetch
        .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
        .mockResolvedValueOnce({ notificationMode: "off" })
        .mockResolvedValueOnce({ maxVideosPerMonth: 0, maxVideoDurationSeconds: 0, videosUsedThisMonth: 0, brandingEnabled: false, aiEnabled: false })
        .mockResolvedValueOnce([])
        .mockResolvedValueOnce({ plan: "pro", subscriptionId: "sub_123", portalUrl: "https://portal.creem.io/cust_abc" });

      renderSettings();

      await waitFor(() => {
        expect(screen.getByText("Pro")).toBeInTheDocument();
      });
      expect(screen.getByText("Manage subscription")).toBeInTheDocument();
      expect(screen.getByText("Cancel subscription")).toBeInTheDocument();
    });

    it("hides billing section when not configured", async () => {
      mockApiFetch
        .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
        .mockResolvedValueOnce({ notificationMode: "off" })
        .mockResolvedValueOnce({ maxVideosPerMonth: 25, maxVideoDurationSeconds: 300, videosUsedThisMonth: 10, brandingEnabled: false, aiEnabled: false })
        .mockResolvedValueOnce([])
        .mockRejectedValueOnce(new Error("Not Found"));

      renderSettings();

      await waitFor(() => {
        expect(screen.getByDisplayValue("Alice")).toBeInTheDocument();
      });
      expect(screen.queryByText("Upgrade to Pro")).not.toBeInTheDocument();
      expect(screen.queryByText("Subscription")).not.toBeInTheDocument();
    });

    it("handles upgrade button click", async () => {
      mockApiFetch
        .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
        .mockResolvedValueOnce({ notificationMode: "off" })
        .mockResolvedValueOnce({ maxVideosPerMonth: 25, maxVideoDurationSeconds: 300, videosUsedThisMonth: 10, brandingEnabled: false, aiEnabled: false })
        .mockResolvedValueOnce([])
        .mockResolvedValueOnce({ plan: "free", subscriptionId: null, portalUrl: null });

      renderSettings();

      await waitFor(() => {
        expect(screen.getByText("Upgrade to Pro")).toBeInTheDocument();
      });

      // Mock the checkout endpoint
      mockApiFetch.mockResolvedValueOnce({ checkoutUrl: "https://checkout.creem.io/test" });

      // Mock window.location
      const originalLocation = window.location;
      Object.defineProperty(window, "location", {
        writable: true,
        value: { ...originalLocation, href: "" },
      });

      await userEvent.click(screen.getByText("Upgrade to Pro"));

      await waitFor(() => {
        expect(window.location.href).toBe("https://checkout.creem.io/test");
      });

      Object.defineProperty(window, "location", {
        writable: true,
        value: originalLocation,
      });
    });
  });
});
