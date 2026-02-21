import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { VideoDetail } from "./VideoDetail";

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
    hasPassword: false,
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

const unlimitedLimits = {
  maxVideosPerMonth: 0,
  maxVideoDurationSeconds: 0,
  videosUsedThisMonth: 0,
  brandingEnabled: false,
  aiEnabled: false,
};

function renderVideoDetail(videoId = "v1", routerState?: unknown) {
  return render(
    <MemoryRouter
      initialEntries={[{ pathname: `/videos/${videoId}`, state: routerState }]}
    >
      <Routes>
        <Route path="/videos/:id" element={<VideoDetail />} />
        <Route path="/library" element={<div>Library Page</div>} />
      </Routes>
    </MemoryRouter>
  );
}

describe("VideoDetail", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders video title and metadata from router state", async () => {
    const video = makeVideo();
    mockApiFetch
      .mockResolvedValueOnce(unlimitedLimits)
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([]);

    renderVideoDetail("v1", { video });

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
        "My Recording"
      );
    });
    expect(screen.getByText(/2:05/)).toBeInTheDocument();
    expect(screen.getByText(/3 views/)).toBeInTheDocument();
  });

  it("fetches video list when no router state and finds video by id", async () => {
    const video = makeVideo({ id: "v1", title: "Fetched Video" });
    mockApiFetch
      .mockResolvedValueOnce([video, makeVideo({ id: "v2", title: "Other" })])
      .mockResolvedValueOnce(unlimitedLimits)
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([]);

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
        "Fetched Video"
      );
    });
    expect(mockApiFetch).toHaveBeenCalledWith("/api/videos");
  });

  it("shows back to library link", async () => {
    const video = makeVideo();
    mockApiFetch
      .mockResolvedValueOnce(unlimitedLimits)
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([]);

    renderVideoDetail("v1", { video });

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    });

    const libraryLink = screen.getByRole("link", { name: /Library/ });
    expect(libraryLink).toHaveAttribute("href", "/library");
  });

  it("shows view as viewer link pointing to /watch/{shareToken}", async () => {
    const video = makeVideo({ shareToken: "tok456" });
    mockApiFetch
      .mockResolvedValueOnce(unlimitedLimits)
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([]);

    renderVideoDetail("v1", { video });

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    });

    const viewerLink = screen.getByRole("link", { name: /View as viewer/ });
    expect(viewerLink).toHaveAttribute("href", "/watch/tok456");
    expect(viewerLink).toHaveAttribute("target", "_blank");
  });

  it("shows video not found when ID does not match", async () => {
    mockApiFetch
      .mockResolvedValueOnce([makeVideo({ id: "v1" })])
      .mockResolvedValueOnce(unlimitedLimits)
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([]);

    renderVideoDetail("nonexistent");

    await waitFor(() => {
      expect(screen.getByText("Video not found")).toBeInTheDocument();
    });

    const backLink = screen.getByRole("link", { name: /Library/ });
    expect(backLink).toHaveAttribute("href", "/library");
  });

  it("shows loading state initially when no router state", () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}));

    renderVideoDetail("v1");

    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("displays thumbnail when available", async () => {
    const video = makeVideo({
      thumbnailUrl: "https://storage.sendrec.eu/thumb.jpg",
    });
    mockApiFetch
      .mockResolvedValueOnce(unlimitedLimits)
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([]);

    renderVideoDetail("v1", { video });

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    });

    const thumbnail = screen.getByAltText("Video thumbnail");
    expect(thumbnail).toHaveAttribute(
      "src",
      "https://storage.sendrec.eu/thumb.jpg"
    );
  });

  it("displays tag chips", async () => {
    const video = makeVideo({
      tags: [
        { id: "t1", name: "Demo", color: "#3b82f6" },
        { id: "t2", name: "Internal", color: null },
      ],
    });
    mockApiFetch
      .mockResolvedValueOnce(unlimitedLimits)
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([]);

    renderVideoDetail("v1", { video });

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    });

    expect(screen.getByText("Demo")).toBeInTheDocument();
    expect(screen.getByText("Internal")).toBeInTheDocument();
  });

  it("shows unique view count when different from total", async () => {
    const video = makeVideo({ viewCount: 10, uniqueViewCount: 7 });
    mockApiFetch
      .mockResolvedValueOnce(unlimitedLimits)
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([]);

    renderVideoDetail("v1", { video });

    await waitFor(() => {
      expect(screen.getByText(/10 views/)).toBeInTheDocument();
    });
    expect(screen.getByText(/7 unique/)).toBeInTheDocument();
  });

  it("shows never expires for null expiry", async () => {
    const video = makeVideo({ shareExpiresAt: null });
    mockApiFetch
      .mockResolvedValueOnce(unlimitedLimits)
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([]);

    renderVideoDetail("v1", { video });

    await waitFor(() => {
      expect(screen.getByText(/Never expires/)).toBeInTheDocument();
    });
  });

  it("shows expired for past expiry date", async () => {
    const video = makeVideo({
      shareExpiresAt: new Date(Date.now() - 86400000).toISOString(),
    });
    mockApiFetch
      .mockResolvedValueOnce(unlimitedLimits)
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([]);

    renderVideoDetail("v1", { video });

    await waitFor(() => {
      expect(screen.getByText(/Expired/)).toBeInTheDocument();
    });
  });
});
