import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { CameraRecorder } from "./CameraRecorder";
import { expectNoA11yViolations } from "../test-utils/a11y";

// Polyfill MediaStream for jsdom
class MockMediaStream {
  id = "mock-stream";
  addTrack = vi.fn();
  getVideoTracks = vi.fn().mockReturnValue([]);
  getAudioTracks = vi.fn().mockReturnValue([]);
  getTracks = vi.fn().mockReturnValue([{ stop: vi.fn() }]);
}
globalThis.MediaStream = MockMediaStream as unknown as typeof MediaStream;

// A chunk larger than MIN_RECORDING_BYTES (1024) so validation passes
const LARGE_CHUNK = new Blob([new Uint8Array(2048)], { type: "video/webm" });

class MockMediaRecorder {
  state = "inactive";
  ondataavailable: ((event: { data: Blob }) => void) | null = null;
  onstop: (() => void) | null = null;
  start = vi.fn().mockImplementation(() => {
    this.state = "recording";
    setTimeout(() => {
      this.ondataavailable?.({ data: LARGE_CHUNK });
    }, 0);
  });
  stop = vi.fn().mockImplementation(() => {
    this.state = "inactive";
    setTimeout(() => this.onstop?.(), 0);
  });
  pause = vi.fn().mockImplementation(() => {
    this.state = "paused";
  });
  resume = vi.fn().mockImplementation(() => {
    this.state = "recording";
  });

  static isTypeSupported = vi.fn().mockReturnValue(true);
}

let mockStream: MockMediaStream;

