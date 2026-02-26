import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { act } from "react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Record } from "./Record";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

// Store the onRecordingComplete callback so tests can trigger it
let capturedOnRecordingComplete: ((blob: Blob, duration: number, webcamBlob?: Blob) => void) | null = null;

// Mock Recorder to avoid browser API dependencies
vi.mock("../components/Recorder", () => ({
  Recorder: ({ maxDurationSeconds, onRecordingComplete }: { maxDurationSeconds?: number; onRecordingComplete: (blob: Blob, duration: number, webcamBlob?: Blob) => void }) => {
    capturedOnRecordingComplete = onRecordingComplete;
    return (
      <div data-testid="recorder" data-max-duration={maxDurationSeconds ?? ""}>
        Mock Recorder
      </div>
    );
  },
}));

// Store the CameraRecorder onRecordingComplete callback
let capturedCameraOnRecordingComplete: ((blob: Blob, duration: number) => void) | null = null;

vi.mock("../components/CameraRecorder", () => ({
  CameraRecorder: ({ maxDurationSeconds, onRecordingComplete }: { maxDurationSeconds?: number; onRecordingComplete: (blob: Blob, duration: number) => void }) => {
    capturedCameraOnRecordingComplete = onRecordingComplete;
    return (
      <div data-testid="camera-recorder" data-max-duration={maxDurationSeconds ?? ""}>
        Mock Camera Recorder
      </div>
    );
  },
}));

vi.mock("./Upload", () => ({
  Upload: () => <div data-testid="upload-component">Mock Upload</div>,
}));

function renderRecord() {
  return render(
    <MemoryRouter>
      <Record />
    </MemoryRouter>
  );
}

