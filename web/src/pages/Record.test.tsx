import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Record } from "./Record";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

// Mock Recorder to avoid browser API dependencies
vi.mock("../components/Recorder", () => ({
  Recorder: ({ maxDurationSeconds }: { maxDurationSeconds?: number }) => (
    <div data-testid="recorder" data-max-duration={maxDurationSeconds ?? ""}>
      Mock Recorder
    </div>
  ),
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
});