beforeEach(() => {
  vi.clearAllMocks();

  mockStream = new MockMediaStream();
  mockStream.getTracks.mockReturnValue([{ stop: vi.fn() }]);

  Object.defineProperty(globalThis.navigator, "mediaDevices", {
    value: {
      getUserMedia: vi.fn().mockResolvedValue(mockStream),
    },
    writable: true,
    configurable: true,
  });

  globalThis.MediaRecorder = MockMediaRecorder as unknown as typeof MediaRecorder;
  HTMLVideoElement.prototype.play = vi.fn().mockResolvedValue(undefined);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("CameraRecorder", () => {
  it("renders start recording button in idle state", async () => {
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });
  });

  it("renders flip camera button", async () => {
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Flip camera" })).toBeInTheDocument();
    });
  });

  it("renders camera preview video element", async () => {
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByTestId("camera-preview")).toBeInTheDocument();
    });
  });

  it("calls getUserMedia on mount with front camera constraints", async () => {
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(navigator.mediaDevices.getUserMedia).toHaveBeenCalledWith({
        video: { facingMode: "user", width: { ideal: 1280 }, height: { ideal: 720 } },
        audio: true,
      });
    });
  });

  it("shows max duration message when maxDurationSeconds is provided", async () => {
    render(<CameraRecorder onRecordingComplete={vi.fn()} maxDurationSeconds={300} />);

    await vi.waitFor(() => {
      expect(screen.getByText(/5:00/)).toBeInTheDocument();
    });
  });

  it("does not show max duration message when maxDurationSeconds is zero", async () => {
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });
    expect(screen.queryByText(/Maximum recording length/)).not.toBeInTheDocument();
  });

  it("mirrors front camera preview with scaleX(-1)", async () => {
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      const preview = screen.getByTestId("camera-preview");
      expect(preview.style.transform).toBe("scaleX(-1)");
    });
  });

  it("flips camera when flip button is clicked", async () => {
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Flip camera" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Flip camera" }));

    await vi.waitFor(() => {
      const calls = (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mock.calls;
      const lastCall = calls[calls.length - 1];
      expect(lastCall[0].video.facingMode).toBe("environment");
    });
  });

  it("removes mirror transform when using back camera", async () => {
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Flip camera" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Flip camera" }));

    await vi.waitFor(() => {
      const preview = screen.getByTestId("camera-preview");
      expect(preview.style.transform).not.toBe("scaleX(-1)");
    });
  });

  it("stops old stream tracks when flipping camera", async () => {
    const stopTrack = vi.fn();
    mockStream.getTracks.mockReturnValue([{ stop: stopTrack }]);

    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Flip camera" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Flip camera" }));

    await vi.waitFor(() => {
      expect(stopTrack).toHaveBeenCalled();
    });
  });

  it("shows pause and stop buttons during recording", async () => {
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    expect(screen.getByRole("button", { name: "Pause recording" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Stop recording" })).toBeInTheDocument();
  });

  it("hides start button during recording", async () => {
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    expect(screen.queryByRole("button", { name: "Start recording" })).not.toBeInTheDocument();
  });

  it("shows resume button when paused", async () => {
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: "Pause recording" }));

    expect(screen.getByRole("button", { name: "Resume recording" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Pause recording" })).not.toBeInTheDocument();
  });

  it("shows paused indicator when paused", async () => {
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: "Pause recording" }));

    expect(screen.getByText("(Paused)")).toBeInTheDocument();
  });

  it("resumes recording when resume is clicked", async () => {
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: "Pause recording" }));
    await user.click(screen.getByRole("button", { name: "Resume recording" }));

    expect(screen.getByRole("button", { name: "Pause recording" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Resume recording" })).not.toBeInTheDocument();
  });

  it("calls onRecordingComplete when recording stops", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const onComplete = vi.fn();
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<CameraRecorder onRecordingComplete={onComplete} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    // Advance past MIN_RECORDING_SECONDS so validation passes
    await act(() => { vi.advanceTimersByTime(1500); });

    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    await vi.waitFor(() => {
      expect(onComplete).toHaveBeenCalledTimes(1);
    });
    expect(onComplete).toHaveBeenCalledWith(
      expect.any(Blob),
      expect.any(Number),
    );
    vi.useRealTimers();
  });

  it("shows remaining time during recording when maxDuration is set", async () => {
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} maxDurationSeconds={120} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    expect(screen.getByText(/2:00 remaining/)).toBeInTheDocument();
  });

  it("auto-stops recording when max duration is reached", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const onComplete = vi.fn();
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

    render(<CameraRecorder onRecordingComplete={onComplete} maxDurationSeconds={2} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    await act(() => {
      vi.advanceTimersByTime(3000);
    });

    await vi.waitFor(() => {
      expect(onComplete).toHaveBeenCalledTimes(1);
    });

    vi.useRealTimers();
  });

  it("shows error message when camera permission is denied", async () => {
    (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
      new Error("Permission denied"),
    );

    await act(async () => {
      render(<CameraRecorder onRecordingComplete={vi.fn()} />);
    });

    await vi.waitFor(() => {
      expect(screen.getByText(/could not access your camera/i)).toBeInTheDocument();
    });
  });

  it("cleans up stream tracks on unmount", async () => {
    const stopTrack = vi.fn();
    mockStream.getTracks.mockReturnValue([{ stop: stopTrack }]);

    const { unmount } = render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByTestId("camera-preview")).toBeInTheDocument();
    });

    unmount();

    expect(stopTrack).toHaveBeenCalled();
  });

  it("disables flip button during recording", async () => {
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    expect(screen.getByRole("button", { name: "Flip camera" })).toBeDisabled();
  });

  it("prefers mp4 mimeType for maximum compatibility", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    // isTypeSupported returns true for everything (default mock)
    MockMediaRecorder.isTypeSupported = vi.fn().mockReturnValue(true);

    const onComplete = vi.fn();
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<CameraRecorder onRecordingComplete={onComplete} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    // Advance past MIN_RECORDING_SECONDS so validation passes
    await act(() => { vi.advanceTimersByTime(1500); });

    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    await vi.waitFor(() => {
      expect(onComplete).toHaveBeenCalledTimes(1);
    });

    const blob = onComplete.mock.calls[0][0] as Blob;
    expect(blob.type).toBe("video/mp4");
    vi.useRealTimers();
  });

  it("falls back to webm when mp4 is not supported", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    MockMediaRecorder.isTypeSupported = vi.fn().mockImplementation((type: string) => {
      return type.includes("webm");
    });

    const onComplete = vi.fn();
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<CameraRecorder onRecordingComplete={onComplete} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    // Advance past MIN_RECORDING_SECONDS so validation passes
    await act(() => { vi.advanceTimersByTime(1500); });

    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    await vi.waitFor(() => {
      expect(onComplete).toHaveBeenCalledTimes(1);
    });

    const blob = onComplete.mock.calls[0][0] as Blob;
    expect(blob.type).toBe("video/webm");
    vi.useRealTimers();
  });

  it("shows countdown after clicking start recording", async () => {
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(screen.getByTestId("countdown-overlay")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
  });

  it("clicking countdown overlay skips to recording", async () => {
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    expect(screen.getByTestId("countdown-overlay")).toBeInTheDocument();

    await user.click(screen.getByTestId("countdown-overlay"));

    expect(screen.queryByTestId("countdown-overlay")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Pause recording" })).toBeInTheDocument();
  });

  it("calls onRecordingError when recording is shorter than 1 second", async () => {
    const onComplete = vi.fn();
    const onError = vi.fn();
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={onComplete} onRecordingError={onError} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    // Stop immediately — elapsed will be 0 seconds and blob will be tiny
    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    await vi.waitFor(() => {
      expect(onError).toHaveBeenCalledWith("Recording too short. Please record for at least 1 second.");
    });
    expect(onComplete).not.toHaveBeenCalled();
  });

  it("shows click to start hint during countdown", async () => {
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(screen.getByText("Click to start now")).toBeInTheDocument();
  });

  it("has no accessibility violations", async () => {
    const { container } = render(<CameraRecorder onRecordingComplete={vi.fn()} />);
    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });
    await expectNoA11yViolations(container);
  });
});
