import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { CameraRecorder } from "./CameraRecorder";

// Polyfill MediaStream for jsdom
class MockMediaStream {
  id = "mock-stream";
  addTrack = vi.fn();
  getVideoTracks = vi.fn().mockReturnValue([]);
  getAudioTracks = vi.fn().mockReturnValue([]);
  getTracks = vi.fn().mockReturnValue([{ stop: vi.fn() }]);
}
globalThis.MediaStream = MockMediaStream as unknown as typeof MediaStream;

class MockMediaRecorder {
  state = "inactive";
  ondataavailable: ((event: { data: Blob }) => void) | null = null;
  onstop: (() => void) | null = null;
  start = vi.fn().mockImplementation(() => {
    this.state = "recording";
    setTimeout(() => {
      this.ondataavailable?.({ data: new Blob(["chunk"], { type: "video/webm" }) });
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

    expect(screen.queryByRole("button", { name: "Start recording" })).not.toBeInTheDocument();
  });

  it("shows resume button when paused", async () => {
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
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
    await user.click(screen.getByRole("button", { name: "Pause recording" }));
    await user.click(screen.getByRole("button", { name: "Resume recording" }));

    expect(screen.getByRole("button", { name: "Pause recording" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Resume recording" })).not.toBeInTheDocument();
  });

  it("calls onRecordingComplete when recording stops", async () => {
    const onComplete = vi.fn();
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={onComplete} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    await vi.waitFor(() => {
      expect(onComplete).toHaveBeenCalledTimes(1);
    });
    expect(onComplete).toHaveBeenCalledWith(
      expect.any(Blob),
      expect.any(Number),
    );
  });

  it("shows remaining time during recording when maxDuration is set", async () => {
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={vi.fn()} maxDurationSeconds={120} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));

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

    render(<CameraRecorder onRecordingComplete={vi.fn()} />);

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

    expect(screen.getByRole("button", { name: "Flip camera" })).toBeDisabled();
  });

  it("uses mp4 mimeType when webm is not supported", async () => {
    MockMediaRecorder.isTypeSupported = vi.fn().mockImplementation((type: string) => {
      return !type.includes("webm");
    });

    const onComplete = vi.fn();
    const user = userEvent.setup();
    render(<CameraRecorder onRecordingComplete={onComplete} />);

    await vi.waitFor(() => {
      expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    await vi.waitFor(() => {
      expect(onComplete).toHaveBeenCalledTimes(1);
    });

    const blob = onComplete.mock.calls[0][0] as Blob;
    expect(blob.type).toBe("video/mp4");
  });
});
