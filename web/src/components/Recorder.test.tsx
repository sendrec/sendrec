import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { Recorder } from "./Recorder";

// Polyfill MediaStream for jsdom
class MockMediaStream {
  addTrack = vi.fn();
  getVideoTracks = vi.fn().mockReturnValue([]);
  getAudioTracks = vi.fn().mockReturnValue([]);
  getTracks = vi.fn().mockReturnValue([]);
}
globalThis.MediaStream = MockMediaStream as unknown as typeof MediaStream;

// Mock useDrawingCanvas
const mockToggleDrawMode = vi.fn();
const mockSetDrawColor = vi.fn();
const mockClearCanvas = vi.fn();
let mockDrawMode = false;

vi.mock("../hooks/useDrawingCanvas", () => ({
  useDrawingCanvas: () => ({
    drawMode: mockDrawMode,
    drawColor: "#ff0000",
    toggleDrawMode: mockToggleDrawMode,
    setDrawColor: mockSetDrawColor,
    clearCanvas: mockClearCanvas,
    handlePointerDown: vi.fn(),
    handlePointerMove: vi.fn(),
    handlePointerUp: vi.fn(),
    handlePointerLeave: vi.fn(),
  }),
}));

// Mock useCanvasCompositing
const mockStartCompositing = vi.fn();
const mockStopCompositing = vi.fn();
const mockGetCompositedStream = vi
  .fn()
  .mockReturnValue(new MockMediaStream());

vi.mock("../hooks/useCanvasCompositing", () => ({
  useCanvasCompositing: () => ({
    startCompositing: mockStartCompositing,
    stopCompositing: mockStopCompositing,
    getCompositedStream: mockGetCompositedStream,
  }),
}));

// Mock browser media APIs
const mockScreenStream = {
  getVideoTracks: vi.fn().mockReturnValue([
    {
      getSettings: () => ({ width: 1920, height: 1080 }),
      addEventListener: vi.fn(),
      stop: vi.fn(),
    },
  ]),
  getAudioTracks: vi.fn().mockReturnValue([]),
  getTracks: vi.fn().mockReturnValue([{ stop: vi.fn() }]),
};

class MockMediaRecorder {
  state = "inactive";
  ondataavailable: ((event: { data: Blob }) => void) | null = null;
  onstop: (() => void) | null = null;
  start = vi.fn().mockImplementation(() => {
    this.state = "recording";
    // Simulate a data chunk being available after a microtask
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
}

beforeEach(() => {
  mockDrawMode = false;
  vi.clearAllMocks();

  Object.defineProperty(globalThis.navigator, "mediaDevices", {
    value: {
      getDisplayMedia: vi.fn().mockResolvedValue(mockScreenStream),
      getUserMedia: vi.fn().mockResolvedValue({
        getTracks: () => [{ stop: vi.fn() }],
      }),
    },
    writable: true,
    configurable: true,
  });

  globalThis.MediaRecorder = MockMediaRecorder as unknown as typeof MediaRecorder;

  // Mock HTMLCanvasElement.captureStream
  HTMLCanvasElement.prototype.captureStream = vi
    .fn()
    .mockReturnValue(new MockMediaStream());

  // Mock HTMLVideoElement.play
  HTMLVideoElement.prototype.play = vi.fn().mockResolvedValue(undefined);
});

describe("Recorder", () => {
  it("renders Start Recording button in idle state", () => {
    render(<Recorder onRecordingComplete={vi.fn()} />);
    expect(
      screen.getByRole("button", { name: "Start recording" }),
    ).toBeInTheDocument();
  });

  it("shows Draw button during recording", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    expect(
      screen.getByRole("button", { name: /drawing/i }),
    ).toBeInTheDocument();
  });

  it("shows screen preview video during recording", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    expect(screen.getByTestId("screen-preview")).toBeInTheDocument();
  });

  it("shows drawing canvas during recording", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    expect(screen.getByTestId("drawing-canvas")).toBeInTheDocument();
  });

