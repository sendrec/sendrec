import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Playlists } from "./Playlists";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

function makePlaylist(overrides: Record<string, unknown> = {}) {
  return {
    id: "pl-1",
    title: "Onboarding",
    description: "",
    isShared: false,
    videoCount: 3,
    position: 0,
    createdAt: "2026-02-23T00:00:00Z",
    updatedAt: "2026-02-23T00:00:00Z",
    ...overrides,
  };
}

function renderPlaylists() {
  return render(
    <MemoryRouter>
      <Playlists />
    </MemoryRouter>,
  );
}

describe("Playlists", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows loading state initially", () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}));
    renderPlaylists();
    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("renders empty state when no playlists", async () => {
    mockApiFetch.mockResolvedValueOnce([]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("No playlists yet")).toBeInTheDocument();
    });
    expect(
      screen.getByText(
        "Create a playlist to organize and share collections of videos.",
      ),
    ).toBeInTheDocument();
  });

  it("renders playlist list", async () => {
    mockApiFetch.mockResolvedValueOnce([
      makePlaylist(),
      makePlaylist({
        id: "pl-2",
        title: "Demos",
        videoCount: 5,
        isShared: true,
        position: 1,
      }),
    ]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
      expect(screen.getByText("Demos")).toBeInTheDocument();
      expect(screen.getByText("3 videos")).toBeInTheDocument();
      expect(screen.getByText("5 videos")).toBeInTheDocument();
      expect(screen.getByText("Shared")).toBeInTheDocument();
    });
  });

  it("shows singular video count", async () => {
    mockApiFetch.mockResolvedValueOnce([makePlaylist({ videoCount: 1 })]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("1 video")).toBeInTheDocument();
    });
  });

  it("links playlist titles to detail page", async () => {
    mockApiFetch.mockResolvedValueOnce([makePlaylist()]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
    });

    const link = screen.getByRole("link", { name: /Onboarding/ });
    expect(link).toHaveAttribute("href", "/playlists/pl-1");
  });

  it("shows create form when button clicked", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce([]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("New Playlist")).toBeInTheDocument();
    });

    await user.click(screen.getByText("New Playlist"));
    expect(
      screen.getByPlaceholderText("Playlist title"),
    ).toBeInTheDocument();
  });

  it("hides create form when Cancel clicked", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce([]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("New Playlist")).toBeInTheDocument();
    });

    await user.click(screen.getByText("New Playlist"));
    expect(
      screen.getByPlaceholderText("Playlist title"),
    ).toBeInTheDocument();

    await user.click(screen.getByText("Cancel"));
    expect(
      screen.queryByPlaceholderText("Playlist title"),
    ).not.toBeInTheDocument();
  });

  it("creates playlist on form submit", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce([]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("New Playlist")).toBeInTheDocument();
    });

    await user.click(screen.getByText("New Playlist"));
    await user.type(
      screen.getByPlaceholderText("Playlist title"),
      "My New Playlist",
    );

    // Mock the create call and subsequent refresh
    mockApiFetch.mockResolvedValueOnce(undefined);
    mockApiFetch.mockResolvedValueOnce([
      makePlaylist({ id: "pl-new", title: "My New Playlist", videoCount: 0 }),
    ]);

    await user.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/playlists", {
        method: "POST",
        body: JSON.stringify({ title: "My New Playlist" }),
      });
    });
  });

  it("shows error when create fails", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce([]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("New Playlist")).toBeInTheDocument();
    });

    await user.click(screen.getByText("New Playlist"));
    await user.type(
      screen.getByPlaceholderText("Playlist title"),
      "Bad Playlist",
    );

    mockApiFetch.mockRejectedValueOnce(new Error("Title already exists"));

    await user.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(screen.getByText("Title already exists")).toBeInTheDocument();
    });
  });

  it("shows confirm dialog before deleting a playlist", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce([makePlaylist()]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Delete playlist" }));

    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(screen.getByText("Delete this playlist? Videos will not be deleted.")).toBeInTheDocument();

    await user.click(within(screen.getByRole("alertdialog")).getByText("Cancel"));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    // Should not have called delete API (only initial fetch)
    expect(mockApiFetch).toHaveBeenCalledTimes(1);
  });

  it("deletes playlist when confirmed", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce([makePlaylist()]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
    });

    // Mock delete call and subsequent refresh
    mockApiFetch.mockResolvedValueOnce(undefined);
    mockApiFetch.mockResolvedValueOnce([]);

    await user.click(screen.getByRole("button", { name: "Delete playlist" }));

    const dialog = screen.getByRole("alertdialog");
    await user.click(within(dialog).getByText("Delete"));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/playlists/pl-1", {
        method: "DELETE",
      });
    });
  });
});
