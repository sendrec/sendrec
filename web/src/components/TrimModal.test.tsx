import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TrimModal } from "./TrimModal";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

const defaultProps = {
  videoId: "v1",
  shareToken: "abc123",
  duration: 120,
  onClose: vi.fn(),
  onTrimStarted: vi.fn(),
};

describe("TrimModal", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    defaultProps.onClose.mockReset();
    defaultProps.onTrimStarted.mockReset();
    // First call fetches the video download URL
    mockApiFetch.mockResolvedValueOnce({ downloadUrl: "https://s3.example.com/video.webm" });
  });

  it("renders modal with video player and trim controls", async () => {
    const { container } = render(<TrimModal {...defaultProps} />);

    expect(screen.getByText("Trim Video")).toBeInTheDocument();

    await waitFor(() => {
      const video = container.querySelector("video");
      expect(video).not.toBeNull();
      expect(video).toHaveAttribute("src", "https://s3.example.com/video.webm");
    });

    expect(screen.getByRole("button", { name: "Trim" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
  });

  it("fetches video URL on mount", async () => {
    render(<TrimModal {...defaultProps} />);

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/download");
    });
  });

  it("shows time labels", async () => {
    render(<TrimModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/Start: 0:00/)).toBeInTheDocument();
      expect(screen.getByText(/End: 2:00/)).toBeInTheDocument();
      expect(screen.getByText(/Duration: 2:00/)).toBeInTheDocument();
    });
  });

  it("calls onClose when Cancel is clicked", async () => {
    const user = userEvent.setup();
    render(<TrimModal {...defaultProps} />);
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(defaultProps.onClose).toHaveBeenCalled();
  });

  it("sends trim request on Trim click", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce(undefined);
    render(<TrimModal {...defaultProps} />);
    await user.click(screen.getByRole("button", { name: "Trim" }));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/trim", {
        method: "POST",
        body: expect.any(String),
      });
    });
  });

  it("calls onTrimStarted after successful trim request", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce(undefined);
    render(<TrimModal {...defaultProps} />);
    await user.click(screen.getByRole("button", { name: "Trim" }));

    await waitFor(() => {
      expect(defaultProps.onTrimStarted).toHaveBeenCalled();
    });
  });

  it("shows error on failure", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockRejectedValueOnce(new Error("trim failed"));
    render(<TrimModal {...defaultProps} />);
    await user.click(screen.getByRole("button", { name: "Trim" }));

    await waitFor(() => {
      expect(screen.getByText(/trim failed/i)).toBeInTheDocument();
    });
  });

  it("renders timeline track", async () => {
    const { container } = render(<TrimModal {...defaultProps} />);

    await waitFor(() => {
      expect(container.querySelector("[data-testid='trim-track']")).not.toBeNull();
      expect(container.querySelector("[data-testid='trim-handle-start']")).not.toBeNull();
      expect(container.querySelector("[data-testid='trim-handle-end']")).not.toBeNull();
    });
  });

  it("closes modal on Escape key press", async () => {
    const user = userEvent.setup();
    render(<TrimModal {...defaultProps} />);

    await user.keyboard("{Escape}");

    expect(defaultProps.onClose).toHaveBeenCalled();
  });

  it("shows error when trim API returns non-Error value", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockRejectedValueOnce("unexpected failure");
    render(<TrimModal {...defaultProps} />);
    await user.click(screen.getByRole("button", { name: "Trim" }));

    await waitFor(() => {
      expect(screen.getByText("Trim failed")).toBeInTheDocument();
    });
  });

  it("disables trim button during loading", async () => {
    const user = userEvent.setup();
    let resolveTrim: () => void;
    mockApiFetch.mockReturnValueOnce(
      new Promise<void>((resolve) => {
        resolveTrim = resolve;
      })
    );
    render(<TrimModal {...defaultProps} />);

    await user.click(screen.getByRole("button", { name: "Trim" }));

    await waitFor(() => {
      const button = screen.getByRole("button", { name: "Trimming..." });
      expect(button).toBeDisabled();
    });

    resolveTrim!();

    await waitFor(() => {
      expect(defaultProps.onTrimStarted).toHaveBeenCalled();
    });
  });

  it("shows video duration after video loads", async () => {
    const { container } = render(<TrimModal {...defaultProps} />);

    await waitFor(() => {
      const video = container.querySelector("video");
      expect(video).not.toBeNull();
      expect(video).toHaveAttribute("src", "https://s3.example.com/video.webm");
    });

    expect(screen.getByText(/Duration: 2:00/)).toBeInTheDocument();
    expect(screen.getByText(/Start: 0:00/)).toBeInTheDocument();
    expect(screen.getByText(/End: 2:00/)).toBeInTheDocument();
  });

  it("closes when clicking backdrop", async () => {
    const { container } = render(<TrimModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("Trim Video")).toBeInTheDocument();
    });

    // Click the outer backdrop div (position: fixed)
    const backdrop = container.firstElementChild as HTMLElement;
    fireEvent.click(backdrop, { target: backdrop, currentTarget: backdrop });

    expect(defaultProps.onClose).toHaveBeenCalled();
  });

  it("does not close when clicking modal content", async () => {
    render(<TrimModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("Trim Video")).toBeInTheDocument();
    });

    // Click the inner modal heading (not the backdrop)
    fireEvent.click(screen.getByText("Trim Video"));

    expect(defaultProps.onClose).not.toHaveBeenCalled();
  });

  it("updates start time when dragging start handle", async () => {
    render(<TrimModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/Start: 0:00/)).toBeInTheDocument();
    });

    const startHandle = screen.getByTestId("trim-handle-start");
    const track = screen.getByTestId("trim-track");

    // Mock getBoundingClientRect on the track element
    vi.spyOn(track, "getBoundingClientRect").mockReturnValue({
      left: 0,
      right: 200,
      width: 200,
      top: 0,
      bottom: 32,
      height: 32,
      x: 0,
      y: 0,
      toJSON: () => {},
    });

    // mousedown on start handle
    fireEvent.mouseDown(startHandle);

    // mousemove on document at x=100 (50% of 200px track = 60s of 120s duration)
    fireEvent.mouseMove(document, { clientX: 100 });

    await waitFor(() => {
      expect(screen.getByText(/Start: 1:00/)).toBeInTheDocument();
    });

    // mouseup to stop dragging
    fireEvent.mouseUp(document);
  });

  it("updates end time when dragging end handle", async () => {
    render(<TrimModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/End: 2:00/)).toBeInTheDocument();
    });

    const endHandle = screen.getByTestId("trim-handle-end");
    const track = screen.getByTestId("trim-track");

    vi.spyOn(track, "getBoundingClientRect").mockReturnValue({
      left: 0,
      right: 200,
      width: 200,
      top: 0,
      bottom: 32,
      height: 32,
      x: 0,
      y: 0,
      toJSON: () => {},
    });

    // mousedown on end handle
    fireEvent.mouseDown(endHandle);

    // mousemove to x=100 (50% = 60s)
    fireEvent.mouseMove(document, { clientX: 100 });

    await waitFor(() => {
      expect(screen.getByText(/End: 1:00/)).toBeInTheDocument();
    });

    fireEvent.mouseUp(document);
  });

  it("stops dragging on mouseup", async () => {
    render(<TrimModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/Start: 0:00/)).toBeInTheDocument();
    });

    const startHandle = screen.getByTestId("trim-handle-start");
    const track = screen.getByTestId("trim-track");

    vi.spyOn(track, "getBoundingClientRect").mockReturnValue({
      left: 0,
      right: 200,
      width: 200,
      top: 0,
      bottom: 32,
      height: 32,
      x: 0,
      y: 0,
      toJSON: () => {},
    });

    // Start dragging
    fireEvent.mouseDown(startHandle);
    fireEvent.mouseMove(document, { clientX: 100 });

    await waitFor(() => {
      expect(screen.getByText(/Start: 1:00/)).toBeInTheDocument();
    });

    // Stop dragging
    fireEvent.mouseUp(document);

    // Move again after mouseup - should NOT change start time
    fireEvent.mouseMove(document, { clientX: 150 });

    // Start time should remain at 1:00, not change to 1:30
    await waitFor(() => {
      expect(screen.getByText(/Start: 1:00/)).toBeInTheDocument();
    });
  });

  it("moves nearer handle on track click", async () => {
    render(<TrimModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(/Start: 0:00/)).toBeInTheDocument();
      expect(screen.getByText(/End: 2:00/)).toBeInTheDocument();
    });

    const track = screen.getByTestId("trim-track");

    vi.spyOn(track, "getBoundingClientRect").mockReturnValue({
      left: 0,
      right: 200,
      width: 200,
      top: 0,
      bottom: 32,
      height: 32,
      x: 0,
      y: 0,
      toJSON: () => {},
    });

    // Click near the start (x=20 = 10% = 12s, closer to start at 0 than end at 120)
    fireEvent.click(track, { clientX: 20, target: track, currentTarget: track });

    await waitFor(() => {
      expect(screen.getByText(/Start: 0:12/)).toBeInTheDocument();
    });
  });
});