  it("drawing canvas has touch-action none", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    const canvas = screen.getByTestId("drawing-canvas");
    expect(canvas.style.touchAction).toBe("none");
  });

  it("has hidden compositing canvas during recording", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    const canvas = screen.getByTestId("compositing-canvas");
    expect(canvas.style.display).toBe("none");
  });

  it("shows Pause and Stop buttons during recording", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    expect(
      screen.getByRole("button", { name: "Pause recording" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Stop recording" }),
    ).toBeInTheDocument();
  });

  it("calls toggleDrawMode when Draw button is clicked", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: /drawing/i }));

    expect(mockToggleDrawMode).toHaveBeenCalledTimes(1);
  });

  it("shows color picker when draw mode is active", async () => {
    mockDrawMode = true;
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    expect(screen.getByTestId("color-picker")).toBeInTheDocument();
  });

  it("shows Clear button when draw mode is active", async () => {
    mockDrawMode = true;
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    expect(
      screen.getByRole("button", { name: "Clear drawing" }),
    ).toBeInTheDocument();
  });

  it("calls clearCanvas when Clear button is clicked", async () => {
    mockDrawMode = true;
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: "Clear drawing" }));

    expect(mockClearCanvas).toHaveBeenCalledTimes(1);
  });

  it("starts compositing when recording starts", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(mockStartCompositing).toHaveBeenCalledTimes(1);
  });

  it("shows max duration message when maxDurationSeconds is provided", () => {
    render(<Recorder onRecordingComplete={vi.fn()} maxDurationSeconds={300} />);
    expect(screen.getByText(/5:00/)).toBeInTheDocument();
  });

  it("does not show max duration message when maxDurationSeconds is zero", () => {
    render(<Recorder onRecordingComplete={vi.fn()} />);
    expect(screen.queryByText(/Maximum recording length/)).not.toBeInTheDocument();
  });

  it("shows Camera Off button in idle state", () => {
    render(<Recorder onRecordingComplete={vi.fn()} />);
    expect(screen.getByText("Camera Off")).toBeInTheDocument();
  });

  it("shows camera toggle button with Enable camera aria-label in idle state", () => {
    render(<Recorder onRecordingComplete={vi.fn()} />);
    expect(
      screen.getByRole("button", { name: "Enable camera" }),
    ).toBeInTheDocument();
  });

  it("calls getDisplayMedia when starting recording", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(navigator.mediaDevices.getDisplayMedia).toHaveBeenCalledTimes(1);
  });

  it("shows Resume button when paused", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: "Pause recording" }));

    expect(
      screen.getByRole("button", { name: "Resume recording" }),
    ).toBeInTheDocument();
  });

  it("hides Pause button when paused", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: "Pause recording" }));

    expect(
      screen.queryByRole("button", { name: "Pause recording" }),
    ).not.toBeInTheDocument();
  });

  it("shows Paused indicator when paused", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: "Pause recording" }));

    expect(screen.getByText("(Paused)")).toBeInTheDocument();
  });

  it("resumes recording when Resume is clicked after pause", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: "Pause recording" }));
    await user.click(screen.getByRole("button", { name: "Resume recording" }));

    expect(
      screen.getByRole("button", { name: "Pause recording" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Resume recording" }),
    ).not.toBeInTheDocument();
  });

  it("transitions to stopped state when Stop is clicked", async () => {
    const onComplete = vi.fn();
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={onComplete} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    // After stop, compositing is cleaned up
    expect(mockStopCompositing).toHaveBeenCalled();
    // The onstop callback fires asynchronously and triggers onRecordingComplete
    await vi.waitFor(() => {
      expect(onComplete).toHaveBeenCalledTimes(1);
    });
  });

  it("calls onRecordingComplete when recording stops", async () => {
    const onComplete = vi.fn();
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={onComplete} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    // The onstop handler fires asynchronously via setTimeout
    await vi.waitFor(() => {
      expect(onComplete).toHaveBeenCalledTimes(1);
    });
    expect(onComplete).toHaveBeenCalledWith(
      expect.any(Blob),
      expect.any(Number),
      undefined,
    );
  });

  it("calls stopCompositing when recording stops", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    mockStopCompositing.mockClear();
    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    expect(mockStopCompositing).toHaveBeenCalledTimes(1);
  });

  it("stops all screen stream tracks when recording stops", async () => {
    const stopTrack = vi.fn();
    mockScreenStream.getTracks.mockReturnValue([{ stop: stopTrack }]);

    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    expect(stopTrack).toHaveBeenCalled();
  });

  it("toggles webcam on when Enable camera is clicked", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Enable camera" }));

    expect(navigator.mediaDevices.getUserMedia).toHaveBeenCalledWith({
      video: { width: 320, height: 240, facingMode: "user" },
      audio: false,
    });
    expect(screen.getByText("Camera On")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Disable camera" }),
    ).toBeInTheDocument();
  });

  it("toggles webcam off when Disable camera is clicked", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Enable camera" }));
    await user.click(screen.getByRole("button", { name: "Disable camera" }));

    expect(screen.getByText("Camera Off")).toBeInTheDocument();
  });

  it("shows alert when webcam access fails", async () => {
    const alertSpy = vi.spyOn(globalThis, "alert").mockImplementation(() => {});
    (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
      new Error("Permission denied"),
    );

    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Enable camera" }));

    expect(alertSpy).toHaveBeenCalledWith(
      "Could not access your camera. Please allow camera access and try again.",
    );
    alertSpy.mockRestore();
  });

  it("shows alert when screen capture fails", async () => {
    const alertSpy = vi.spyOn(globalThis, "alert").mockImplementation(() => {});
    (navigator.mediaDevices.getDisplayMedia as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
      new Error("User cancelled"),
    );

    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(alertSpy).toHaveBeenCalledWith(
      "Screen recording was blocked or failed. Please allow screen capture and try again.",
    );
    alertSpy.mockRestore();
  });

  it("calls setDrawColor when color picker changes", async () => {
    mockDrawMode = true;
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    const colorPicker = screen.getByTestId("color-picker");
    // fireEvent is needed for native input change since userEvent doesn't support color inputs well
    const { fireEvent } = await import("@testing-library/react");
    fireEvent.change(colorPicker, { target: { value: "#00ff00" } });

    expect(mockSetDrawColor).toHaveBeenCalledWith("#00ff00");
  });

  it("shows remaining time during recording when maxDurationSeconds is set", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} maxDurationSeconds={300} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    expect(screen.getByText(/remaining/)).toBeInTheDocument();
  });

  it("stops compositing and streams when stopping from paused state", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: "Pause recording" }));

    mockStopCompositing.mockClear();
    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    expect(mockStopCompositing).toHaveBeenCalledTimes(1);
  });

  it("renders max duration remaining text during recording", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} maxDurationSeconds={120} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    // During recording with maxDuration, remaining time is shown
    expect(screen.getByText(/remaining/)).toBeInTheDocument();
    expect(screen.getByText(/2:00 remaining/)).toBeInTheDocument();
  });

  it("auto-stops recording when max duration is reached", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

    render(<Recorder onRecordingComplete={vi.fn()} maxDurationSeconds={2} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    // Advance timers past the max duration to trigger the auto-stop effect
    await act(() => {
      vi.advanceTimersByTime(3000);
    });

    // stopCompositing should have been called by the auto-stop
    expect(mockStopCompositing).toHaveBeenCalled();

    vi.useRealTimers();
  });

  it("shows webcam preview when camera is enabled in idle state", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Enable camera" }));

    // Webcam preview video should appear (the pip-preview class video)
    const videos = document.querySelectorAll("video.pip-preview");
    expect(videos.length).toBe(1);
  });

  it("sets up webcam recorder when webcam is enabled before recording starts", async () => {
    // Return a proper mock stream from getUserMedia
    const webcamStop = vi.fn();
    const mockWebcamStream = new MockMediaStream();
    mockWebcamStream.getTracks.mockReturnValue([{ stop: webcamStop }]);
    (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockResolvedValue(mockWebcamStream);

    const onComplete = vi.fn();
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={onComplete} />);

    // Enable webcam first
    await user.click(screen.getByRole("button", { name: "Enable camera" }));
    expect(screen.getByText("Camera On")).toBeInTheDocument();

    // Start recording with webcam enabled
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    // Should be in recording state with webcam active
    expect(
      screen.getByRole("button", { name: "Stop recording" }),
    ).toBeInTheDocument();

    // Stop and verify onRecordingComplete is called with webcamBlob
    await user.click(screen.getByRole("button", { name: "Stop recording" }));
    await vi.waitFor(() => {
      expect(onComplete).toHaveBeenCalledTimes(1);
    });
    // Third argument should be the webcam blob
    expect(onComplete).toHaveBeenCalledWith(
      expect.any(Blob),
      expect.any(Number),
      expect.any(Blob),
    );
  });

  it("shows alert when composited stream creation fails", async () => {
    const alertSpy = vi.spyOn(globalThis, "alert").mockImplementation(() => {});
    mockGetCompositedStream.mockReturnValueOnce(null);

    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(alertSpy).toHaveBeenCalledWith(
      "Screen recording was blocked or failed. Please allow screen capture and try again.",
    );
    alertSpy.mockRestore();
  });

  it("stops recording when screen share track ends", async () => {
    // Capture the "ended" event handler
    let endedCallback: (() => void) | undefined;
    const trackWithEndedCapture = {
      getSettings: () => ({ width: 1920, height: 1080 }),
      addEventListener: vi.fn().mockImplementation((event: string, handler: () => void) => {
        if (event === "ended") {
          endedCallback = handler;
        }
      }),
      stop: vi.fn(),
    };
    mockScreenStream.getVideoTracks.mockReturnValue([trackWithEndedCapture]);

    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    expect(endedCallback).toBeDefined();

    // Simulate screen share track ending (user clicks "Stop sharing")
    mockStopCompositing.mockClear();
    act(() => {
      endedCallback!();
    });

    expect(mockStopCompositing).toHaveBeenCalledTimes(1);
  });

  it("shows countdown after clicking start recording", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(screen.getByTestId("countdown-overlay")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
  });

  it("countdown decrements from 3 to 2 to 1", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    await vi.waitFor(() => {
      expect(screen.getByText("3")).toBeInTheDocument();
    });

    act(() => {
      vi.advanceTimersByTime(1000);
    });
    await vi.waitFor(() => {
      expect(screen.getByText("2")).toBeInTheDocument();
    });

    act(() => {
      vi.advanceTimersByTime(1000);
    });
    await vi.waitFor(() => {
      expect(screen.getByText("1")).toBeInTheDocument();
    });

    act(() => {
      vi.advanceTimersByTime(1000);
    });
    // After countdown finishes, overlay should be gone and recording controls visible
    await vi.waitFor(() => {
      expect(screen.queryByTestId("countdown-overlay")).not.toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "Pause recording" })).toBeInTheDocument();

    vi.useRealTimers();
  });

  it("clicking countdown overlay skips to recording", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(screen.getByTestId("countdown-overlay")).toBeInTheDocument();

    await user.click(screen.getByTestId("countdown-overlay"));

    expect(screen.queryByTestId("countdown-overlay")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Pause recording" })).toBeInTheDocument();
  });

  it("shows click to start hint during countdown", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(screen.getByText("Click to start now")).toBeInTheDocument();
  });

  it("hides recording controls during countdown", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    // Countdown overlay should be visible
    expect(screen.getByTestId("countdown-overlay")).toBeInTheDocument();
    // Recording controls should NOT be visible
    expect(screen.queryByRole("button", { name: "Pause recording" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Stop recording" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /drawing/i })).not.toBeInTheDocument();
  });
});
