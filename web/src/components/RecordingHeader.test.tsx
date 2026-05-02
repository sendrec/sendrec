import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RecordingHeader } from "./RecordingHeader";

const baseProps = {
  duration: 12,
  isPaused: false,
  remaining: null as number | null,
  drawMode: false,
  drawColor: "#ff0000",
  lineWidth: 4,
  onPause: vi.fn(),
  onResume: vi.fn(),
  onStop: vi.fn(),
  onToggleDraw: vi.fn(),
  onClearCanvas: vi.fn(),
  onSetDrawColor: vi.fn(),
  onSetLineWidth: vi.fn(),
};

describe("RecordingHeader", () => {
  it("renders timer and Pause + Stop while active", () => {
    render(<RecordingHeader {...baseProps} />);
    expect(screen.getByText("0:12")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Pause recording" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Stop recording" })).toBeInTheDocument();
  });

  it("renders Resume when paused", () => {
    render(<RecordingHeader {...baseProps} isPaused />);
    expect(screen.getByRole("button", { name: "Resume recording" })).toBeInTheDocument();
    expect(screen.getByText("(Paused)")).toBeInTheDocument();
  });

  it("renders remaining time when provided and not paused", () => {
    render(<RecordingHeader {...baseProps} remaining={45} />);
    expect(screen.getByText("(0:45 remaining)")).toBeInTheDocument();
  });

  it("invokes onStop when Stop is clicked", async () => {
    const onStop = vi.fn();
    render(<RecordingHeader {...baseProps} onStop={onStop} />);
    await userEvent.click(screen.getByRole("button", { name: "Stop recording" }));
    expect(onStop).toHaveBeenCalledTimes(1);
  });

  it("shows draw color picker, clear, and thickness when drawMode is on", () => {
    render(<RecordingHeader {...baseProps} drawMode />);
    expect(screen.getByLabelText("Drawing color")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Clear drawing" })).toBeInTheDocument();
    expect(screen.getByTestId("thickness-selector")).toBeInTheDocument();
  });
});
