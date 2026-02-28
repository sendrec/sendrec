import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { VideoDetail } from "./VideoDetail";
import { expectNoA11yViolations } from "../test-utils/a11y";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

vi.mock("../components/TrimModal", () => ({
  TrimModal: ({
    onClose,
    onTrimStarted,
  }: {
    onClose: () => void;
    onTrimStarted: () => void;
  }) => (
    <div data-testid="trim-modal">
      <button onClick={onClose}>Close trim</button>
      <button onClick={onTrimStarted}>Start trim</button>
    </div>
  ),
}));

vi.mock("../components/FillerRemovalModal", () => ({
  FillerRemovalModal: ({
    onClose,
    onRemovalStarted,
  }: {
    onClose: () => void;
    onRemovalStarted: () => void;
  }) => (
    <div data-testid="filler-modal">
      <button onClick={onClose}>Close filler</button>
      <button onClick={onRemovalStarted}>Start filler removal</button>
    </div>
  ),
}));

vi.mock("../components/SilenceRemovalModal", () => ({
  SilenceRemovalModal: ({
    onClose,
    onRemovalStarted,
  }: {
    onClose: () => void;
    onRemovalStarted: () => void;
  }) => (
    <div data-testid="silence-modal">
      <button onClick={onClose}>Close silence</button>
      <button onClick={onRemovalStarted}>Start silence removal</button>
    </div>
  ),
}));

