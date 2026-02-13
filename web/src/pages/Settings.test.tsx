import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
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
      .mockResolvedValueOnce({ viewNotification: "off" });
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("alice@example.com")).toBeInTheDocument();
    });
    expect(screen.getByDisplayValue("Alice")).toBeInTheDocument();
  });

  it("disables email field", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ viewNotification: "off" });
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue("alice@example.com")).toBeDisabled();
    });
  });

  it("disables save button when name is unchanged", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ viewNotification: "off" });
    renderSettings();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Save name" })).toBeDisabled();
    });
  });

  it("enables save button when name changes", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ viewNotification: "off" });
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
      .mockResolvedValueOnce({ viewNotification: "off" })
      .mockResolvedValueOnce(undefined);
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
      .mockResolvedValueOnce({ viewNotification: "off" })
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
      .mockResolvedValueOnce({ viewNotification: "off" });
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
      .mockResolvedValueOnce({ viewNotification: "off" })
      .mockResolvedValueOnce(undefined);
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
      .mockResolvedValueOnce({ viewNotification: "off" })
      .mockResolvedValueOnce(undefined);
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
      .mockResolvedValueOnce({ viewNotification: "every" });
    renderSettings();

    await waitFor(() => {
      const select = screen.getByLabelText("View notifications") as HTMLSelectElement;
      expect(select.value).toBe("every");
    });
  });

  it("defaults notification preference to off", async () => {
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ viewNotification: "off" });
    renderSettings();

    await waitFor(() => {
      const select = screen.getByLabelText("View notifications") as HTMLSelectElement;
      expect(select.value).toBe("off");
    });
  });

  it("updates notification preference on change", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ viewNotification: "off" })
      .mockResolvedValueOnce(undefined); // PUT response
    renderSettings();

    await waitFor(() => {
      expect(screen.getByLabelText("View notifications")).toBeInTheDocument();
    });

    await user.selectOptions(screen.getByLabelText("View notifications"), "digest");

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/settings/notifications", {
        method: "PUT",
        body: JSON.stringify({ viewNotification: "digest" }),
      });
    });
  });

  it("shows saved confirmation after notification change", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ name: "Alice", email: "alice@example.com" })
      .mockResolvedValueOnce({ viewNotification: "off" })
      .mockResolvedValueOnce(undefined); // PUT response
    renderSettings();

    await waitFor(() => {
      expect(screen.getByLabelText("View notifications")).toBeInTheDocument();
    });

    await user.selectOptions(screen.getByLabelText("View notifications"), "every");

    await waitFor(() => {
      expect(screen.getByText("Preference saved")).toBeInTheDocument();
    });
  });
});
