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
  });

  it("renders modal with trim controls", () => {
    render(<TrimModal {...defaultProps} />);
    expect(screen.getByText("Trim Video")).toBeInTheDocument();
    expect(screen.getByText("Start")).toBeInTheDocument();
    expect(screen.getByText("End")).toBeInTheDocument();
    expect(screen.getAllByRole("slider")).toHaveLength(2);
    expect(screen.getByRole("button", { name: "Trim" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
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
});
