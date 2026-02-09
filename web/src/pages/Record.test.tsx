import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
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

  it("shows remaining videos count when limits active", async () => {
    mockApiFetch.mockResolvedValueOnce({
      maxVideosPerMonth: 25,
      maxVideoDurationSeconds: 300,
      videosUsedThisMonth: 20,
    });
    renderRecord();

    await waitFor(() => {
      expect(screen.getByText(/5 videos remaining/i)).toBeInTheDocument();
    });
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
    capturedOnRecordingComplete!(screenBlob, 60, webcamBlob);

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
});
