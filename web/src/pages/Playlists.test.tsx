import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Playlists } from "./Playlists";

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

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

const defaultLimits = { maxPlaylists: 0, playlistsUsed: 0 };

function mockFetch(
  playlists: unknown[],
  limits: Record<string, unknown> = defaultLimits,
) {
  mockApiFetch.mockResolvedValueOnce(playlists).mockResolvedValueOnce(limits);
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
    mockNavigate.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows skeleton cards during loading", () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}));
    const { container } = renderPlaylists();
    expect(container.querySelectorAll(".skeleton-card")).toHaveLength(3);
  });

  it("renders empty state with CTA button", async () => {
    mockFetch([]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("No playlists yet")).toBeInTheDocument();
    });
    expect(
      screen.getByText(
        "Create a playlist to organize and share collections of videos.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Create your first playlist" }),
    ).toBeInTheDocument();
  });

  it("renders playlists in grid layout", async () => {
    mockFetch([makePlaylist()]);
    const { container } = renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
    });
    expect(container.querySelector(".video-grid")).toBeInTheDocument();
  });

  it("renders stacked thumbnail on card", async () => {
    mockFetch([makePlaylist()]);
    const { container } = renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
    });
    expect(container.querySelector(".playlist-thumb-stack")).toBeInTheDocument();
    expect(
      container.querySelector(".playlist-thumb--front"),
    ).toBeInTheDocument();
  });

  it("renders playlist list", async () => {
    mockFetch([
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
      expect(screen.getAllByText("3 videos")).toHaveLength(2); // thumb count + meta
      expect(screen.getAllByText("5 videos")).toHaveLength(2);
      expect(screen.getByText("Shared")).toBeInTheDocument();
    });
  });

  it("shows singular video count", async () => {
    mockFetch([makePlaylist({ videoCount: 1 })]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getAllByText("1 video")).toHaveLength(2); // thumb count + meta
    });
  });

  it("shows Private badge for non-shared playlist", async () => {
    mockFetch([makePlaylist({ isShared: false })]);
    const { container } = renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Private")).toBeInTheDocument();
    });
    expect(
      container.querySelector(".playlist-card-badge--private"),
    ).toBeInTheDocument();
  });

  it("shows Shared badge for shared playlist", async () => {
    mockFetch([makePlaylist({ isShared: true })]);
    const { container } = renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Shared")).toBeInTheDocument();
    });
    expect(
      container.querySelector(".playlist-card-badge--shared"),
    ).toBeInTheDocument();
  });

  it("shows creation date on card", async () => {
    mockFetch([makePlaylist({ createdAt: "2026-02-23T00:00:00Z" })]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Created 23/02/2026")).toBeInTheDocument();
    });
  });

  it("links playlist titles to detail page", async () => {
    mockFetch([makePlaylist()]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
    });

    const link = screen.getByRole("link", { name: /Onboarding/ });
    expect(link).toHaveAttribute("href", "/playlists/pl-1");
  });

  it("navigates to detail on card click", async () => {
    const user = userEvent.setup();
    mockFetch([makePlaylist()]);
    const { container } = renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
    });

    const card = container.querySelector(".playlist-card")!;
    await user.click(card);
    expect(mockNavigate).toHaveBeenCalledWith("/playlists/pl-1");
  });

  it("shows context menu when menu button clicked", async () => {
    const user = userEvent.setup();
    mockFetch([makePlaylist()]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
    });

    await user.click(screen.getByLabelText("Playlist options"));
    expect(screen.getByText("Edit")).toBeInTheDocument();
    expect(screen.getByText("Delete")).toBeInTheDocument();
  });

  it("closes context menu on Escape", async () => {
    const user = userEvent.setup();
    mockFetch([makePlaylist()]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
    });

    await user.click(screen.getByLabelText("Playlist options"));
    expect(screen.getByText("Edit")).toBeInTheDocument();

    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => {
      expect(screen.queryByText("Edit")).not.toBeInTheDocument();
    });
  });

  it("shows usage bar when limits active", async () => {
    mockFetch(
      [makePlaylist()],
      { maxPlaylists: 3, playlistsUsed: 2 },
    );
    const { container } = renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
    });
    expect(screen.getByText("of 3 playlists used")).toBeInTheDocument();
    expect(container.querySelector(".playlist-usage")).toBeInTheDocument();
  });

  it("hides usage bar when unlimited", async () => {
    mockFetch(
      [makePlaylist()],
      { maxPlaylists: 0, playlistsUsed: 0 },
    );
    const { container } = renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
    });
    expect(container.querySelector(".playlist-usage")).not.toBeInTheDocument();
  });

  it("shows warning usage bar when at limit", async () => {
    mockFetch(
      [makePlaylist()],
      { maxPlaylists: 3, playlistsUsed: 3 },
    );
    const { container } = renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
    });
    expect(
      container.querySelector(".usage-bar-fill--warning"),
    ).toBeInTheDocument();
  });

  it("shows create form when button clicked", async () => {
    const user = userEvent.setup();
    mockFetch([]);
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
    mockFetch([]);
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
    mockFetch([]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("New Playlist")).toBeInTheDocument();
    });

    await user.click(screen.getByText("New Playlist"));
    await user.type(
      screen.getByPlaceholderText("Playlist title"),
      "My New Playlist",
    );

    // Mock the create call and subsequent refresh (playlists + limits)
    mockApiFetch.mockResolvedValueOnce(undefined);
    mockFetch([
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
    mockFetch([]);
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
    mockFetch([makePlaylist()]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
    });

    // Open context menu, then click Delete
    await user.click(screen.getByLabelText("Playlist options"));
    await user.click(screen.getByText("Delete"));

    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(screen.getByText("Delete this playlist? Videos will not be deleted.")).toBeInTheDocument();

    await user.click(within(screen.getByRole("alertdialog")).getByText("Cancel"));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    // Should not have called delete API (only initial playlists + limits fetch)
    expect(mockApiFetch).toHaveBeenCalledTimes(2);
  });

  it("deletes playlist when confirmed", async () => {
    const user = userEvent.setup();
    mockFetch([makePlaylist()]);
    renderPlaylists();

    await waitFor(() => {
      expect(screen.getByText("Onboarding")).toBeInTheDocument();
    });

    // Mock delete call and subsequent refresh (playlists + limits)
    mockApiFetch.mockResolvedValueOnce(undefined);
    mockFetch([]);

    // Open context menu, then click Delete
    await user.click(screen.getByLabelText("Playlist options"));
    await user.click(screen.getByText("Delete"));

    const dialog = screen.getByRole("alertdialog");
    await user.click(within(dialog).getByText("Delete"));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/playlists/pl-1", {
        method: "DELETE",
      });
    });
  });
});
