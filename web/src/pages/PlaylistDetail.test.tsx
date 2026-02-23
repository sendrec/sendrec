import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { PlaylistDetail } from "./PlaylistDetail";

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

const mockPlaylist = {
  id: "pl-1",
  title: "Onboarding Videos",
  description: "Welcome sequence",
  isShared: true,
  shareToken: "abc123def456",
  shareUrl: "https://app.sendrec.eu/watch/playlist/abc123def456",
  requireEmail: false,
  position: 0,
  videoCount: 2,
  videos: [
    {
      id: "v1",
      title: "Welcome",
      duration: 120,
      shareToken: "tok1",
      shareUrl: "https://app.sendrec.eu/watch/tok1",
      status: "ready",
      position: 0,
      createdAt: "2026-02-23T00:00:00Z",
    },
    {
      id: "v2",
      title: "Setup Guide",
      duration: 300,
      shareToken: "tok2",
      shareUrl: "https://app.sendrec.eu/watch/tok2",
      status: "ready",
      position: 1,
      createdAt: "2026-02-23T00:00:00Z",
    },
  ],
  createdAt: "2026-02-23T00:00:00Z",
  updatedAt: "2026-02-23T00:00:00Z",
};

function renderDetail() {
  return render(
    <MemoryRouter initialEntries={["/playlists/pl-1"]}>
      <Routes>
        <Route path="/playlists/:id" element={<PlaylistDetail />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("PlaylistDetail", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    mockNavigate.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows loading state initially", () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}));
    renderDetail();
    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("renders playlist title and videos", async () => {
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(screen.getByText("Onboarding Videos")).toBeInTheDocument();
      expect(screen.getByText("Welcome")).toBeInTheDocument();
      expect(screen.getByText("Setup Guide")).toBeInTheDocument();
    });
  });

  it("shows video durations formatted", async () => {
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(screen.getByText("2:00")).toBeInTheDocument();
      expect(screen.getByText("5:00")).toBeInTheDocument();
    });
  });

  it("shows video count", async () => {
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(screen.getByText(/2 videos/)).toBeInTheDocument();
    });
  });

  it("shows description", async () => {
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(screen.getByText("Welcome sequence")).toBeInTheDocument();
    });
  });

  it("shows sharing controls when shared", async () => {
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(
        screen.getByDisplayValue(
          "https://app.sendrec.eu/watch/playlist/abc123def456",
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows sharing toggle as enabled when shared", async () => {
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      const toggle = screen.getAllByText("Enabled");
      expect(toggle.length).toBeGreaterThan(0);
    });
  });

  it("shows empty state when no videos", async () => {
    mockApiFetch.mockResolvedValueOnce({
      ...mockPlaylist,
      videos: [],
      videoCount: 0,
    });
    renderDetail();

    await waitFor(() => {
      expect(
        screen.getByText(/no videos in this playlist/i),
      ).toBeInTheDocument();
    });
  });

  it("shows not found state when playlist not found", async () => {
    mockApiFetch.mockRejectedValueOnce(new Error("Not found"));
    renderDetail();

    await waitFor(() => {
      expect(screen.getByText("Playlist not found")).toBeInTheDocument();
    });
    expect(
      screen.getByRole("link", { name: "Back to Playlists" }),
    ).toHaveAttribute("href", "/playlists");
  });

  it("renders position numbers for videos", async () => {
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(screen.getByText("1")).toBeInTheDocument();
      expect(screen.getByText("2")).toBeInTheDocument();
    });
  });

  it("renders move up and move down buttons", async () => {
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Move Welcome up" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Move Welcome down" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Move Setup Guide up" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Move Setup Guide down" }),
      ).toBeInTheDocument();
    });
  });

  it("renders remove buttons for each video", async () => {
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Remove Welcome" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Remove Setup Guide" }),
      ).toBeInTheDocument();
    });
  });

  it("removes video from playlist when remove clicked", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(screen.getByText("Welcome")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce(undefined);

    await user.click(
      screen.getByRole("button", { name: "Remove Welcome" }),
    );

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        "/api/playlists/pl-1/videos/v1",
        { method: "DELETE" },
      );
    });
    expect(screen.queryByText("Welcome")).not.toBeInTheDocument();
    expect(screen.getByText("Setup Guide")).toBeInTheDocument();
  });

  it("shows Add Videos button", async () => {
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Add Videos" }),
      ).toBeInTheDocument();
    });
  });

  it("opens add videos modal and shows available videos", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Add Videos" }),
      ).toBeInTheDocument();
    });

    // Mock library fetch for add modal
    mockApiFetch.mockResolvedValueOnce([
      {
        id: "v1",
        title: "Welcome",
        duration: 120,
        shareToken: "tok1",
        status: "ready",
        createdAt: "2026-02-23T00:00:00Z",
      },
      {
        id: "v3",
        title: "New Feature Demo",
        duration: 180,
        shareToken: "tok3",
        status: "ready",
        createdAt: "2026-02-23T00:00:00Z",
      },
    ]);

    await user.click(
      screen.getByRole("button", { name: "Add Videos" }),
    );

    await waitFor(() => {
      // v1 is already in playlist, so only v3 should appear
      expect(screen.getByText("New Feature Demo")).toBeInTheDocument();
    });
  });

  it("confirms before deleting playlist", async () => {
    const user = userEvent.setup();
    const confirmSpy = vi
      .spyOn(window, "confirm")
      .mockReturnValue(false);
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(screen.getByText("Onboarding Videos")).toBeInTheDocument();
    });

    await user.click(
      screen.getByRole("button", { name: "Delete playlist" }),
    );

    expect(confirmSpy).toHaveBeenCalledWith(
      "Delete this playlist? Videos will not be deleted.",
    );
  });

  it("deletes playlist and navigates to playlists list", async () => {
    const user = userEvent.setup();
    vi.spyOn(window, "confirm").mockReturnValue(true);
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(screen.getByText("Onboarding Videos")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce(undefined);

    await user.click(
      screen.getByRole("button", { name: "Delete playlist" }),
    );

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/playlists/pl-1", {
        method: "DELETE",
      });
    });
    expect(mockNavigate).toHaveBeenCalledWith("/playlists");
  });

  it("shows back link to playlists", async () => {
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(screen.getByText("Onboarding Videos")).toBeInTheDocument();
    });

    const backLink = screen.getByRole("link", { name: /Playlists/ });
    expect(backLink).toHaveAttribute("href", "/playlists");
  });

  it("shows email gate toggle when sharing is enabled", async () => {
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(screen.getByText("Email gate")).toBeInTheDocument();
    });
  });

  it("hides sharing details when not shared", async () => {
    mockApiFetch.mockResolvedValueOnce({
      ...mockPlaylist,
      isShared: false,
      shareToken: undefined,
      shareUrl: undefined,
    });
    renderDetail();

    await waitFor(() => {
      expect(screen.getByText("Onboarding Videos")).toBeInTheDocument();
    });

    expect(screen.queryByLabelText("Share link")).not.toBeInTheDocument();
    expect(screen.queryByText("Email gate")).not.toBeInTheDocument();
  });

  it("links video titles to video detail pages", async () => {
    mockApiFetch.mockResolvedValueOnce(mockPlaylist);
    renderDetail();

    await waitFor(() => {
      expect(screen.getByText("Welcome")).toBeInTheDocument();
    });

    const welcomeLink = screen.getByRole("link", { name: "Welcome" });
    expect(welcomeLink).toHaveAttribute("href", "/videos/v1");

    const setupLink = screen.getByRole("link", { name: "Setup Guide" });
    expect(setupLink).toHaveAttribute("href", "/videos/v2");
  });
});
