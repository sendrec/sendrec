import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Library } from "./Library";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

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
    ...overrides,
  };
}

function renderLibrary() {
  return render(
    <MemoryRouter>
      <Library />
    </MemoryRouter>
  );
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
    mockApiFetch.mockResolvedValueOnce([]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("No recordings yet.")).toBeInTheDocument();
    });
    expect(screen.getByRole("link", { name: "Create your first recording" })).toHaveAttribute("href", "/");
  });

  it("renders video list with title and metadata", async () => {
    mockApiFetch.mockResolvedValueOnce([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("My Recording")).toBeInTheDocument();
    });
    // Duration "2:05" should appear
    expect(screen.getByText(/2:05/)).toBeInTheDocument();
  });

  it("shows view counts", async () => {
    mockApiFetch.mockResolvedValueOnce([makeVideo({ viewCount: 3, uniqueViewCount: 2 })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText(/3 views \(2 unique\)/)).toBeInTheDocument();
    });
  });

  it("shows 'No views yet' for zero views", async () => {
    mockApiFetch.mockResolvedValueOnce([makeVideo({ viewCount: 0, uniqueViewCount: 0 })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText(/No views yet/)).toBeInTheDocument();
    });
  });

  it("shows expiry label", async () => {
    mockApiFetch.mockResolvedValueOnce([
      makeVideo({ shareExpiresAt: new Date(Date.now() + 3 * 86400000).toISOString() }),
    ]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText(/Expires in/)).toBeInTheDocument();
    });
  });

  it("shows expired label for past expiry", async () => {
    mockApiFetch.mockResolvedValueOnce([
      makeVideo({ shareExpiresAt: new Date(Date.now() - 86400000).toISOString() }),
    ]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText(/Expired/)).toBeInTheDocument();
    });
  });

  it("renders copy link and delete buttons for ready videos", async () => {
    mockApiFetch.mockResolvedValueOnce([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Copy link" })).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "Delete" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Extend" })).toBeInTheDocument();
  });

  it("shows uploading status", async () => {
    mockApiFetch.mockResolvedValueOnce([makeVideo({ status: "uploading" })]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("uploading...")).toBeInTheDocument();
    });
  });

  it("renders thumbnail when available", async () => {
    mockApiFetch.mockResolvedValueOnce([makeVideo()]);
    const { container } = renderLibrary();

    await waitFor(() => {
      const img = container.querySelector("img");
      expect(img).not.toBeNull();
      expect(img).toHaveAttribute("src", "https://storage.sendrec.eu/thumb.jpg");
    });
  });

  it("does not render thumbnail when unavailable", async () => {
    mockApiFetch.mockResolvedValueOnce([makeVideo({ thumbnailUrl: undefined })]);
    const { container } = renderLibrary();

    await waitFor(() => {
      expect(screen.getByText("My Recording")).toBeInTheDocument();
    });
    expect(container.querySelector("img")).toBeNull();
  });

  it("confirms before deleting", async () => {
    const user = userEvent.setup();
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    mockApiFetch.mockResolvedValueOnce([makeVideo()]);
    renderLibrary();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Delete" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Delete" }));
    expect(confirmSpy).toHaveBeenCalledWith("Delete this recording? This cannot be undone.");
    // Should not have called delete API
    expect(mockApiFetch).toHaveBeenCalledTimes(1); // only initial fetch
  });

  it("deletes video when confirmed", async () => {
    const user = userEvent.setup();
    vi.spyOn(window, "confirm").mockReturnValue(true);
    mockApiFetch
      .mockResolvedValueOnce([makeVideo()])
      .mockResolvedValueOnce(undefined); // delete response
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
});
