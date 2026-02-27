import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SilenceRemovalModal } from "./SilenceRemovalModal";

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

describe("SilenceRemovalModal", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    defaultProps.onClose = vi.fn();
    defaultProps.onRemovalStarted = vi.fn();
  });

  it("has dialog role", () => {
    mockApiFetch.mockReturnValueOnce(new Promise(() => {}));
    render(<SilenceRemovalModal {...defaultProps} />);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("shows loading state while detecting", () => {
    mockApiFetch.mockReturnValueOnce(new Promise(() => {}));

    render(<SilenceRemovalModal {...defaultProps} />);

    expect(screen.getByText("Detecting silence...")).toBeInTheDocument();
  });

  it("displays detected silence segments", async () => {
    mockApiFetch.mockResolvedValueOnce({
      segments: [
        { start: 10.5, end: 13.2 },
        { start: 45.0, end: 48.8 },
      ],
    });

    render(<SilenceRemovalModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/Found 2 silent pauses/)).toBeInTheDocument();
    });

    expect(screen.getByText("[0:10 \u2013 0:13]")).toBeInTheDocument();
    expect(screen.getByText("[0:45 \u2013 0:48]")).toBeInTheDocument();
    expect(screen.getByText("2.7s")).toBeInTheDocument();
    expect(screen.getByText("3.8s")).toBeInTheDocument();
  });

  it("shows no-silence message when none detected", async () => {
    mockApiFetch.mockResolvedValueOnce({ segments: [] });

    render(<SilenceRemovalModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("No silent pauses detected.")).toBeInTheDocument();
    });
  });

  it("calls detect-silence API with default parameters", async () => {
    mockApiFetch.mockResolvedValueOnce({ segments: [] });

    render(<SilenceRemovalModal {...defaultProps} />);

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/detect-silence", {
        method: "POST",
        body: JSON.stringify({ noiseDB: -30, minDuration: 1.0 }),
      });
    });
  });

  it("calls remove-segments API with selected segments", async () => {
    const user = userEvent.setup();

    mockApiFetch
      .mockResolvedValueOnce({
        segments: [
          { start: 10.5, end: 13.2 },
          { start: 45.0, end: 48.8 },
        ],
      })
      .mockResolvedValueOnce(undefined);

    render(<SilenceRemovalModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/Found 2 silent pauses/)).toBeInTheDocument();
    });

    const removeButton = screen.getByRole("button", { name: /Remove 2 pauses/ });
    await user.click(removeButton);

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/remove-segments", {
        method: "POST",
        body: JSON.stringify({
          segments: [
            { start: 10.5, end: 13.2 },
            { start: 45.0, end: 48.8 },
          ],
        }),
      });
    });

    expect(defaultProps.onRemovalStarted).toHaveBeenCalled();
  });

  it("disables remove button when no pauses checked", async () => {
    const user = userEvent.setup();

    mockApiFetch.mockResolvedValueOnce({
      segments: [{ start: 10.5, end: 13.2 }],
    });

    render(<SilenceRemovalModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/Found 1 silent pause\b/)).toBeInTheDocument();
    });

    const checkbox = screen.getByRole("checkbox");
    await user.click(checkbox);

    const removeButton = screen.getByRole("button", { name: /Remove 0 pauses/ });
    expect(removeButton).toBeDisabled();
  });

  it("shows error state when detection fails", async () => {
    mockApiFetch.mockRejectedValueOnce(new Error("Network error"));

    render(<SilenceRemovalModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("Failed to detect silence.")).toBeInTheDocument();
    });
  });

  it("closes on Escape key", async () => {
    const user = userEvent.setup();

    mockApiFetch.mockReturnValueOnce(new Promise(() => {}));

    render(<SilenceRemovalModal {...defaultProps} />);

    await user.keyboard("{Escape}");

    expect(defaultProps.onClose).toHaveBeenCalled();
  });
});