vi.mock("../components/DocumentModal", () => ({
  DocumentModal: ({
    onClose,
  }: {
    document: string;
    onClose: () => void;
  }) => (
    <div data-testid="document-modal">
      <button onClick={onClose}>Close document modal</button>
    </div>
  ),
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
    documentStatus: "none",
    folderId: null,
    transcriptionLanguage: null,
    tags: [],
    playlists: [],
    ...overrides,
  };
}

const defaultLimits = {
  maxVideosPerMonth: 0,
  maxVideoDurationSeconds: 0,
  videosUsedThisMonth: 0,
  brandingEnabled: false,
  aiEnabled: false,
  transcriptionEnabled: false,
};

const defaultFolders = [
  {
    id: "f1",
    name: "Marketing",
    position: 0,
    videoCount: 3,
    createdAt: "2026-01-01T00:00:00Z",
  },
  {
    id: "f2",
    name: "Product",
    position: 1,
    videoCount: 1,
    createdAt: "2026-01-02T00:00:00Z",
  },
];

const defaultTags = [
  {
    id: "t1",
    name: "Demo",
    color: "#3b82f6",
    videoCount: 2,
    createdAt: "2026-01-01T00:00:00Z",
  },
  {
    id: "t2",
    name: "Internal",
    color: null,
    videoCount: 1,
    createdAt: "2026-01-02T00:00:00Z",
  },
];

const defaultPlaylists = [
  {
    id: "pl1",
    title: "Onboarding",
    videoCount: 3,
    createdAt: "2026-01-01T00:00:00Z",
  },
  {
    id: "pl2",
    title: "Product Demos",
    videoCount: 1,
    createdAt: "2026-01-02T00:00:00Z",
  },
];

const defaultComments = { comments: [], commentMode: "disabled" };

function setupDefaultMocks(
  overrides: {
    video?: Record<string, unknown>;
    limits?: Record<string, unknown>;
    folders?: Record<string, unknown>[];
    tags?: Record<string, unknown>[];
    playlists?: Record<string, unknown>[];
    comments?: Record<string, unknown>;
  } = {},
) {
  mockApiFetch
    .mockResolvedValueOnce([overrides.video ?? makeVideo()])
    .mockResolvedValueOnce(overrides.limits ?? defaultLimits)
    .mockResolvedValueOnce(overrides.folders ?? defaultFolders)
    .mockResolvedValueOnce(overrides.tags ?? defaultTags)
    .mockResolvedValueOnce(overrides.playlists ?? defaultPlaylists)
    .mockResolvedValueOnce({ downloadUrl: "https://s3.example.com/video.webm" })
    .mockResolvedValueOnce(overrides.comments ?? defaultComments);
}

function setupWithTranscript(
  overrides: {
    video?: Record<string, unknown>;
    limits?: Record<string, unknown>;
    folders?: Record<string, unknown>[];
    tags?: Record<string, unknown>[];
    playlists?: Record<string, unknown>[];
    comments?: Record<string, unknown>;
    transcript?: Record<string, unknown>;
  } = {},
) {
  const { video: videoOverrides, transcript, ...rest } = overrides;
  setupDefaultMocks({
    video: makeVideo({ transcriptStatus: "ready", ...(videoOverrides ?? {}) }),
    ...rest,
  });
  mockApiFetch.mockResolvedValueOnce(
    transcript ?? { status: "ready", segments: [{ start: 0, end: 2.5, text: "Hello" }] },
  );
}

function renderVideoDetail(videoId = "v1") {
  return render(
    <MemoryRouter
      initialEntries={[`/videos/${videoId}`]}
    >
      <Routes>
        <Route path="/videos/:id" element={<VideoDetail />} />
        <Route path="/library" element={<div>Library Page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("VideoDetail", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("has no accessibility violations", async () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}));
    const { container } = renderVideoDetail();
    await expectNoA11yViolations(container);
  });

  // ─── Skeleton tests ───────────────────────────────────────────

  it("renders video title and metadata after fetching", async () => {
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
        "My Recording",
      );
    });
    expect(screen.getByText(/2:05/)).toBeInTheDocument();
    expect(screen.getByText(/3 views/)).toBeInTheDocument();
  });

  it("fetches video list when no router state and finds video by id", async () => {
    const video = makeVideo({ id: "v1", title: "Fetched Video" });
    mockApiFetch
      .mockResolvedValueOnce([video, makeVideo({ id: "v2", title: "Other" })])
      .mockResolvedValueOnce(defaultLimits)
      .mockResolvedValueOnce(defaultFolders)
      .mockResolvedValueOnce(defaultTags)
      .mockResolvedValueOnce(defaultPlaylists)
      .mockResolvedValueOnce({ downloadUrl: "https://s3.example.com/video.webm" })
      .mockResolvedValueOnce(defaultComments);

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
        "Fetched Video",
      );
    });
    expect(mockApiFetch).toHaveBeenCalledWith("/api/videos");
  });

  it("shows back to library link", async () => {
    const video = makeVideo();
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    });

    const libraryLink = screen.getByRole("link", { name: /Library/ });
    expect(libraryLink).toHaveAttribute("href", "/library");
  });

  it("shows view as viewer link pointing to /watch/{shareToken}", async () => {
    const video = makeVideo({ shareToken: "tok456" });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

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
      .mockResolvedValueOnce(defaultLimits)
      .mockResolvedValueOnce(defaultFolders)
      .mockResolvedValueOnce(defaultTags)
      .mockResolvedValueOnce(defaultPlaylists)
      .mockResolvedValueOnce({ downloadUrl: "https://s3.example.com/video.webm" })
      .mockResolvedValueOnce(defaultComments);

    renderVideoDetail("nonexistent");

    await waitFor(() => {
      expect(screen.getByText("Video not found")).toBeInTheDocument();
    });

    const backLink = screen.getByRole("link", { name: /Library/ });
    expect(backLink).toHaveAttribute("href", "/library");
  });

  it("shows loading state initially when no router state", () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}));

    const { container } = renderVideoDetail("v1");

    expect(container.querySelector(".skeleton-thumb")).toBeInTheDocument();
    expect(container.querySelector(".skeleton-title")).toBeInTheDocument();
    expect(container.querySelector(".skeleton-btn")).toBeInTheDocument();
  });

  it("displays video player when download URL is available", async () => {
    setupDefaultMocks();

    const { container } = renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    });

    await waitFor(() => {
      const video = container.querySelector("video");
      expect(video).not.toBeNull();
      expect(video?.getAttribute("src")).toBe("https://s3.example.com/video.webm");
      expect(video?.getAttribute("poster")).toBe("https://storage.sendrec.eu/thumb.jpg");
    });
  });

  it("displays tag chips", async () => {
    const video = makeVideo({
      tags: [
        { id: "t1", name: "Demo", color: "#3b82f6" },
        { id: "t2", name: "Internal", color: null },
      ],
    });
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    });

    // Tags appear as header chips and as Organize toggle buttons
    expect(screen.getAllByText("Demo").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Internal").length).toBeGreaterThanOrEqual(1);
  });

  it("shows unique view count when different from total", async () => {
    const video = makeVideo({ viewCount: 10, uniqueViewCount: 7 });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText(/10 views/)).toBeInTheDocument();
    });
    expect(screen.getByText(/7 unique/)).toBeInTheDocument();
  });

  it("shows never expires for null expiry", async () => {
    const video = makeVideo({ shareExpiresAt: null });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      // Appears in both header metadata and Sharing expiry row
      expect(screen.getAllByText(/Never expires/).length).toBeGreaterThanOrEqual(1);
    });
  });

  it("shows expired for past expiry date", async () => {
    const video = makeVideo({
      shareExpiresAt: new Date(Date.now() - 86400000).toISOString(),
    });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      // Appears in both header metadata and Sharing expiry row
      expect(screen.getAllByText(/Expired/).length).toBeGreaterThanOrEqual(1);
    });
  });

  // ─── Share Settings section ──────────────────────────────────

  it("shows Share Settings section heading", async () => {
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 2, name: "Share Settings" }),
      ).toBeInTheDocument();
    });
  });

  it("shows share link input with video URL and copy button", async () => {
    const video = makeVideo({
      shareUrl: "https://app.sendrec.eu/watch/abc123",
    });
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      const input = screen.getByLabelText("Share link") as HTMLInputElement;
      expect(input.value).toBe("https://app.sendrec.eu/watch/abc123");
      expect(input).toHaveAttribute("readOnly");
    });

    expect(screen.getByText("Copy link")).toBeInTheDocument();
  });

  it("copies share link to clipboard when copy button clicked", async () => {
    const video = makeVideo({
      shareUrl: "https://app.sendrec.eu/watch/abc123",
    });
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Copy link")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Copy link"));

    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
        "https://app.sendrec.eu/watch/abc123",
      );
    });
  });

  it("shows embed copy button", async () => {
    const video = makeVideo();
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Copy embed")).toBeInTheDocument();
    });

    const embedInput = screen.getByLabelText("Embed code") as HTMLInputElement;
    expect(embedInput.value).toContain("<iframe");
    expect(embedInput.value).toContain("/embed/abc123");
  });

  it("shows password controls - set password when none", async () => {
    const video = makeVideo({ hasPassword: false });
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("No password")).toBeInTheDocument();
      expect(screen.getByText("Set password")).toBeInTheDocument();
    });
  });

  it("shows password controls - remove password when set", async () => {
    const video = makeVideo({ hasPassword: true });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Password set")).toBeInTheDocument();
      expect(screen.getByText("Remove password")).toBeInTheDocument();
    });
  });

  it("toggle download calls API with correct body", async () => {
    const video = makeVideo({ downloadEnabled: true });
    setupDefaultMocks();
    mockApiFetch.mockResolvedValueOnce(undefined); // toggle response

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Enabled")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Enabled"));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        "/api/videos/v1/download-enabled",
        {
          method: "PUT",
          body: JSON.stringify({ downloadEnabled: false }),
        },
      );
    });
  });

  it("toggle email gate calls API", async () => {
    const video = makeVideo({ emailGateEnabled: false });
    setupDefaultMocks();
    mockApiFetch.mockResolvedValueOnce(undefined);

    renderVideoDetail("v1");

    await waitFor(() => {
      // Find the toggle in the email gate row
      const rows = document.querySelectorAll(".detail-setting-row");
      expect(rows.length).toBeGreaterThan(0);
    });

    // The email gate toggle shows "Disabled" since emailGateEnabled: false
    const emailToggle = screen.getAllByText("Disabled")[0];
    fireEvent.click(emailToggle);

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        "/api/videos/v1/email-gate",
        {
          method: "PUT",
          body: JSON.stringify({ enabled: true }),
        },
      );
    });
  });

  it("shows comments dropdown with correct value", async () => {
    const video = makeVideo({ commentMode: "anonymous" });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      const select = screen.getByLabelText("Comment mode") as HTMLSelectElement;
      expect(select.value).toBe("anonymous");
    });
  });

  it("changes comment mode when dropdown changed", async () => {
    const video = makeVideo({ commentMode: "disabled" });
    setupDefaultMocks();
    mockApiFetch.mockResolvedValueOnce(undefined);

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByLabelText("Comment mode")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Comment mode"), {
      target: { value: "name_required" },
    });

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        "/api/videos/v1/comment-mode",
        {
          method: "PUT",
          body: JSON.stringify({ commentMode: "name_required" }),
        },
      );
    });
  });

  it("shows expiry controls with extend when expiry is set", async () => {
    const video = makeVideo({
      shareExpiresAt: new Date(Date.now() + 5 * 86400000).toISOString(),
    });
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Remove expiry")).toBeInTheDocument();
      expect(screen.getByText("Extend")).toBeInTheDocument();
    });
  });

  it("shows set expiry when never expires", async () => {
    const video = makeVideo({ shareExpiresAt: null });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Set expiry")).toBeInTheDocument();
    });

    expect(screen.queryByText("Extend")).not.toBeInTheDocument();
  });

  it("shows CTA 'None' and 'Add CTA' when no CTA set", async () => {
    const video = makeVideo({ ctaText: null, ctaUrl: null });
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Add CTA")).toBeInTheDocument();
    });

    // "None" text appears both in CTA value and Folder dropdown option
    // Check that the CTA row has the "None" span
    const ctaRow = screen.getByText("Call to action").closest(".detail-setting-row");
    expect(ctaRow).not.toBeNull();
    const noneSpan = ctaRow!.querySelector(".detail-setting-value span");
    expect(noneSpan).toHaveTextContent("None");
  });

  it("shows CTA form when Add CTA button clicked", async () => {
    const video = makeVideo({ ctaText: null, ctaUrl: null });
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Add CTA")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Add CTA"));

    expect(screen.getByLabelText("CTA text")).toBeInTheDocument();
    expect(screen.getByLabelText("CTA URL")).toBeInTheDocument();
    expect(screen.getByText("Save")).toBeInTheDocument();
    expect(screen.getByText("Cancel")).toBeInTheDocument();
  });

  it("shows Edit CTA when CTA is set", async () => {
    const video = makeVideo({
      ctaText: "Book a demo",
      ctaUrl: "https://example.com",
    });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Book a demo")).toBeInTheDocument();
      expect(screen.getByText("Edit CTA")).toBeInTheDocument();
    });
  });

  // ─── Editing section ──────────────────────────────────────────

  it("shows Editing section heading", async () => {
    const video = makeVideo();
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 2, name: "Editing" }),
      ).toBeInTheDocument();
    });
  });

  it("shows title edit button, clicking reveals input", async () => {
    const video = makeVideo({ title: "My Recording" });
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 1 }),
      ).toHaveTextContent("My Recording");
    });

    const editButton = screen.getByLabelText("Edit title");
    fireEvent.click(editButton);

    const input = screen.getByDisplayValue("My Recording");
    expect(input).toBeInTheDocument();
    expect(input).toHaveAttribute("aria-label", "Edit title");
  });

  it("saves title on Enter key", async () => {
    const video = makeVideo({ title: "Old Title" });
    setupDefaultMocks({ video });
    mockApiFetch.mockResolvedValueOnce(undefined);

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByLabelText("Edit title"));

    const input = screen.getByDisplayValue("Old Title");
    fireEvent.change(input, { target: { value: "New Title" } });
    fireEvent.keyDown(input, { key: "Enter" });

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1", {
        method: "PATCH",
        body: JSON.stringify({ title: "New Title" }),
      });
    });
  });

  it("shows transcript status 'Not started' with Transcribe button", async () => {
    const video = makeVideo({ transcriptStatus: "none" });
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Not started")).toBeInTheDocument();
      expect(screen.getByText("Transcribe")).toBeInTheDocument();
    });
  });

  it("shows transcript status 'Pending...' without action button", async () => {
    const video = makeVideo({ transcriptStatus: "pending" });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Pending...")).toBeInTheDocument();
    });

    expect(screen.queryByText("Transcribe")).not.toBeInTheDocument();
    expect(screen.queryByText("Redo transcript")).not.toBeInTheDocument();
  });

  it("shows transcript status 'Transcribing...' without action button", async () => {
    const video = makeVideo({ transcriptStatus: "processing" });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Transcribing...")).toBeInTheDocument();
    });

    expect(screen.queryByText("Transcribe")).not.toBeInTheDocument();
  });

  it("shows transcript status 'Ready' with Redo transcript button", async () => {
    setupWithTranscript();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Ready")).toBeInTheDocument();
      expect(screen.getByText("Redo transcript")).toBeInTheDocument();
    });
  });

  it("shows transcript status 'Failed' with Retry transcript button", async () => {
    const video = makeVideo({ transcriptStatus: "failed" });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Failed")).toBeInTheDocument();
      expect(screen.getByText("Retry transcript")).toBeInTheDocument();
    });
  });

  it("calls retranscribe API when Transcribe clicked", async () => {
    const video = makeVideo({ transcriptStatus: "none" });
    setupDefaultMocks();
    mockApiFetch.mockResolvedValueOnce(undefined);

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Transcribe")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Transcribe"));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        "/api/videos/v1/retranscribe",
        { method: "POST" },
      );
    });
  });

  it("shows summarize button when AI enabled", async () => {
    const video = makeVideo({
      transcriptStatus: "ready",
      summaryStatus: "none",
    });
    setupDefaultMocks({ limits: { ...defaultLimits, aiEnabled: true } });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Summarize")).toBeInTheDocument();
    });
  });

  it("hides summarize button when AI disabled", async () => {
    setupDefaultMocks({ limits: { ...defaultLimits, aiEnabled: false } });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 2, name: "AI" }),
      ).toBeInTheDocument();
    });

    expect(screen.queryByText("Summarize")).not.toBeInTheDocument();
  });

  it("shows Re-summarize when summary is ready", async () => {
    const video = makeVideo({ summaryStatus: "ready" });
    setupDefaultMocks({ video, limits: { ...defaultLimits, aiEnabled: true } });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Re-summarize")).toBeInTheDocument();
    });
  });

  it("disables summarize when pending", async () => {
    const video = makeVideo({ summaryStatus: "pending" });
    setupDefaultMocks({ video, limits: { ...defaultLimits, aiEnabled: true } });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Summarize")).toBeInTheDocument();
    });

    expect(screen.getByText("Summarize")).toBeDisabled();
  });

  it("shows suggested title with accept and dismiss buttons", async () => {
    const video = makeVideo({ suggestedTitle: "Better Title" });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Better Title")).toBeInTheDocument();
      expect(screen.getByText("Accept")).toBeInTheDocument();
      expect(screen.getByText("Dismiss")).toBeInTheDocument();
    });
  });

  it("accepts suggested title", async () => {
    const video = makeVideo({ suggestedTitle: "Better Title" });
    setupDefaultMocks({ video });
    mockApiFetch.mockResolvedValueOnce(undefined);

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Accept")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Accept"));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1", {
        method: "PATCH",
        body: JSON.stringify({ title: "Better Title" }),
      });
    });
  });

  it("dismisses suggested title", async () => {
    const video = makeVideo({ suggestedTitle: "Better Title" });
    setupDefaultMocks({ video });
    mockApiFetch.mockResolvedValueOnce(undefined);

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Dismiss")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Dismiss"));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        "/api/videos/v1/dismiss-title",
        { method: "PUT" },
      );
    });
  });

  it("does not show suggested title when null", async () => {
    const video = makeVideo({ suggestedTitle: null });
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 2, name: "Editing" }),
      ).toBeInTheDocument();
    });

    expect(screen.queryByText("Suggested title")).not.toBeInTheDocument();
  });

  it("shows trim button", async () => {
    const video = makeVideo();
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Trim video")).toBeInTheDocument();
    });
  });

  it("opens trim modal when trim button clicked", async () => {
    const video = makeVideo();
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Trim video")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Trim video"));

    expect(screen.getByTestId("trim-modal")).toBeInTheDocument();
  });

  it("shows remove fillers button when transcript ready", async () => {
    setupWithTranscript();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Remove fillers")).toBeInTheDocument();
    });
  });

  it("hides remove fillers when transcript not ready", async () => {
    const video = makeVideo({ transcriptStatus: "none" });
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 2, name: "Editing" }),
      ).toBeInTheDocument();
    });

    expect(screen.queryByText("Remove fillers")).not.toBeInTheDocument();
  });

  it("opens filler modal when remove fillers clicked", async () => {
    setupWithTranscript();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Remove fillers")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Remove fillers"));

    expect(screen.getByTestId("filler-modal")).toBeInTheDocument();
  });

  it("opens silence removal modal when remove silence clicked", async () => {
    const video = makeVideo({ status: "ready" });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Remove silence")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Remove silence"));

    expect(screen.getByTestId("silence-modal")).toBeInTheDocument();
  });

  // ─── Thumbnail, Notifications, Branding (in Share Settings) ──

  it("shows thumbnail upload button", async () => {
    const video = makeVideo();
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Upload")).toBeInTheDocument();
    });
  });

  it("shows reset thumbnail when custom thumbnail exists", async () => {
    const video = makeVideo({
      thumbnailUrl: "https://storage.sendrec.eu/custom-thumb.jpg",
    });
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Reset thumbnail")).toBeInTheDocument();
    });
  });

  it("hides reset thumbnail when no thumbnail", async () => {
    const video = makeVideo({ thumbnailUrl: undefined });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 2, name: "Share Settings" }),
      ).toBeInTheDocument();
    });

    expect(screen.queryByText("Reset thumbnail")).not.toBeInTheDocument();
  });

  it("shows notifications dropdown", async () => {
    const video = makeVideo({ viewNotification: "every" });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      const select = screen.getByLabelText(
        "View notifications",
      ) as HTMLSelectElement;
      expect(select.value).toBe("every");
    });
  });

  it("shows branding button when branding enabled", async () => {
    const video = makeVideo();
    setupDefaultMocks({
      limits: { ...defaultLimits, brandingEnabled: true },
    });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Customize")).toBeInTheDocument();
    });
  });

  it("hides branding button when branding disabled", async () => {
    setupDefaultMocks({
      limits: { ...defaultLimits, brandingEnabled: false },
    });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 2, name: "Share Settings" }),
      ).toBeInTheDocument();
    });

    expect(screen.queryByText("Customize")).not.toBeInTheDocument();
  });

  it("opens branding modal when Customize clicked", async () => {
    const video = makeVideo();
    setupDefaultMocks({
      limits: { ...defaultLimits, brandingEnabled: true },
    });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Customize")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      companyName: null,
      colorBackground: null,
      colorSurface: null,
      colorText: null,
      colorAccent: null,
      footerText: null,
    });

    fireEvent.click(screen.getByText("Customize"));

    await waitFor(() => {
      expect(screen.getByText("Video Branding")).toBeInTheDocument();
    });
  });

  // ─── Organize section ─────────────────────────────────────

  it("shows Organize section heading", async () => {
    const video = makeVideo();
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 2, name: "Organize" }),
      ).toBeInTheDocument();
    });
  });

  it("shows folder dropdown with options from fetched folders", async () => {
    const video = makeVideo({ folderId: null });
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      const select = screen.getByLabelText("Folder") as HTMLSelectElement;
      expect(select.value).toBe("");
    });

    const options = screen.getByLabelText("Folder").querySelectorAll("option");
    expect(options).toHaveLength(3); // None + Marketing + Product
    expect(options[0]).toHaveTextContent("None");
    expect(options[1]).toHaveTextContent("Marketing");
    expect(options[2]).toHaveTextContent("Product");
  });

  it("shows selected folder in dropdown", async () => {
    const video = makeVideo({ folderId: "f1" });
    setupDefaultMocks({ video });

    renderVideoDetail("v1");

    await waitFor(() => {
      const select = screen.getByLabelText("Folder") as HTMLSelectElement;
      expect(select.value).toBe("f1");
    });
  });

  it("changes folder via API when dropdown changed", async () => {
    const video = makeVideo({ folderId: null });
    setupDefaultMocks();
    mockApiFetch.mockResolvedValueOnce(undefined);

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByLabelText("Folder")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Folder"), {
      target: { value: "f1" },
    });

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/folder", {
        method: "PUT",
        body: JSON.stringify({ folderId: "f1" }),
      });
    });
  });

  it("shows tag toggle buttons from fetched tags", async () => {
    const video = makeVideo({
      tags: [{ id: "t1", name: "Demo", color: "#3b82f6" }],
    });
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByLabelText("Tag Demo")).toBeInTheDocument();
      expect(screen.getByLabelText("Tag Internal")).toBeInTheDocument();
    });
  });

  it("toggles tag via API when tag button clicked", async () => {
    const video = makeVideo({ tags: [] });
    setupDefaultMocks();
    mockApiFetch.mockResolvedValueOnce(undefined);

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByLabelText("Tag Demo")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByLabelText("Tag Demo"));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/tags", {
        method: "PUT",
        body: JSON.stringify({ tagIds: ["t1"] }),
      });
    });
  });

  // ─── Delete ───────────────────────────────────────────────────

  it("shows delete button", async () => {
    const video = makeVideo();
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Delete video")).toBeInTheDocument();
    });
  });

  it("deletes video and navigates to library on confirm", async () => {
    setupDefaultMocks();
    mockApiFetch.mockResolvedValueOnce(undefined);

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Delete video")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Delete video"));

    const dialog = screen.getByRole("alertdialog");
    fireEvent.click(within(dialog).getByText("Delete"));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1", {
        method: "DELETE",
      });
    });

    await waitFor(() => {
      expect(screen.getByText("Library Page")).toBeInTheDocument();
    });
  });

  it("does not delete when confirm is cancelled", async () => {
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Delete video")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Delete video"));

    const dialog = screen.getByRole("alertdialog");
    fireEvent.click(within(dialog).getByText("Cancel"));

    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    // Should not have called delete API (only the initial 7 setup calls)
    expect(mockApiFetch).toHaveBeenCalledTimes(7);
  });

  // ─── Toast ────────────────────────────────────────────────────

  it("shows toast after copying link", async () => {
    const video = makeVideo();
    setupDefaultMocks();

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Copy link")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Copy link"));

    await waitFor(() => {
      expect(screen.getByText("Link copied")).toBeInTheDocument();
    });
  });

  // ─── Language selector ──────────────────────────────────────

  it("shows language selector next to retranscribe when transcription enabled", async () => {
    setupWithTranscript({ limits: { ...defaultLimits, transcriptionEnabled: true } });

    renderVideoDetail();

    await waitFor(() => {
      expect(screen.getByLabelText("Transcription language")).toBeInTheDocument();
    });
  });

  it("hides language selector when transcription disabled", async () => {
    setupWithTranscript();

    renderVideoDetail();

    await waitFor(() => {
      expect(screen.getByText("Ready")).toBeInTheDocument();
    });
    expect(screen.queryByLabelText("Transcription language")).not.toBeInTheDocument();
  });

  it("sends language when retranscribing with specific language", async () => {
    setupWithTranscript({ limits: { ...defaultLimits, transcriptionEnabled: true } });
    mockApiFetch.mockResolvedValueOnce(undefined); // retranscribe response

    renderVideoDetail();

    await waitFor(() => {
      expect(screen.getByLabelText("Transcription language")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Transcription language"), { target: { value: "de" } });
    fireEvent.click(screen.getByText("Redo transcript"));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        "/api/videos/v1/retranscribe",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ language: "de" }),
        }),
      );
    });
  });

  it("does not send language body when auto is selected", async () => {
    setupWithTranscript({ limits: { ...defaultLimits, transcriptionEnabled: true } });
    mockApiFetch.mockResolvedValueOnce(undefined); // retranscribe response

    renderVideoDetail();

    await waitFor(() => {
      expect(screen.getByLabelText("Transcription language")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Redo transcript"));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        "/api/videos/v1/retranscribe",
        { method: "POST" },
      );
    });
  });

  it("pre-selects video transcription language in dropdown", async () => {
    setupWithTranscript({
      video: { transcriptionLanguage: "de" },
      limits: { ...defaultLimits, transcriptionEnabled: true },
    });

    renderVideoDetail();

    await waitFor(() => {
      expect(screen.getByLabelText("Transcription language")).toHaveValue("de");
    });
  });

  it("disables summarize button when transcript is not ready", async () => {
    const video = makeVideo({ transcriptStatus: "pending", summaryStatus: "none" });
    setupDefaultMocks({ video, limits: { ...defaultLimits, aiEnabled: true } });

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByText("Summarize")).toBeInTheDocument();
    });

    expect(screen.getByText("Summarize")).toBeDisabled();
  });

  it("shows generate document button when AI enabled and transcript ready", async () => {
    setupWithTranscript({
      video: { documentStatus: "none" },
      limits: { ...defaultLimits, aiEnabled: true },
    });
    renderVideoDetail();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Generate" })).toBeInTheDocument();
    });
  });

  it("shows view and regenerate buttons when document is ready", async () => {
    setupWithTranscript({
      video: { documentStatus: "ready", document: "## Doc" },
      limits: { ...defaultLimits, aiEnabled: true },
    });
    renderVideoDetail();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "View" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Regenerate" })).toBeInTheDocument();
    });
  });

  // ─── Primary Actions Bar ──────────────────────────────────────

  it("shows primary action buttons", async () => {
    setupDefaultMocks();
    renderVideoDetail();
    await waitFor(() => {
      expect(screen.getByText("Copy share link")).toBeInTheDocument();
      expect(screen.getByText("View analytics")).toBeInTheDocument();
      expect(screen.getByText("Download")).toBeInTheDocument();
    });
  });

  // ─── AI section ───────────────────────────────────────────────

  it("shows AI section heading", async () => {
    setupDefaultMocks();
    renderVideoDetail();
    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 2, name: "AI" }),
      ).toBeInTheDocument();
    });
  });

  // ─── Transcript segments ──────────────────────────────────────

  it("shows transcript segments when ready", async () => {
    setupWithTranscript();
    const { container } = renderVideoDetail();
    await waitFor(() => {
      expect(container.querySelector(".transcript-segments")).toBeInTheDocument();
    });
    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.getByText("0:00")).toBeInTheDocument();
  });

  it("hides transcript segments when not ready", async () => {
    setupDefaultMocks();
    const { container } = renderVideoDetail();
    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    });
    expect(container.querySelector(".transcript-segments")).not.toBeInTheDocument();
  });

  // ─── Call to Action section ───────────────────────────────────

  it("shows Call to Action section heading", async () => {
    setupDefaultMocks();
    renderVideoDetail();
    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 2, name: "Call to Action" }),
      ).toBeInTheDocument();
    });
  });

  // ─── Comments section ─────────────────────────────────────────

  it("shows Comments section with count", async () => {
    setupDefaultMocks({
      comments: {
        comments: [
          {
            id: "c1",
            authorName: "Alice",
            body: "Great video!",
            isPrivate: false,
            isOwner: false,
            createdAt: "2026-02-25T10:00:00Z",
            videoTimestamp: null,
          },
        ],
        commentMode: "anonymous",
      },
    });
    renderVideoDetail();
    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 2, name: "Comments (1)" }),
      ).toBeInTheDocument();
      expect(screen.getByText("Great video!")).toBeInTheDocument();
      expect(screen.getByText("Alice")).toBeInTheDocument();
    });
  });

  it("shows empty comments state", async () => {
    setupDefaultMocks();
    renderVideoDetail();
    await waitFor(() => {
      expect(screen.getByText("No comments yet.")).toBeInTheDocument();
    });
  });

  it("shows video timestamp on comment", async () => {
    setupDefaultMocks({
      comments: {
        comments: [
          {
            id: "c1",
            authorName: "Bob",
            body: "Nice part",
            isPrivate: false,
            isOwner: false,
            createdAt: "2026-02-25T10:00:00Z",
            videoTimestamp: 65.5,
          },
        ],
        commentMode: "anonymous",
      },
    });
    renderVideoDetail();
    await waitFor(() => {
      expect(screen.getByText("@1:05")).toBeInTheDocument();
    });
  });

  it("shows private badge on private comment", async () => {
    setupDefaultMocks({
      comments: {
        comments: [
          {
            id: "c1",
            authorName: "Eve",
            body: "Secret note",
            isPrivate: true,
            isOwner: false,
            createdAt: "2026-02-25T10:00:00Z",
            videoTimestamp: null,
          },
        ],
        commentMode: "anonymous",
      },
    });
    renderVideoDetail();
    await waitFor(() => {
      expect(screen.getByText("Private")).toBeInTheDocument();
    });
  });

  it("deletes comment when delete button clicked", async () => {
    setupDefaultMocks({
      comments: {
        comments: [
          {
            id: "c1",
            authorName: "Alice",
            body: "Delete me",
            isPrivate: false,
            isOwner: true,
            createdAt: "2026-02-25T10:00:00Z",
            videoTimestamp: null,
          },
        ],
        commentMode: "anonymous",
      },
    });
    renderVideoDetail();
    await waitFor(() => {
      expect(screen.getByText("Delete me")).toBeInTheDocument();
    });
    mockApiFetch.mockResolvedValueOnce(undefined);
    fireEvent.click(screen.getByLabelText("Delete comment"));
    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        "/api/videos/v1/comments/c1",
        { method: "DELETE" },
      );
    });
  });

  // ─── Processing state tests ────────────────────────────────────

  it("shows processing status badge when video is processing", async () => {
    setupDefaultMocks({ video: makeVideo({ status: "processing" }) });
    const { container } = renderVideoDetail("v1");
    await waitFor(() => {
      expect(
        container.querySelector(".status-badge--processing"),
      ).toBeInTheDocument();
    });
    expect(screen.getByText("Processing")).toBeInTheDocument();
  });

  it("shows processing overlay on thumbnail when processing", async () => {
    setupDefaultMocks({ video: makeVideo({ status: "processing" }) });
    const { container } = renderVideoDetail("v1");
    await waitFor(() => {
      expect(
        container.querySelector(".hero-processing-overlay"),
      ).toBeInTheDocument();
    });
    expect(screen.getByText("Processing video...")).toBeInTheDocument();
  });

  it("disables copy link button when processing", async () => {
    setupDefaultMocks({ video: makeVideo({ status: "processing" }) });
    renderVideoDetail("v1");
    await waitFor(() => {
      const btn = screen.getByRole("button", { name: /copy share link/i });
      expect(btn).toBeDisabled();
    });
  });

  it("hides download button when processing", async () => {
    setupDefaultMocks({ video: makeVideo({ status: "processing" }) });
    renderVideoDetail("v1");
    await waitFor(() => {
      expect(screen.getByText("My Recording")).toBeInTheDocument();
    });
    expect(screen.queryByText("Download")).not.toBeInTheDocument();
  });

  it("shows placeholder for share link when processing", async () => {
    setupDefaultMocks({ video: makeVideo({ status: "processing" }) });
    renderVideoDetail("v1");
    await waitFor(() => {
      expect(
        screen.getByText("Available once processing completes"),
      ).toBeInTheDocument();
    });
  });

  it("disables trim button when processing", async () => {
    setupDefaultMocks({ video: makeVideo({ status: "processing" }) });
    renderVideoDetail("v1");
    await waitFor(() => {
      const btn = screen.getByRole("button", { name: /trim/i });
      expect(btn).toBeDisabled();
    });
  });

  // ─── Video error / placeholder tests ─────────────────────────

  it("shows error overlay when video fails to load", async () => {
    setupDefaultMocks();
    renderVideoDetail("v1");

    const videoEl = await screen.findByRole("generic", { hidden: true }).catch(
      () => null,
    );
    // Wait for the video element to render
    await waitFor(() => {
      expect(document.querySelector("video")).toBeInTheDocument();
    });

    const video = document.querySelector("video")!;
    fireEvent.error(video);

    await waitFor(() => {
      expect(screen.getByText("Video failed to load")).toBeInTheDocument();
    });
    expect(document.querySelector("video")).not.toBeInTheDocument();
  });

  it("shows placeholder when no thumbnail and no video URL", async () => {
    mockApiFetch
      .mockResolvedValueOnce([makeVideo({ thumbnailUrl: undefined })])
      .mockResolvedValueOnce(defaultLimits)
      .mockResolvedValueOnce(defaultFolders)
      .mockResolvedValueOnce(defaultTags)
      .mockResolvedValueOnce(defaultPlaylists)
      .mockRejectedValueOnce(new Error("no download"))
      .mockResolvedValueOnce(defaultComments);

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
        "My Recording",
      );
    });
    expect(
      document.querySelector(".video-thumbnail-placeholder"),
    ).toBeInTheDocument();
    expect(document.querySelector("video")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("img", { name: /thumbnail/i }),
    ).not.toBeInTheDocument();
  });

  it("does not show placeholder when thumbnail exists", async () => {
    mockApiFetch
      .mockResolvedValueOnce([makeVideo()])
      .mockResolvedValueOnce(defaultLimits)
      .mockResolvedValueOnce(defaultFolders)
      .mockResolvedValueOnce(defaultTags)
      .mockResolvedValueOnce(defaultPlaylists)
      .mockRejectedValueOnce(new Error("no download"))
      .mockResolvedValueOnce(defaultComments);

    renderVideoDetail("v1");

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
        "My Recording",
      );
    });
    expect(screen.getByRole("img", { name: /thumbnail/i })).toBeInTheDocument();
    expect(
      document.querySelector(".video-thumbnail-placeholder"),
    ).not.toBeInTheDocument();
  });

  it("polls video status when processing", async () => {
    setupDefaultMocks({ video: makeVideo({ status: "processing" }) });
    renderVideoDetail("v1");
    await waitFor(() => {
      expect(screen.getByText("Processing")).toBeInTheDocument();
    });
    mockApiFetch.mockClear();
    mockApiFetch.mockResolvedValue([makeVideo({ status: "ready" })]);
    await waitFor(
      () => {
        expect(mockApiFetch).toHaveBeenCalledWith("/api/videos");
      },
      { timeout: 4000 },
    );
  }, 10000);
});
