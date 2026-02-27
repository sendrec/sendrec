import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FillerRemovalModal } from "./FillerRemovalModal";
import { expectNoA11yViolations } from "../test-utils/a11y";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

const defaultProps = {
  videoId: "v1",
  shareToken: "abc123",
  duration: 120,
  onClose: vi.fn(),
  onRemovalStarted: vi.fn(),
};

function makeMockWatchResponse(segments: { start: number; end: number; text: string }[]) {
  return {
    title: "Test Video",
    videoUrl: "https://example.com/video.mp4",
    duration: 120,
    creator: "Test User",
    createdAt: "2026-01-01T00:00:00Z",
    contentType: "video/mp4",
    transcriptStatus: "ready",
    segments,
    branding: {},
    summaryStatus: "none",
  };
}

describe("FillerRemovalModal", () => {
  let originalFetch: typeof globalThis.fetch;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
    mockApiFetch.mockReset();
    defaultProps.onClose = vi.fn();
    defaultProps.onRemovalStarted = vi.fn();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it("has dialog role", () => {
    globalThis.fetch = vi.fn().mockReturnValueOnce(new Promise(() => {}));
    render(<FillerRemovalModal {...defaultProps} />);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("detects filler-only segments", async () => {
    const segments = [
      { start: 0, end: 3, text: "Hello everyone, welcome." },
      { start: 3, end: 4.5, text: " Um" },
      { start: 4.5, end: 8, text: " Today we're going to talk about the product." },
      { start: 8, end: 9.2, text: " Uh" },
      { start: 9.2, end: 15, text: " So the first feature is..." },
    ];

    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(makeMockWatchResponse(segments)),
    });

    render(<FillerRemovalModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/Found 2 filler words/)).toBeInTheDocument();
    });
  });

  it("does not flag segments with mixed content", async () => {
    const segments = [
      { start: 0, end: 3, text: "Hello everyone, welcome." },
      { start: 3, end: 6, text: " Um, I think the product is good" },
      { start: 6, end: 10, text: " The second feature is great." },
    ];

    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(makeMockWatchResponse(segments)),
    });

    render(<FillerRemovalModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/No filler words detected/)).toBeInTheDocument();
    });
  });

  it("shows no-fillers message when none detected", async () => {
    const segments = [
      { start: 0, end: 5, text: "Hello everyone, welcome to the demo." },
      { start: 5, end: 10, text: " Today we will discuss features." },
      { start: 10, end: 15, text: " Thank you for watching." },
    ];

    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(makeMockWatchResponse(segments)),
    });

    render(<FillerRemovalModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/No filler words detected/)).toBeInTheDocument();
    });
  });

  it("calls remove-segments API with correct segment times", async () => {
    const user = userEvent.setup();
    const segments = [
      { start: 0, end: 3, text: "Hello everyone, welcome." },
      { start: 3, end: 4.5, text: " Um" },
      { start: 4.5, end: 8, text: " Today we're going to talk about the product." },
      { start: 8, end: 9.2, text: " Uh" },
      { start: 9.2, end: 15, text: " So the first feature is..." },
    ];

    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(makeMockWatchResponse(segments)),
    });

    mockApiFetch.mockResolvedValueOnce(undefined); // remove-segments response

    render(<FillerRemovalModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/Found 2 filler words/)).toBeInTheDocument();
    });

    // Both fillers should be checked by default, click Remove
    const removeButton = screen.getByRole("button", { name: /Remove 2 fillers/ });
    await user.click(removeButton);

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/remove-segments", {
        method: "POST",
        body: JSON.stringify({
          segments: [
            { start: 3, end: 4.5 },
            { start: 8, end: 9.2 },
          ],
        }),
      });
    });

    expect(defaultProps.onRemovalStarted).toHaveBeenCalled();
  });

  it("shows loading state before transcript loads", () => {
    globalThis.fetch = vi.fn().mockReturnValueOnce(new Promise(() => {}));

    render(<FillerRemovalModal {...defaultProps} />);

    expect(screen.getByText("Loading transcript...")).toBeInTheDocument();
  });

  it("disables remove button when no fillers are checked", async () => {
    const user = userEvent.setup();
    const segments = [
      { start: 0, end: 3, text: "Hello everyone." },
      { start: 3, end: 4.5, text: " Um" },
    ];

    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(makeMockWatchResponse(segments)),
    });

    render(<FillerRemovalModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/Found 1 filler word\b/)).toBeInTheDocument();
    });

    // Uncheck the filler by clicking the checkbox
    const checkbox = screen.getByRole("checkbox");
    await user.click(checkbox);

    const removeButton = screen.getByRole("button", { name: /Remove 0 fillers/ });
    expect(removeButton).toBeDisabled();
  });

  it("has no accessibility violations", async () => {
    globalThis.fetch = vi.fn().mockReturnValueOnce(new Promise(() => {}));
    const { container } = render(<FillerRemovalModal {...defaultProps} />);
    await expectNoA11yViolations(container);
  });
});