describe("Record", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    Object.defineProperty(navigator, "mediaDevices", {
      value: { getDisplayMedia: vi.fn(), getUserMedia: vi.fn() },
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows limit reached message when monthly quota is full", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 25,
      maxVideoDurationSeconds: 300,
      videosUsedThisMonth: 25,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByText(/reached your limit of 25 videos/i)).toBeInTheDocument();
    });
    expect(screen.queryByTestId("recorder")).not.toBeInTheDocument();
  });

  it("shows recorder when below monthly limit", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 25,
      maxVideoDurationSeconds: 300,
      videosUsedThisMonth: 10,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });
  });

  it("passes maxDurationSeconds to Recorder", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 25,
      maxVideoDurationSeconds: 300,
      videosUsedThisMonth: 10,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toHaveAttribute("data-max-duration", "300");
    });
  });

  it("shows recorder without duration limit when unlimited", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
      expect(screen.getByTestId("recorder")).toHaveAttribute("data-max-duration", "0");
    });
  });

  it("shows usage progress bar when limits active", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 25,
      maxVideoDurationSeconds: 300,
      videosUsedThisMonth: 20,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByText("20 / 25 videos this month")).toBeInTheDocument();
    });
    const bar = screen.getByRole("progressbar");
    expect(bar).toBeInTheDocument();
    expect(bar).toHaveAttribute("aria-valuenow", "20");
    expect(bar).toHaveAttribute("aria-valuemax", "25");
  });

  it("shows red progress bar at 80%+ usage", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 25,
      maxVideoDurationSeconds: 300,
      videosUsedThisMonth: 22,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByText("22 / 25 videos this month")).toBeInTheDocument();
    });
    const fill = screen.getByRole("progressbar").querySelector(".usage-bar-fill");
    expect(fill).toHaveClass("usage-bar-fill--warning");
  });

  it("hides progress bar for unlimited plan", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 5,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
  });

  it("includes webcamFileSize in create request when webcam blob provided", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    // Mock create response with webcamUploadUrl
    mockApiFetch.mockResolvedValueOnce({
      id: "video-1",
      uploadUrl: "https://s3.example.com/screen",
      shareToken: "abc123",
      webcamUploadUrl: "https://s3.example.com/webcam",
    });

    // Mock the fetch calls for S3 uploads
    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });

    // Mock the PATCH status update
    mockApiFetch.mockResolvedValueOnce(undefined);

    const screenBlob = new Blob(["screen"], { type: "video/webm" });
    const webcamBlob = new Blob(["webcam"], { type: "video/webm" });

    // Trigger recording complete with webcam blob
    await act(async () => {
      capturedOnRecordingComplete!(screenBlob, 60, webcamBlob);
    });

    await waitFor(() => {
      // Verify create request includes webcamFileSize
      const createCall = mockApiFetch.mock.calls.find(
        (call: unknown[]) => call[0] === "/api/videos" && (call[1] as { method: string })?.method === "POST"
      );
      expect(createCall).toBeDefined();
      const body = JSON.parse((createCall![1] as { body: string }).body);
      expect(body.webcamFileSize).toBe(webcamBlob.size);
    });

    // Verify webcam was uploaded
    await waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith(
        "https://s3.example.com/webcam",
        expect.objectContaining({ method: "PUT", body: webcamBlob })
      );
    });

    globalThis.fetch = originalFetch;
  });

  it("shows loading state initially", async () => {
    // Mock limits as a promise that never resolves during this test
    let resolveLimits!: (value: unknown) => void;
    mockApiFetch.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveLimits = resolve;
      })
    );
    renderRecord();

    expect(screen.getByText("Loading...")).toBeInTheDocument();
    expect(screen.queryByTestId("recorder")).not.toBeInTheDocument();

    // Clean up: resolve the promise so React doesn't complain about updates after unmount
    await act(async () => {
      resolveLimits({
        maxVideosPerMonth: 0,
        maxVideoDurationSeconds: 0,
        videosUsedThisMonth: 0,
      });
    });
  });

  it("shows uploading state after recording completes", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    // Mock create to hang so we stay in uploading state
    let resolveCreate!: (value: unknown) => void;
    mockApiFetch.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveCreate = resolve;
      })
    );

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    expect(screen.getByText("Creating video...")).toBeInTheDocument();
    expect(screen.queryByTestId("recorder")).not.toBeInTheDocument();

    // Clean up
    await act(async () => {
      resolveCreate(null);
    });
  });

  it("shows share URL after successful upload", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-1",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "share-abc",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });

    // Mock PATCH status update
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 45);
    });

    await waitFor(() => {
      expect(screen.getByText("Recording complete")).toBeInTheDocument();
    });

    // Share URL should be displayed
    expect(screen.getByText(/\/watch\/share-abc/)).toBeInTheDocument();

    globalThis.fetch = originalFetch;
  });

  it("shows copy link button on share page", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-2",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "token-xyz",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      expect(screen.getByText("Copy link")).toBeInTheDocument();
    });

    globalThis.fetch = originalFetch;
  });

  it("shows watch video link on share page", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-3",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "token-watch",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      const watchLink = screen.getByText("Watch video");
      expect(watchLink).toBeInTheDocument();
      expect(watchLink.closest("a")).toHaveAttribute(
        "href",
        expect.stringContaining("/watch/token-watch")
      );
      expect(watchLink.closest("a")).toHaveAttribute("target", "_blank");
    });

    globalThis.fetch = originalFetch;
  });

  it("shows record another button on share page", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-4",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "token-another",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      expect(screen.getByText("Record another")).toBeInTheDocument();
    });

    globalThis.fetch = originalFetch;
  });

  it("shows go to library link on share page", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-5",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "token-lib",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      const libraryLink = screen.getByText("Go to Library");
      expect(libraryLink).toBeInTheDocument();
      expect(libraryLink.closest("a")).toHaveAttribute("href", "/library");
    });

    globalThis.fetch = originalFetch;
  });

  it("clicking record another resets to recorder", async () => {
    const user = userEvent.setup();

    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-6",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "token-reset",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      expect(screen.getByText("Record another")).toBeInTheDocument();
    });

    await user.click(screen.getByText("Record another"));

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });
    expect(screen.queryByText("Recording complete")).not.toBeInTheDocument();

    globalThis.fetch = originalFetch;
  });

  it("shows error when upload fails", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-fail",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "token-fail",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: false, status: 500 });

    // Mock the DELETE cleanup call
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      expect(screen.getByText("Upload failed")).toBeInTheDocument();
    });

    globalThis.fetch = originalFetch;
  });

  it("shows try again button on error", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-try",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "token-try",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: false, status: 500 });
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      expect(screen.getByText("Try again")).toBeInTheDocument();
    });

    globalThis.fetch = originalFetch;
  });

  it("clicking try again resets to recorder", async () => {
    const user = userEvent.setup();

    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-retry",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "token-retry",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: false, status: 500 });
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      expect(screen.getByText("Try again")).toBeInTheDocument();
    });

    await user.click(screen.getByText("Try again"));

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });
    expect(screen.queryByText("Upload failed")).not.toBeInTheDocument();

    globalThis.fetch = originalFetch;
  });

  it("deletes created video when upload fails", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-cleanup",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "token-cleanup",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: false, status: 500 });

    // Mock the DELETE cleanup
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      expect(screen.getByText("Upload failed")).toBeInTheDocument();
    });

    // Verify DELETE was called for cleanup
    const deleteCall = mockApiFetch.mock.calls.find(
      (call: unknown[]) =>
        call[0] === "/api/videos/video-cleanup" &&
        (call[1] as { method: string })?.method === "DELETE"
    );
    expect(deleteCall).toBeDefined();

    globalThis.fetch = originalFetch;
  });

  it("shows error when create API returns null", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    // Mock create to return null (undefined from apiFetch)
    mockApiFetch.mockResolvedValueOnce(null);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      expect(screen.getByText("Failed to create video")).toBeInTheDocument();
    });
  });

  it("copies share URL to clipboard when copy link is clicked", async () => {
    const user = userEvent.setup();

    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-copy",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "token-copy",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      expect(screen.getByText("Copy link")).toBeInTheDocument();
    });

    // Mock clipboard API
    const writeTextMock = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: writeTextMock },
      writable: true,
      configurable: true,
    });

    await user.click(screen.getByText("Copy link"));

    await waitFor(() => {
      expect(screen.getByText("Copied!")).toBeInTheDocument();
    });

    expect(writeTextMock).toHaveBeenCalledWith(
      expect.stringContaining("/watch/token-copy")
    );

    globalThis.fetch = originalFetch;
  });

  it("creates title with current date and time", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-title",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "token-title",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 45);
    });

    await waitFor(() => {
      const createCall = mockApiFetch.mock.calls.find(
        (call: unknown[]) =>
          call[0] === "/api/videos" &&
          (call[1] as { method: string })?.method === "POST"
      );
      expect(createCall).toBeDefined();
      const body = JSON.parse((createCall![1] as { body: string }).body);
      expect(body.title).toMatch(/^Recording /);
    });

    globalThis.fetch = originalFetch;
  });

  it("sends PUT to upload URL with screen blob", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-put",
      uploadUrl: "https://s3.example.com/screen-upload",
      shareToken: "token-put",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["screen-data"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith(
        "https://s3.example.com/screen-upload",
        expect.objectContaining({
          method: "PUT",
          body: blob,
          headers: { "Content-Type": "video/webm" },
        })
      );
    });

    globalThis.fetch = originalFetch;
  });

  it("sends PATCH status update after successful upload", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-patch",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "token-patch",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      expect(screen.getByText("Recording complete")).toBeInTheDocument();
    });

    const patchCall = mockApiFetch.mock.calls.find(
      (call: unknown[]) =>
        call[0] === "/api/videos/video-patch" &&
        (call[1] as { method: string })?.method === "PATCH"
    );
    expect(patchCall).toBeDefined();
    const patchBody = JSON.parse((patchCall![1] as { body: string }).body);
    expect(patchBody.status).toBe("ready");

    globalThis.fetch = originalFetch;
  });

  it("handles webcam upload failure", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-wcfail",
      uploadUrl: "https://s3.example.com/screen",
      shareToken: "token-wcfail",
      webcamUploadUrl: "https://s3.example.com/webcam",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes("webcam")) {
        return Promise.resolve({ ok: false, status: 500 });
      }
      return Promise.resolve({ ok: true });
    });

    // Mock DELETE cleanup
    mockApiFetch.mockResolvedValueOnce(undefined);

    const screenBlob = new Blob(["screen"], { type: "video/webm" });
    const webcamBlob = new Blob(["webcam"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(screenBlob, 30, webcamBlob);
    });

    await waitFor(() => {
      expect(screen.getByText("Webcam upload failed")).toBeInTheDocument();
    });

    globalThis.fetch = originalFetch;
  });

  it("shows recorder when limits API throws error", async () => {
    mockApiFetch.mockRejectedValueOnce(new Error("Network error"));
    renderRecord();

    // When limits fetch fails, limits is null and loadingLimits becomes false.
    // With null limits, quotaReached is false, so recorder is shown.
    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });
    // No "remaining" text since limits is null
    expect(screen.queryByText(/videos remaining/i)).not.toBeInTheDocument();
  });

  it("shows CameraRecorder when getDisplayMedia is unavailable but getUserMedia is available", async () => {
    Object.defineProperty(navigator, "mediaDevices", {
      value: { getUserMedia: vi.fn() },
      writable: true,
      configurable: true,
    });

    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("camera-recorder")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("recorder")).not.toBeInTheDocument();
  });

  it("passes maxDurationSeconds to CameraRecorder", async () => {
    Object.defineProperty(navigator, "mediaDevices", {
      value: { getUserMedia: vi.fn() },
      writable: true,
      configurable: true,
    });

    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 25,
      maxVideoDurationSeconds: 300,
      videosUsedThisMonth: 10,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("camera-recorder")).toHaveAttribute("data-max-duration", "300");
    });
  });

  it("shows usage progress bar with CameraRecorder", async () => {
    Object.defineProperty(navigator, "mediaDevices", {
      value: { getUserMedia: vi.fn() },
      writable: true,
      configurable: true,
    });

    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 25,
      maxVideoDurationSeconds: 300,
      videosUsedThisMonth: 20,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByText("20 / 25 videos this month")).toBeInTheDocument();
    });
    expect(screen.getByRole("progressbar")).toBeInTheDocument();
  });

  it("shows unsupported message when both APIs are unavailable", async () => {
    Object.defineProperty(navigator, "mediaDevices", {
      value: {},
      writable: true,
      configurable: true,
    });

    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByText(/recording is not available/i)).toBeInTheDocument();
    });
    expect(screen.queryByTestId("recorder")).not.toBeInTheDocument();
    expect(screen.queryByTestId("camera-recorder")).not.toBeInTheDocument();
  });

  it("shows unsupported message when mediaDevices is undefined", async () => {
    Object.defineProperty(navigator, "mediaDevices", {
      value: undefined,
      writable: true,
      configurable: true,
    });

    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByText(/recording is not available/i)).toBeInTheDocument();
    });
    expect(screen.queryByTestId("recorder")).not.toBeInTheDocument();
    expect(screen.queryByTestId("camera-recorder")).not.toBeInTheDocument();
  });

  it("uploads camera recording with correct content type from blob", async () => {
    Object.defineProperty(navigator, "mediaDevices", {
      value: { getUserMedia: vi.fn() },
      writable: true,
      configurable: true,
    });

    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("camera-recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-cam",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "cam-token",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["camera-data"], { type: "video/mp4" });
    await act(async () => {
      capturedCameraOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith(
        "https://s3.example.com/upload",
        expect.objectContaining({
          method: "PUT",
          body: blob,
          headers: { "Content-Type": "video/mp4" },
        }),
      );
    });

    // Verify contentType is sent in create request
    const createCall = mockApiFetch.mock.calls.find(
      (call: unknown[]) => call[0] === "/api/videos" && (call[1] as { method: string })?.method === "POST"
    );
    expect(createCall).toBeDefined();
    const body = JSON.parse((createCall![1] as { body: string }).body);
    expect(body.contentType).toBe("video/mp4");

    globalThis.fetch = originalFetch;
  });

  describe("onboarding empty state", () => {
    it("shows getting started guide when user has no videos", async () => {
      mockApiFetch.mockResolvedValueOnce({
        maxVideosPerMonth: 25,
        maxVideoDurationSeconds: 300,
        videosUsedThisMonth: 0,
      });
      renderRecord();

      expect(await screen.findByText(/get started in 3 steps/i)).toBeInTheDocument();
      expect(screen.getByText(/record your screen/i)).toBeInTheDocument();
      expect(screen.getByText(/share the link/i)).toBeInTheDocument();
      expect(screen.getByText(/track views/i)).toBeInTheDocument();
    });

    it("does not show guide when user has videos", async () => {
      mockApiFetch.mockResolvedValueOnce({
        maxVideosPerMonth: 25,
        maxVideoDurationSeconds: 300,
        videosUsedThisMonth: 5,
      });
      renderRecord();

      await waitFor(() => {
        expect(screen.getByTestId("recorder")).toBeInTheDocument();
      });
      expect(screen.queryByText(/get started in 3 steps/i)).not.toBeInTheDocument();
    });
  });

  it("renders Record and Upload tabs", async () => {
    mockApiFetch.mockResolvedValueOnce({ maxVideosPerMonth: 0, maxVideoDurationSeconds: 0, videosUsedThisMonth: 0 });
    renderRecord();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Record" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Upload" })).toBeInTheDocument();
    });
  });

  it("shows Upload component when Upload tab is clicked", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce({ maxVideosPerMonth: 0, maxVideoDurationSeconds: 0, videosUsedThisMonth: 0 });
    renderRecord();
    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });
    await user.click(screen.getByRole("button", { name: "Upload" }));
    expect(screen.getByTestId("upload-component")).toBeInTheDocument();
    expect(screen.queryByTestId("recorder")).not.toBeInTheDocument();
  });

  it("falls back to execCommand copy when clipboard API fails", async () => {
    const user = userEvent.setup();

    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 0,
      maxVideoDurationSeconds: 0,
      videosUsedThisMonth: 0,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByTestId("recorder")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce({
      id: "video-fallback",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "token-fallback",
    });

    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });
    mockApiFetch.mockResolvedValueOnce(undefined);

    const blob = new Blob(["video"], { type: "video/webm" });
    await act(async () => {
      capturedOnRecordingComplete!(blob, 30);
    });

    await waitFor(() => {
      expect(screen.getByText("Copy link")).toBeInTheDocument();
    });

    // Mock clipboard API to reject so fallback path is taken
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: vi.fn().mockRejectedValue(new Error("Not allowed")) },
      writable: true,
      configurable: true,
    });

    // Define document.execCommand for fallback copy (not available in jsdom by default)
    const execCommandMock = vi.fn().mockReturnValue(true);
    document.execCommand = execCommandMock;

    await user.click(screen.getByText("Copy link"));

    await waitFor(() => {
      expect(screen.getByText("Copied!")).toBeInTheDocument();
    });

    expect(execCommandMock).toHaveBeenCalledWith("copy");

    globalThis.fetch = originalFetch;
  });
});
