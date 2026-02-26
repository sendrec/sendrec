import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Library } from "./Library";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

const unlimitedLimits = { maxVideosPerMonth: 0, maxVideoDurationSeconds: 0, videosUsedThisMonth: 0, brandingEnabled: false };

function makeVideo(overrides: Record<string, unknown> = {}) {
  return {
    id: "v1",
    title: "My Recording",
    status: "ready",
    duration: 125,
    shareToken: "abc123",
    shareUrl: "https://app.sendrec.eu/watch/abc123",
    createdAt: "2026-02-01T10:00:00Z",
    shareExpiresAt: new Date(Date.now() + 5 * 86400000).toISOString(),
    viewCount: 3,
    uniqueViewCount: 2,
    thumbnailUrl: "https://storage.sendrec.eu/thumb.jpg",
    commentMode: "disabled",
    commentCount: 0,
    transcriptStatus: "none",
    viewNotification: null,
    downloadEnabled: true,
    emailGateEnabled: false,
    ctaText: null,
    ctaUrl: null,
    suggestedTitle: null,
    summaryStatus: "none",
    folderId: null,
    tags: [],
    ...overrides,
  };
}

function mockFetch(videos: unknown[], limits = unlimitedLimits, folders: unknown[] = [], tags: unknown[] = [], playlists: unknown[] = []) {
  mockApiFetch
    .mockResolvedValueOnce(videos)
    .mockResolvedValueOnce(limits)
    .mockResolvedValueOnce(folders)
    .mockResolvedValueOnce(tags)
    .mockResolvedValueOnce(playlists);
}

function renderLibrary() {
  return render(
    <MemoryRouter>
      <Library />
    </MemoryRouter>
  );
}

async function openOverflowMenu(user?: ReturnType<typeof userEvent.setup>) {
  const moreButton = screen.getByRole("button", { name: "More actions" });
  if (user) {
    await user.click(moreButton);
  } else {
    await userEvent.click(moreButton);
  }
}

