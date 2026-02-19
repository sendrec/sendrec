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
    ...overrides,
  };
}

function mockFetch(videos: unknown[], limits = unlimitedLimits) {
  mockApiFetch
    .mockResolvedValueOnce(videos)
    .mockResolvedValueOnce(limits);
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

  it("shows loading state initially", () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}));
    renderLibrary();
    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("shows empty state when no videos", async () => {
    mockFetch([]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("No recordings yet.")).toBeInTheDocument();
    });
    expect(screen.getByRole("link", { name: "Create your first recording" })).toHaveAttribute("href", "/");
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
    expect(screen.queryByText("Trim")).not.toBeInTheDocument();

    await openOverflowMenu();

    expect(screen.getByText("Trim")).toBeInTheDocument();
    expect(screen.getByText("Downloads on")).toBeInTheDocument();
  });

  it("closes overflow menu on Escape", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo()]);
    renderLibrary();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    expect(screen.getByText("Trim")).toBeInTheDocument();

    await user.keyboard("{Escape}");
    await waitFor(() => {
      expect(screen.queryByText("Trim")).not.toBeInTheDocument();
    });
  });

  it("renders copy link and delete buttons for ready videos", async () => {
    mockFetch([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Copy link" })).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "Delete" })).toBeInTheDocument();
    await openOverflowMenu();
    expect(screen.getByRole("button", { name: "Extend" })).toBeInTheDocument();
  });

  it("shows uploading status", async () => {
    mockFetch([makeVideo({ status: "uploading" })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("uploading...")).toBeInTheDocument();
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
      expect(screen.getByRole("button", { name: "Delete" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Delete" }));
    expect(confirmSpy).toHaveBeenCalledWith("Delete this recording? This cannot be undone.");
    // Should not have called delete API (only initial fetch + limits fetch)
    expect(mockApiFetch).toHaveBeenCalledTimes(2);
  });

  it("deletes video when confirmed", async () => {
    const user = userEvent.setup();
    vi.spyOn(window, "confirm").mockReturnValue(true);
    mockFetch([makeVideo()]);
    mockApiFetch.mockResolvedValueOnce(undefined); // delete response
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Delete" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Delete" }));

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

  it("renders download button for ready videos", async () => {
    mockFetch([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Download" })).toBeInTheDocument();
    });
  });

  it("triggers download API call on click", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo()]);
    mockApiFetch.mockResolvedValueOnce({ downloadUrl: "https://s3.example.com/download" });
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Download" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Download" }));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/download");
    });
  });

  it("shows 'Remove password' button when hasPassword is true", async () => {
    mockFetch([makeVideo({ hasPassword: true })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });
    await openOverflowMenu();
    expect(screen.getByRole("button", { name: "Remove password" })).toBeInTheDocument();
  });

  it("shows 'Add password' button when hasPassword is false", async () => {
    mockFetch([makeVideo({ hasPassword: false })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });
    await openOverflowMenu();
    expect(screen.getByRole("button", { name: "Add password" })).toBeInTheDocument();
    expect(screen.queryByText("Password protected")).not.toBeInTheDocument();
  });

  it("sets password via prompt and API call", async () => {
    const user = userEvent.setup();
    vi.spyOn(window, "prompt").mockReturnValue("mysecret");
    mockFetch([makeVideo({ hasPassword: false })]);
    mockApiFetch.mockResolvedValueOnce(undefined); // PUT password response
    mockApiFetch.mockResolvedValueOnce([makeVideo({ hasPassword: true })]); // refetch videos
    mockApiFetch.mockResolvedValueOnce(unlimitedLimits); // refetch limits
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    await user.click(screen.getByRole("button", { name: "Add password" }));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/password", {
        method: "PUT",
        body: JSON.stringify({ password: "mysecret" }),
      });
    });
  });

  it("removes password via API call", async () => {
    const user = userEvent.setup();
    vi.spyOn(window, "confirm").mockReturnValue(true);
    mockFetch([makeVideo({ hasPassword: true })]);
    mockApiFetch.mockResolvedValueOnce(undefined); // PUT password response
    mockApiFetch.mockResolvedValueOnce([makeVideo({ hasPassword: false })]); // refetch videos
    mockApiFetch.mockResolvedValueOnce(unlimitedLimits); // refetch limits
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    await user.click(screen.getByRole("button", { name: "Remove password" }));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/password", {
        method: "PUT",
        body: JSON.stringify({ password: "" }),
      });
    });
  });

  it("does not set password when prompt is cancelled", async () => {
    const user = userEvent.setup();
    vi.spyOn(window, "prompt").mockReturnValue(null);
    mockFetch([makeVideo({ hasPassword: false })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    await user.click(screen.getByRole("button", { name: "Add password" }));

    // Should not have called password API (only initial fetch + limits fetch)
    expect(mockApiFetch).toHaveBeenCalledTimes(2);
  });

  it("shows processing status", async () => {
    mockFetch([makeVideo({ status: "processing" })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("processing...")).toBeInTheDocument();
    });
  });

  it("enters edit mode on title click", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("My Recording")).toBeInTheDocument();
    });

    await user.click(screen.getByText("My Recording"));

    const input = screen.getByDisplayValue("My Recording");
    expect(input).toBeInTheDocument();
    expect(input.tagName).toBe("INPUT");
  });

  it("saves title on Enter", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo()]);
    mockApiFetch.mockResolvedValueOnce(undefined); // PATCH response
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("My Recording")).toBeInTheDocument();
    });

    await user.click(screen.getByText("My Recording"));
    const input = screen.getByDisplayValue("My Recording");
    await user.clear(input);
    await user.type(input, "New Title{Enter}");

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1", {
        method: "PATCH",
        body: JSON.stringify({ title: "New Title" }),
      });
    });
    expect(screen.getByText("New Title")).toBeInTheDocument();
  });

  it("cancels edit on Escape", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("My Recording")).toBeInTheDocument();
    });

    await user.click(screen.getByText("My Recording"));
    const input = screen.getByDisplayValue("My Recording");
    await user.clear(input);
    await user.type(input, "Something else{Escape}");

    expect(screen.getByText("My Recording")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("Something else")).not.toBeInTheDocument();
    // Should not have called PATCH (only initial fetch + limits fetch)
    expect(mockApiFetch).toHaveBeenCalledTimes(2);
  });

  it("renders Trim button for ready videos", async () => {
    mockFetch([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });
    await openOverflowMenu();
    expect(screen.getByRole("button", { name: "Trim" })).toBeInTheDocument();
  });

  it("opens trim modal and updates status to processing after trim", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo()]);
    mockApiFetch.mockResolvedValueOnce({ downloadUrl: "https://s3.example.com/video.webm" }); // download URL for TrimModal
    mockApiFetch.mockResolvedValueOnce(undefined); // trim API response
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    await user.click(screen.getByRole("button", { name: "Trim" }));

    // TrimModal should be visible
    expect(screen.getByText("Trim Video")).toBeInTheDocument();

    // The only Trim button now is in the modal (overflow menu closed)
    await user.click(screen.getByRole("button", { name: "Trim" }));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/trim", {
        method: "POST",
        body: expect.any(String),
      });
    });

    // After trim starts, modal should close and status should show processing
    await waitFor(() => {
      expect(screen.getByText("processing...")).toBeInTheDocument();
    });
  });

  it("polls for video status after trim and updates when ready", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    mockFetch([makeVideo()]);
    mockApiFetch.mockResolvedValueOnce({ downloadUrl: "https://s3.example.com/video.webm" }); // download URL for TrimModal
    mockApiFetch.mockResolvedValueOnce(undefined); // trim API response
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    await user.click(screen.getByRole("button", { name: "Trim" }));
    expect(screen.getByText("Trim Video")).toBeInTheDocument();

    // The only Trim button now is in the modal (overflow menu closed)
    await user.click(screen.getByRole("button", { name: "Trim" }));

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

  it("shows comment mode dropdown with current value", async () => {
    mockFetch([makeVideo({ commentMode: "anonymous", commentCount: 5 })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });
    await openOverflowMenu();
    const select = screen.getByLabelText("Comment mode") as HTMLSelectElement;
    expect(select.value).toBe("anonymous");
    expect(screen.getByText(/\(5\)/)).toBeInTheDocument();
  });

  it("shows comment mode dropdown with disabled value", async () => {
    mockFetch([makeVideo({ commentMode: "disabled", commentCount: 0 })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });
    await openOverflowMenu();
    const select = screen.getByLabelText("Comment mode") as HTMLSelectElement;
    expect(select.value).toBe("disabled");
  });

  it("changes comment mode on select", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo({ commentMode: "disabled" })]);
    mockApiFetch.mockResolvedValueOnce(undefined); // PUT comment-mode response
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    await user.selectOptions(screen.getByLabelText("Comment mode"), "name_required");

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/comment-mode", {
        method: "PUT",
        body: JSON.stringify({ commentMode: "name_required" }),
      });
    });

    // Menu stays open, select shows updated value
    const select = screen.getByLabelText("Comment mode") as HTMLSelectElement;
    expect(select.value).toBe("name_required");
  });

  it("links thumbnail to watch page", async () => {
    mockFetch([makeVideo()]);
    const { container } = renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("My Recording")).toBeInTheDocument();
    });

    const thumbnailLink = container.querySelector("a[href='/watch/abc123']");
    expect(thumbnailLink).not.toBeNull();
    const img = thumbnailLink!.querySelector("img");
    expect(img).not.toBeNull();
  });

  it("shows View button linking to watch page", async () => {
    mockFetch([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      const viewLink = screen.getByRole("link", { name: "View" });
      expect(viewLink).toHaveAttribute("href", "/watch/abc123");
    });
  });

  it("saves title on blur", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo()]);
    mockApiFetch.mockResolvedValueOnce(undefined); // PATCH response
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("My Recording")).toBeInTheDocument();
    });

    await user.click(screen.getByText("My Recording"));
    const input = screen.getByDisplayValue("My Recording");
    await user.clear(input);
    await user.type(input, "Blurred Title");
    await user.tab(); // triggers blur

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1", {
        method: "PATCH",
        body: JSON.stringify({ title: "Blurred Title" }),
      });
    });
  });

  it("shows Transcribing badge when transcriptStatus is processing", async () => {
    mockFetch([makeVideo({ transcriptStatus: "processing" })]);
    renderLibrary();

    expect(await screen.findByText(/Transcribing/)).toBeInTheDocument();
  });

  it("shows Pending transcription badge when transcriptStatus is pending", async () => {
    mockFetch([makeVideo({ transcriptStatus: "pending" })]);
    renderLibrary();

    expect(await screen.findByText(/Pending transcription/)).toBeInTheDocument();
  });

  it("hides transcript buttons when transcriptStatus is pending", async () => {
    mockFetch([makeVideo({ transcriptStatus: "pending" })]);
    renderLibrary();

    await screen.findByText(/Pending transcription/);
    expect(screen.queryByText("Transcribe")).not.toBeInTheDocument();
    expect(screen.queryByText("Retry transcript")).not.toBeInTheDocument();
    expect(screen.queryByText("Redo transcript")).not.toBeInTheDocument();
  });

  it("shows Retry transcript button when transcriptStatus is failed", async () => {
    mockFetch([makeVideo({ transcriptStatus: "failed" })]);
    renderLibrary();

    expect(await screen.findByText("Retry transcript")).toBeInTheDocument();
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

  it("extends video share link", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo()]);
    mockApiFetch.mockResolvedValueOnce(undefined); // POST extend response
    mockApiFetch.mockResolvedValueOnce([makeVideo()]); // refetch videos
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    await user.click(screen.getByRole("button", { name: "Extend" }));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/extend", { method: "POST" });
    });
  });

  it("shows Extending... while extending", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo()]);

    let resolveExtend: () => void;
    mockApiFetch.mockReturnValueOnce(
      new Promise<void>((resolve) => {
        resolveExtend = resolve;
      })
    );

    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    await user.click(screen.getByRole("button", { name: "Extend" }));

    // Menu closed after click, reopen to see Extending... state
    await openOverflowMenu(user);
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Extending..." })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Extending..." })).toBeDisabled();
    });

    // Resolve extend + refetch
    mockApiFetch.mockResolvedValueOnce([makeVideo()]);
    resolveExtend!();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Extend" })).toBeInTheDocument();
    });
  });

  it("retranscribes video", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo({ transcriptStatus: "none" })]);
    mockApiFetch.mockResolvedValueOnce(undefined); // POST retranscribe response
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("Transcribe")).toBeInTheDocument();
    });

    await user.click(screen.getByText("Transcribe"));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/retranscribe", { method: "POST" });
    });

    // Status should update to pending
    await waitFor(() => {
      expect(screen.getByText(/Pending transcription/)).toBeInTheDocument();
    });
  });

  it("shows Redo transcript for ready transcript", async () => {
    mockFetch([makeVideo({ transcriptStatus: "ready" })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("Redo transcript")).toBeInTheDocument();
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

  it("copies embed code when clicking Embed button", async () => {
    const user = userEvent.setup();
    const writeTextMock = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: writeTextMock },
      writable: true,
      configurable: true,
    });

    mockFetch([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });
    await openOverflowMenu(user);

    const embedButton = screen.getByRole("button", { name: "Embed" });
    await user.click(embedButton);

    expect(writeTextMock).toHaveBeenCalledWith(
      expect.stringContaining("<iframe")
    );
    expect(writeTextMock).toHaveBeenCalledWith(
      expect.stringContaining("/embed/")
    );

    expect(screen.getByText("Embed code copied")).toBeInTheDocument();
  });

  it("shows analytics link for ready videos", async () => {
    mockFetch([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      const analyticsLink = screen.getByRole("link", { name: "Analytics" });
      expect(analyticsLink).toHaveAttribute("href", "/videos/v1/analytics");
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

  it("shows notification dropdown with account default for null", async () => {
    mockFetch([makeVideo({ viewNotification: null })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });
    await openOverflowMenu();
    const select = screen.getByLabelText("View notifications") as HTMLSelectElement;
    expect(select.value).toBe("");
  });

  it("shows notification dropdown with video override value", async () => {
    mockFetch([makeVideo({ viewNotification: "every" })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });
    await openOverflowMenu();
    const select = screen.getByLabelText("View notifications") as HTMLSelectElement;
    expect(select.value).toBe("every");
  });

  it("changes notification preference on select", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo({ viewNotification: null })]);
    mockApiFetch.mockResolvedValueOnce(undefined); // PUT response
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    await user.selectOptions(screen.getByLabelText("View notifications"), "every");

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/notifications", {
        method: "PUT",
        body: JSON.stringify({ viewNotification: "every" }),
      });
    });
  });

  it("clears notification override when selecting account default", async () => {
    const user = userEvent.setup();
    mockFetch([makeVideo({ viewNotification: "every" })]);
    mockApiFetch.mockResolvedValueOnce(undefined); // PUT response
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });

    await openOverflowMenu(user);
    await user.selectOptions(screen.getByLabelText("View notifications"), "");

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/notifications", {
        method: "PUT",
        body: JSON.stringify({ viewNotification: null }),
      });
    });
  });

  it("shows Thumbnail action button for ready videos", async () => {
    mockFetch([makeVideo()]);
    renderLibrary();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });
    await openOverflowMenu();
    expect(screen.getByText("Thumbnail")).toBeInTheDocument();
  });

  it("shows Reset thumbnail button when video has a thumbnail", async () => {
    mockFetch([makeVideo({ thumbnailUrl: "https://storage.sendrec.eu/thumb.jpg" })]);
    renderLibrary();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });
    await openOverflowMenu();
    expect(screen.getByText("Reset thumbnail")).toBeInTheDocument();
  });

  it("does not show Reset thumbnail when no thumbnail exists", async () => {
    mockFetch([makeVideo({ thumbnailUrl: undefined })]);
    renderLibrary();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
    });
    await openOverflowMenu();
    expect(screen.getByText("Thumbnail")).toBeInTheDocument();
    expect(screen.queryByText("Reset thumbnail")).not.toBeInTheDocument();
  });

  describe("branding", () => {
    it("shows branding action when enabled", async () => {
      mockFetch([makeVideo()], { ...unlimitedLimits, brandingEnabled: true });
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
      });
      await openOverflowMenu();
      expect(screen.getByText("Branding")).toBeInTheDocument();
    });

    it("hides branding action when disabled", async () => {
      mockFetch([makeVideo()]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByText("View")).toBeInTheDocument();
      });
      await openOverflowMenu();
      expect(screen.queryByText("Branding")).not.toBeInTheDocument();
    });
  });

  describe("download toggle", () => {
    it("shows Downloads on when enabled", async () => {
      mockFetch([makeVideo({ downloadEnabled: true })]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
      });
      await openOverflowMenu();
      expect(screen.getByText("Downloads on")).toBeInTheDocument();
    });

    it("shows Downloads off when disabled", async () => {
      mockFetch([makeVideo({ downloadEnabled: false })]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
      });
      await openOverflowMenu();
      expect(screen.getByText("Downloads off")).toBeInTheDocument();
    });

    it("calls API to toggle download off", async () => {
      mockFetch([makeVideo({ downloadEnabled: true })]);
      mockApiFetch.mockResolvedValueOnce(undefined);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
      });

      await openOverflowMenu();
      await userEvent.click(screen.getByText("Downloads on"));

      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/download-enabled", {
        method: "PUT",
        body: JSON.stringify({ downloadEnabled: false }),
      });
    });
  });

  describe("link expiry toggle", () => {
    it("shows Never expires when shareExpiresAt is null", async () => {
      mockFetch([makeVideo({ shareExpiresAt: null })]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByText(/Never expires/)).toBeInTheDocument();
      });
    });

    it("hides Extend button when shareExpiresAt is null", async () => {
      mockFetch([makeVideo({ shareExpiresAt: null })]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByText(/Never expires/)).toBeInTheDocument();
      });

      await openOverflowMenu();
      expect(screen.queryByText("Extend")).not.toBeInTheDocument();
    });

    it("shows Set expiry button when shareExpiresAt is null", async () => {
      mockFetch([makeVideo({ shareExpiresAt: null })]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
      });
      await openOverflowMenu();
      expect(screen.getByText("Set expiry")).toBeInTheDocument();
    });

    it("shows Remove expiry button when shareExpiresAt is set", async () => {
      mockFetch([makeVideo()]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
      });
      await openOverflowMenu();
      expect(screen.getByText("Remove expiry")).toBeInTheDocument();
    });

    it("calls API to remove expiry", async () => {
      mockFetch([makeVideo()]);
      mockApiFetch.mockResolvedValueOnce(undefined);
      mockApiFetch.mockResolvedValueOnce([makeVideo()]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
      });

      await openOverflowMenu();
      await userEvent.click(screen.getByText("Remove expiry"));

      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/link-expiry", {
        method: "PUT",
        body: JSON.stringify({ neverExpires: true }),
      });
    });

    it("calls API to set expiry", async () => {
      mockFetch([makeVideo({ shareExpiresAt: null })]);
      mockApiFetch.mockResolvedValueOnce(undefined);
      mockApiFetch.mockResolvedValueOnce([makeVideo()]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
      });

      await openOverflowMenu();
      await userEvent.click(screen.getByText("Set expiry"));

      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/link-expiry", {
        method: "PUT",
        body: JSON.stringify({ neverExpires: false }),
      });
    });
  });

  describe("toggle actions keep menu open", () => {
    it("keeps menu open after toggling download", async () => {
      const user = userEvent.setup();
      mockFetch([makeVideo({ downloadEnabled: true })]);
      mockApiFetch.mockResolvedValueOnce(undefined);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
      });

      await openOverflowMenu(user);
      await user.click(screen.getByText("Downloads on"));

      // Menu stays open, label updates
      expect(screen.getByText("Downloads off")).toBeInTheDocument();
      expect(screen.getByText("Trim")).toBeInTheDocument(); // menu still visible
    });

    it("keeps menu open after toggling expiry", async () => {
      const user = userEvent.setup();
      mockFetch([makeVideo()]);
      mockApiFetch.mockResolvedValueOnce(undefined);
      mockApiFetch.mockResolvedValueOnce([makeVideo({ shareExpiresAt: null })]);
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
      });

      await openOverflowMenu(user);
      await user.click(screen.getByText("Remove expiry"));

      await waitFor(() => {
        // Menu stays open, label updates
        expect(screen.getByText("Set expiry")).toBeInTheDocument();
      });
      expect(screen.getByText("Trim")).toBeInTheDocument(); // menu still visible
    });

    it("keeps menu open after adding password", async () => {
      const user = userEvent.setup();
      vi.spyOn(window, "prompt").mockReturnValue("secret123");
      mockFetch([makeVideo({ hasPassword: false })]);
      mockApiFetch.mockResolvedValueOnce(undefined); // PUT password
      mockApiFetch.mockResolvedValueOnce([makeVideo({ hasPassword: true })]); // refetch videos
      mockApiFetch.mockResolvedValueOnce(unlimitedLimits); // refetch limits
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
      });

      await openOverflowMenu(user);
      await user.click(screen.getByRole("button", { name: "Add password" }));

      await waitFor(() => {
        // Menu stays open, label updates
        expect(screen.getByText("Remove password")).toBeInTheDocument();
      });
      expect(screen.getByText("Trim")).toBeInTheDocument(); // menu still visible
    });

    it("keeps menu open after changing notification", async () => {
      const user = userEvent.setup();
      mockFetch([makeVideo({ viewNotification: null })]);
      mockApiFetch.mockResolvedValueOnce(undefined); // PUT response
      renderLibrary();

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "More actions" })).toBeInTheDocument();
      });

      await openOverflowMenu(user);
      await user.selectOptions(screen.getByLabelText("View notifications"), "every");

      await waitFor(() => {
        const select = screen.getByLabelText("View notifications") as HTMLSelectElement;
        expect(select.value).toBe("every");
      });
      expect(screen.getByText("Trim")).toBeInTheDocument(); // menu still visible
    });
  });
});
