import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TrimModal } from "./TrimModal";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

const defaultProps = {
  videoId: "v1",
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
});