describe("Library", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows skeleton cards during loading", () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}));
    const { container } = renderLibrary();
    expect(container.querySelectorAll(".skeleton-card")).toHaveLength(6);
    expect(container.querySelector(".skeleton-thumb")).not.toBeNull();
    expect(container.querySelector(".skeleton-title")).not.toBeNull();
  });

  it("shows empty state when no videos", async () => {
    mockFetch([]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("No recordings yet.")).toBeInTheDocument();
    });
    expect(screen.getByRole("link", { name: "Record" })).toHaveAttribute("href", "/");
    expect(screen.getByRole("link", { name: "Upload" })).toHaveAttribute("href", "/upload");
  });

  it("renders video list with title and metadata", async () => {
    mockFetch([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("My Recording")).toBeInTheDocument();
    });
    // Duration "2:05" should appear
    expect(screen.getByText(/2:05/)).toBeInTheDocument();
  });

  it("shows view counts", async () => {
    mockFetch([makeVideo({ viewCount: 3, uniqueViewCount: 2 })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText(/3 views \(2 unique\)/)).toBeInTheDocument();
    });
  });

  it("shows 'No views yet' for zero views", async () => {
    mockFetch([makeVideo({ viewCount: 0, uniqueViewCount: 0 })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText(/No views yet/)).toBeInTheDocument();
    });
  });

  it("shows expiry label", async () => {
    mockFetch([
      makeVideo({ shareExpiresAt: new Date(Date.now() + 3 * 86400000).toISOString() }),
    ]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText(/Expires in/)).toBeInTheDocument();
    });
  });

  it("shows expired label for past expiry", async () => {
    mockFetch([
      makeVideo({ shareExpiresAt: new Date(Date.now() - 86400000).toISOString() }),
    ]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText(/Expired/)).toBeInTheDocument();
    });
  });

  it("shows overflow menu when clicking More actions button", async () => {
    mockFetch([makeVideo()]);
    renderLibrary();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });
    expect(screen.queryByText("Download")).not.toBeInTheDocument();

    await openOverflowMenu();

    expect(screen.getByText("Download")).toBeInTheDocument();
    expect(screen.getByText("Analytics")).toBeInTheDocument();
    expect(screen.getByText("Delete")).toBeInTheDocument();
  });

  it("closes overflow menu on Escape", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo()]);
    renderLibrary();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    expect(screen.getByText("Download")).toBeInTheDocument();

    await user.keyboard("{Escape}");
    await waitFor(() => {
      expect(screen.queryByText("Download")).not.toBeInTheDocument();
    });
  });

  it("renders copy link button for ready videos", async () => {
    mockFetch([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Copy link" })).toBeInTheDocument();
    });
  });

  it("shows uploading status", async () => {
    mockFetch([makeVideo({ status: "uploading" })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("uploading...")).toBeInTheDocument();
    });
  });

  it("shows processing status", async () => {
    mockFetch([makeVideo({ status: "processing" })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("processing...")).toBeInTheDocument();
    });
  });

  it("renders thumbnail when available", async () => {
    mockFetch([makeVideo()]);
    const { container } = renderLibrary();

    await waitFor(() => {
      const img = container.querySelector("img");
      expect(img).not.toBeNull();
      expect(img).toHaveAttribute("src", "https://storage.sendrec.eu/thumb.jpg");
    });
  });

  it("does not render thumbnail when unavailable", async () => {
    mockFetch([makeVideo({ thumbnailUrl: undefined })]);
    const { container } = renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("My Recording")).toBeInTheDocument();
    });
    expect(container.querySelector("img")).toBeNull();
  });

  it("confirms before deleting", async () => {
    const user = userEvent.setup();
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    mockFetch([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    await user.click(screen.getByText("Delete"));

    expect(confirmSpy).toHaveBeenCalledWith("Delete this recording? This cannot be undone.");
    // Should not have called delete API (only initial fetches: videos, limits, folders, tags, playlists)
    expect(mockApiFetch).toHaveBeenCalledTimes(5);
  });

  it("deletes video when confirmed", async () => {
    const user = userEvent.setup();
    vi.spyOn(window, "confirm").mockReturnValue(true);
    mockFetch([makeVideo()]);
    mockApiFetch.mockResolvedValueOnce(undefined); // delete response
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    await user.click(screen.getByText("Delete"));

    await waitFor(() => {
      expect(screen.queryByText("My Recording")).not.toBeInTheDocument();
    });
    expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1", { method: "DELETE" });
  });

  it("shows usage indicator when limits are active", async () => {
    mockFetch([makeVideo()], {
      maxVideosPerMonth: 25,
      maxVideoDurationSeconds: 300,
      videosUsedThisMonth: 12,
      brandingEnabled: false,
    });
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText(/12 \/ 25 videos this month/i)).toBeInTheDocument();
    });
  });

  it("shows usage indicator in empty state when limits are active", async () => {
    mockFetch([], {
      maxVideosPerMonth: 25,
      maxVideoDurationSeconds: 300,
      videosUsedThisMonth: 10,
      brandingEnabled: false,
    });
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("No recordings yet.")).toBeInTheDocument();
    });
    expect(screen.getByText(/10 \/ 25 videos this month/i)).toBeInTheDocument();
  });

  it("hides usage indicator when limits are unlimited", async () => {
    mockFetch([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("My Recording")).toBeInTheDocument();
    });
    expect(screen.queryByText(/videos this month/i)).not.toBeInTheDocument();
  });

  it("triggers download API call from overflow menu", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo()]);
    mockApiFetch.mockResolvedValueOnce({ downloadUrl: "https://s3.example.com/download" });
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    await user.click(screen.getByText("Download"));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/download");
    });
  });

  it("renders search input", async () => {
    mockFetch([makeVideo()]);
    renderLibrary();

    await screen.findByText("My Recording");
    expect(screen.getByPlaceholderText("Search videos...")).toBeInTheDocument();
  });

  it("fetches with query param when typing in search", async () => {
    mockFetch([makeVideo()]);
    renderLibrary();

    await screen.findByText("My Recording");
    mockApiFetch.mockClear();

    // Mock the search response
    mockApiFetch
      .mockResolvedValueOnce([makeVideo({ title: "Deploy walkthrough" })])
      .mockResolvedValueOnce(unlimitedLimits);

    const input = screen.getByPlaceholderText("Search videos...");
    await userEvent.type(input, "deploy");

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos?q=deploy");
    });
  });

  it("shows singular view count", async () => {
    mockFetch([makeVideo({ viewCount: 1, uniqueViewCount: 1 })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText(/1 view(?!s)/)).toBeInTheDocument();
    });
  });

  it("shows expires tomorrow label", async () => {
    const tomorrow = new Date(Date.now() + 1 * 86400000).toISOString();
    mockFetch([makeVideo({ shareExpiresAt: tomorrow })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText(/Expires tomorrow/)).toBeInTheDocument();
    });
  });

  it("copies link using clipboard API", async () => {
    const user = userEvent.setup();
    const writeTextSpy = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: writeTextSpy },
      writable: true,
      configurable: true,
    });
    mockFetch([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Copy link" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Copy link" }));

    await waitFor(() => {
      expect(writeTextSpy).toHaveBeenCalledWith("https://app.sendrec.eu/watch/abc123");
    });
  });

  it("shows toast after copying link", async () => {
    const user = userEvent.setup();
    const writeTextSpy = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: writeTextSpy },
      writable: true,
      configurable: true,
    });
    mockFetch([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Copy link" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Copy link" }));

    await waitFor(() => {
      expect(screen.getByText("Link copied")).toBeInTheDocument();
    });
  });

  it("shows Never expires when shareExpiresAt is null", async () => {
    mockFetch([makeVideo({ shareExpiresAt: null })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText(/Never expires/)).toBeInTheDocument();
    });
  });

  it("polls for video status when processing and updates when ready", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockFetch([makeVideo({ status: "processing" })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("processing...")).toBeInTheDocument();
    });

    // First poll: still processing
    mockApiFetch.mockResolvedValueOnce([makeVideo({ status: "processing" })]);
    mockApiFetch.mockResolvedValueOnce(unlimitedLimits);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000);
    });

    await waitFor(() => {
      expect(screen.getByText("processing...")).toBeInTheDocument();
    });

    // Second poll: ready
    mockApiFetch.mockResolvedValueOnce([makeVideo({ status: "ready" })]);
    mockApiFetch.mockResolvedValueOnce(unlimitedLimits);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000);
    });

    await waitFor(() => {
      expect(screen.queryByText("processing...")).not.toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "Copy link" })).toBeInTheDocument();

    vi.useRealTimers();
  });

  describe("simplified card layout", () => {
    it("title is a Link to video detail page", async () => {
      mockFetch([makeVideo()]);
      renderLibrary();

      await waitFor(() => {
        const titleLink = screen.getByRole("link", { name: "My Recording" });
        expect(titleLink).toHaveAttribute("href", "/videos/v1");
      });
    });

    it("title is not inline editable", async () => {
      const user = userEvent.setup();
      mockFetch([makeVideo()]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("My Recording")).toBeInTheDocument();
      });

      await user.click(screen.getByText("My Recording"));

      // Should NOT show an input for editing
      expect(screen.queryByDisplayValue("My Recording")).not.toBeInTheDocument();
    });

    it("thumbnail links to video detail page", async () => {
      mockFetch([makeVideo()]);
      const { container } = renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("My Recording")).toBeInTheDocument();
      });

      const thumbnailLink = container.querySelector("a[href='/videos/v1']");
      expect(thumbnailLink).not.toBeNull();
      const img = thumbnailLink!.querySelector("img");
      expect(img).not.toBeNull();
    });

    it("action row has only Copy link and overflow menu", async () => {
      mockFetch([makeVideo()]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "Copy link" })).toBeInTheDocument();
      });

      // These should NOT be inline actions anymore
      expect(screen.queryByRole("link", { name: "View" })).not.toBeInTheDocument();
      expect(screen.queryByRole("link", { name: "Analytics" })).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Download" })).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();

      // Overflow menu button should be present
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    it("overflow menu has only Analytics, Download, and Delete", async () => {
      mockFetch([makeVideo()]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
      });

      await openOverflowMenu();

      // Should have these items
      expect(screen.getByText("Analytics")).toBeInTheDocument();
      expect(screen.getByText("Download")).toBeInTheDocument();
      expect(screen.getByText("Delete")).toBeInTheDocument();

      // Should NOT have these old overflow menu items
      expect(screen.queryByText("Organization")).not.toBeInTheDocument();
      expect(screen.queryByText("Sharing")).not.toBeInTheDocument();
      expect(screen.queryByText("Customization")).not.toBeInTheDocument();
      expect(screen.queryByText("Editing")).not.toBeInTheDocument();
      expect(screen.queryByText("Trim")).not.toBeInTheDocument();
      expect(screen.queryByText("Remove fillers")).not.toBeInTheDocument();
      expect(screen.queryByText("Downloads on")).not.toBeInTheDocument();
      expect(screen.queryByText("Require email")).not.toBeInTheDocument();
      expect(screen.queryByText("Embed")).not.toBeInTheDocument();
      expect(screen.queryByText("Branding")).not.toBeInTheDocument();
      expect(screen.queryByText("Thumbnail")).not.toBeInTheDocument();
      expect(screen.queryByLabelText("Comment mode")).not.toBeInTheDocument();
      expect(screen.queryByLabelText("View notifications")).not.toBeInTheDocument();
      expect(screen.queryByLabelText("Move to folder")).not.toBeInTheDocument();
    });

    it("does not show transcript/summary actions on card", async () => {
      mockFetch([makeVideo({ transcriptStatus: "ready", summaryStatus: "ready" })]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("My Recording")).toBeInTheDocument();
      });

      expect(screen.queryByText("Redo transcript")).not.toBeInTheDocument();
      expect(screen.queryByText("Transcribe")).not.toBeInTheDocument();
      expect(screen.queryByText("Re-summarize")).not.toBeInTheDocument();
      expect(screen.queryByText("Summarize")).not.toBeInTheDocument();
      expect(screen.queryByText("Transcribing...")).not.toBeInTheDocument();
    });

    it("does not show suggested title on card", async () => {
      mockFetch([makeVideo({ suggestedTitle: "Product Demo for Q1" })]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("My Recording")).toBeInTheDocument();
      });

      expect(screen.queryByText(/Suggested:/)).not.toBeInTheDocument();
      expect(screen.queryByText("Accept")).not.toBeInTheDocument();
    });

    it("analytics link in overflow menu points to correct URL", async () => {
      mockFetch([makeVideo()]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
      });

      await openOverflowMenu();

      const analyticsLink = screen.getByRole("link", { name: "Analytics" });
      expect(analyticsLink).toHaveAttribute("href", "/videos/v1/analytics");
    });
  });

  describe("onboarding empty state", () => {
    it("shows getting started steps when library is empty", async () => {
      mockFetch([]);
      renderLibrary();

      expect(await screen.findByText(/get started in 3 steps/i)).toBeInTheDocument();
      expect(screen.getByText(/record your screen/i)).toBeInTheDocument();
      expect(screen.getByText(/share the link/i)).toBeInTheDocument();
      expect(screen.getByText(/track views/i)).toBeInTheDocument();
    });

    it("shows both Record and Upload buttons when library is empty", async () => {
      mockFetch([]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("No recordings yet.")).toBeInTheDocument();
      });
      expect(screen.getByRole("link", { name: "Record" })).toHaveAttribute("href", "/");
      expect(screen.getByRole("link", { name: "Upload" })).toHaveAttribute("href", "/upload");
    });
  });

  it("renders playlists in sidebar", async () => {
    const playlists = [{ id: "p1", title: "Demo Reel", videoCount: 3 }];
    mockFetch([makeVideo()], unlimitedLimits, [], [], playlists);
    renderLibrary();
    await waitFor(() => {
      expect(screen.getByText("Playlists")).toBeInTheDocument();
    });
    const link = screen.getByRole("link", { name: /Demo Reel/ });
    expect(link).toHaveAttribute("href", "/playlists/p1");
  });

  describe("batch operations", () => {
    it("shows checkbox on video cards", async () => {
      mockFetch([makeVideo()]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("My Recording")).toBeInTheDocument();
      });

      expect(screen.getByRole("checkbox", { name: "Select My Recording" })).toBeInTheDocument();
    });

    it("shows batch toolbar when videos are selected", async () => {
      const user = userEvent.setup();
      mockFetch([makeVideo()]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("My Recording")).toBeInTheDocument();
      });

      await user.click(screen.getByRole("checkbox", { name: "Select My Recording" }));

      expect(screen.getByText("1 selected")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Select all" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Deselect all" })).toBeInTheDocument();
    });

    it("batch delete calls API and refreshes", async () => {
      const user = userEvent.setup();
      vi.spyOn(window, "confirm").mockReturnValue(true);
      mockFetch([makeVideo(), makeVideo({ id: "v2", title: "Second Recording" })]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("My Recording")).toBeInTheDocument();
      });

      await user.click(screen.getByRole("checkbox", { name: "Select My Recording" }));
      await user.click(screen.getByRole("checkbox", { name: "Select Second Recording" }));

      expect(screen.getByText("2 selected")).toBeInTheDocument();

      // Mock batch delete response + subsequent refresh (videos, limits, folders, tags)
      mockApiFetch.mockResolvedValueOnce({ deleted: 2 });
      mockApiFetch.mockResolvedValueOnce([]);
      mockApiFetch.mockResolvedValueOnce(unlimitedLimits);
      mockApiFetch.mockResolvedValueOnce([]);
      mockApiFetch.mockResolvedValueOnce([]);

      await user.click(screen.getByRole("button", { name: "Delete" }));

      await waitFor(() => {
        expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/batch/delete", {
          method: "POST",
          body: JSON.stringify({ videoIds: ["v1", "v2"] }),
        });
      });
    });

    it("deselect all clears selection", async () => {
      const user = userEvent.setup();
      mockFetch([makeVideo()]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("My Recording")).toBeInTheDocument();
      });

      await user.click(screen.getByRole("checkbox", { name: "Select My Recording" }));
      expect(screen.getByText("1 selected")).toBeInTheDocument();

      await user.click(screen.getByRole("button", { name: "Deselect all" }));

      expect(screen.queryByText("1 selected")).not.toBeInTheDocument();
    });

    it("select all selects all visible videos", async () => {
      const user = userEvent.setup();
      mockFetch([makeVideo(), makeVideo({ id: "v2", title: "Second Recording" })]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("My Recording")).toBeInTheDocument();
      });

      // Click one checkbox to show toolbar
      await user.click(screen.getByRole("checkbox", { name: "Select My Recording" }));
      expect(screen.getByText("1 selected")).toBeInTheDocument();

      await user.click(screen.getByRole("button", { name: "Select all" }));
      expect(screen.getByText("2 selected")).toBeInTheDocument();

      const checkboxes = screen.getAllByRole("checkbox");
      for (const cb of checkboxes) {
        expect(cb).toBeChecked();
      }
    });
  });

  describe("grid view and controls", () => {
    it("renders videos in grid layout by default", async () => {
      mockFetch([makeVideo()]);
      const { container } = renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("My Recording")).toBeInTheDocument();
      });

      expect(container.querySelector(".video-grid")).not.toBeNull();
      expect(container.querySelector(".video-list")).toBeNull();
    });

    it("switches to list view when list toggle is clicked", async () => {
      const user = userEvent.setup();
      mockFetch([makeVideo()]);
      const { container } = renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("My Recording")).toBeInTheDocument();
      });

      await user.click(screen.getByRole("button", { name: "List view" }));

      expect(container.querySelector(".video-list")).not.toBeNull();
      expect(container.querySelector(".video-grid")).toBeNull();
    });

    it("renders sort dropdown with options", async () => {
      mockFetch([makeVideo()]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("My Recording")).toBeInTheDocument();
      });

      const sortSelect = screen.getByRole("combobox", { name: "Sort videos" });
      expect(sortSelect).toBeInTheDocument();
      expect(screen.getByText("Newest first")).toBeInTheDocument();
      expect(screen.getByText("Oldest first")).toBeInTheDocument();
      expect(screen.getByText("Most viewed")).toBeInTheDocument();
      expect(screen.getByText("Title A-Z")).toBeInTheDocument();
    });

    it("renders duration badge on thumbnail", async () => {
      mockFetch([makeVideo()]);
      const { container } = renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("My Recording")).toBeInTheDocument();
      });

      const badge = container.querySelector(".video-card-duration");
      expect(badge).not.toBeNull();
      expect(badge!.textContent).toBe("2:05");
    });

    it("renders play overlay on thumbnail", async () => {
      mockFetch([makeVideo()]);
      const { container } = renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("My Recording")).toBeInTheDocument();
      });

      expect(container.querySelector(".video-card-play")).not.toBeNull();
    });
  });
});
