import { StrictMode } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { Recorder } from "./Recorder";
import { expectNoA11yViolations } from "../test-utils/a11y";

// Polyfill MediaStream for jsdom
const mediaStreamArgs: unknown[][] = [];
class MockMediaStream {
  constructor(tracks: unknown[] = []) {
    mediaStreamArgs.push(tracks);
  }
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
    hasDrawing: { current: false },
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

// Mock the annotation overlay: the recorded video track is the track it returns.
const OVERLAY_TRACK = { kind: "video", stop: vi.fn() };
const mockOverlayDrawingOnTrack = vi.fn().mockReturnValue(OVERLAY_TRACK);

let mockCanRecordAnnotations = true;

vi.mock("../utils/drawingOverlay", () => ({
  overlayDrawingOnTrack: (...args: unknown[]) => mockOverlayDrawingOnTrack(...args),
  canRecordAnnotations: () => mockCanRecordAnnotations,
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

// A chunk larger than MIN_RECORDING_BYTES (1024) so validation passes
const LARGE_CHUNK = new Blob([new Uint8Array(2048)], { type: "video/webm" });

const mediaRecorderInstances: MockMediaRecorder[] = [];
const mediaRecorderOptions: ({ mimeType: string } | undefined)[] = [];

class MockMediaRecorder {
  static isTypeSupported = vi.fn().mockReturnValue(true);
  state = "inactive";
  ondataavailable: ((event: { data: Blob }) => void) | null = null;
  onstop: (() => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  constructor(_stream?: unknown, options?: { mimeType: string }) {
    mediaRecorderInstances.push(this);
    mediaRecorderOptions.push(options);
  }

  start = vi.fn().mockImplementation(() => {
    this.state = "recording";
    // Simulate a data chunk being available after a microtask
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
}

// Lets a pending webcam open (or failure) settle.
const flushWebcam = () => act(async () => {});
const webcamPreview = () => document.querySelector("video.pip-preview");

beforeEach(() => {
  mockDrawMode = false;
  mediaRecorderInstances.length = 0;
  mediaRecorderOptions.length = 0;
  mediaStreamArgs.length = 0;
  mockCanRecordAnnotations = true;
  vi.clearAllMocks();
  mockOverlayDrawingOnTrack.mockReturnValue(OVERLAY_TRACK);

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

  it("records the annotated track so drawings end up in the recording", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(mockOverlayDrawingOnTrack).toHaveBeenCalledTimes(1);
    const [track, canvas] = mockOverlayDrawingOnTrack.mock.calls[0];
    expect(track).toBe(mockScreenStream.getVideoTracks()[0]);
    expect(canvas).toBe(screen.getByTestId("drawing-canvas"));
    expect(mediaStreamArgs.at(-1)).toContain(OVERLAY_TRACK);
  });

  it("shows max duration message when maxDurationSeconds is provided", () => {
    render(<Recorder onRecordingComplete={vi.fn()} maxDurationSeconds={300} />);
    expect(screen.getByText(/5:00/)).toBeInTheDocument();
  });

  it("does not show max duration message when maxDurationSeconds is zero", () => {
    render(<Recorder onRecordingComplete={vi.fn()} />);
    expect(screen.queryByText(/Maximum recording length/)).not.toBeInTheDocument();
  });

  it("reports when it leaves and returns to idle", async () => {
    const onIdleChange = vi.fn();
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} onIdleChange={onIdleChange} />);
    expect(onIdleChange).toHaveBeenLastCalledWith(true);

    await user.click(screen.getByRole("button", { name: "Start recording" }));
    expect(onIdleChange).toHaveBeenLastCalledWith(false);
  });

  it("has no camera toggle; the Record page mode picker owns the camera", () => {
    render(<Recorder onRecordingComplete={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /camera/i })).not.toBeInTheDocument();
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
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const onComplete = vi.fn();
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<Recorder onRecordingComplete={onComplete} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    // Advance past MIN_RECORDING_SECONDS so validation passes
    await act(() => { vi.advanceTimersByTime(1500); });

    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    // The onstop callback fires asynchronously and triggers onRecordingComplete
    await vi.waitFor(() => {
      expect(onComplete).toHaveBeenCalledTimes(1);
    });
    vi.useRealTimers();
  });

  it("calls onRecordingComplete when recording stops", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const onComplete = vi.fn();
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<Recorder onRecordingComplete={onComplete} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    // Advance past MIN_RECORDING_SECONDS so validation passes
    await act(() => { vi.advanceTimersByTime(1500); });

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
    vi.useRealTimers();
  });

  it("detaches the preview when recording stops", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    expect(
      (screen.getByTestId("screen-preview") as HTMLVideoElement).srcObject,
    ).toBeNull();
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

  it("opens the webcam when withCamera is set", async () => {
    render(<Recorder withCamera onRecordingComplete={vi.fn()} />);
    await flushWebcam();

    expect(navigator.mediaDevices.getUserMedia).toHaveBeenCalledWith({
      video: { width: 320, height: 240, facingMode: "user" },
      audio: false,
    });
    expect(webcamPreview()).not.toBeNull();
  });

  it("closes the webcam when withCamera is turned off", async () => {
    const stop = vi.fn();
    (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockResolvedValue({
      getTracks: () => [{ stop }],
    });
    const { rerender } = render(<Recorder withCamera onRecordingComplete={vi.fn()} />);
    await flushWebcam();
    rerender(<Recorder onRecordingComplete={vi.fn()} />);

    expect(stop).toHaveBeenCalled();
    expect(webcamPreview()).toBeNull();
  });

  it("shows inline error when webcam access fails", async () => {
    (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
      new Error("Permission denied"),
    );

    const user = userEvent.setup();
    render(<Recorder withCamera onRecordingComplete={vi.fn()} />);
    await flushWebcam();

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("Could not access your camera. Please allow camera access and try again.")).toBeInTheDocument();
  });

  it("shows inline error when screen capture fails", async () => {
    (navigator.mediaDevices.getDisplayMedia as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
      new Error("User cancelled"),
    );

    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("Screen recording was blocked or failed. Please allow screen capture and try again.")).toBeInTheDocument();
  });

  it("blames the audio device when screen capture with audio is not readable", async () => {
    (navigator.mediaDevices.getDisplayMedia as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
      new DOMException("Device in use", "NotReadableError"),
    );

    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(screen.getByText(/system audio device is unavailable or in use/)).toBeInTheDocument();
  });

  it("keeps the generic error for unreadable capture when audio is off", async () => {
    (navigator.mediaDevices.getDisplayMedia as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
      new DOMException("Device in use", "NotReadableError"),
    );

    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Disable system audio" }));
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(screen.getByText("Screen recording was blocked or failed. Please allow screen capture and try again.")).toBeInTheDocument();
  });

  it.each(["NotReadableError", "NotFoundError", "NotAllowedError"])(
    "warns and keeps recording when the microphone fails with %s",
    async (name) => {
      (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
        new DOMException("mic failed", name),
      );

      const user = userEvent.setup();
      render(<Recorder onRecordingComplete={vi.fn()} />);
      await user.click(screen.getByRole("button", { name: "Start recording" }));

      expect(screen.getByTestId("countdown-overlay")).toBeInTheDocument();
      expect(screen.getByText(/Microphone unavailable — recording without your voice/)).toBeInTheDocument();
    },
  );

  it("does not request the microphone or warn when audio is off", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Disable system audio" }));
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(navigator.mediaDevices.getUserMedia).not.toHaveBeenCalled();
    expect(screen.queryByTestId("mic-note")).not.toBeInTheDocument();
  });

  describe("with a working audio mixer", () => {
    beforeEach(() => {
      vi.stubGlobal("AudioContext", class {
        createMediaStreamDestination() {
          return { stream: { getAudioTracks: () => [{ kind: "audio" }] } };
        }
        createMediaStreamSource() {
          return { connect: vi.fn() };
        }
        close() {}
      });
    });
    afterEach(() => {
      vi.unstubAllGlobals();
    });

    it.each([
      ["Voicemeeter Out B1 (VB-Audio Voicemeeter VAIO)", "Microphone: Voicemeeter Out B1 (VB-Audio Voicemeeter VAIO)"],
      ["", "Microphone: default microphone"],
    ])("shows the recorded microphone for label %j", async (label, text) => {
      (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
        getTracks: () => [{ stop: vi.fn() }],
        getAudioTracks: () => [{ label }],
      });

      const user = userEvent.setup();
      render(<Recorder onRecordingComplete={vi.fn()} />);
      await user.click(screen.getByRole("button", { name: "Start recording" }));

      expect(screen.getByText(text)).toBeInTheDocument();
      expect(screen.queryByText(/Microphone unavailable/)).not.toBeInTheDocument();
    });
  });

  describe("microphone picker", () => {
    const devices = [
      { kind: "audioinput", deviceId: "vm", label: "Voicemeeter Out B1" },
      { kind: "audioinput", deviceId: "jabra", label: "Microphone (Jabra PRO 9470)" },
      { kind: "videoinput", deviceId: "cam", label: "Webcam" },
    ];

    beforeEach(() => {
      Object.assign(navigator.mediaDevices, {
        enumerateDevices: vi.fn().mockResolvedValue(devices),
      });
    });
    afterEach(() => {
      localStorage.removeItem("recording-mic");
    });

    it("lists only named audio inputs plus the browser default", async () => {
      render(<Recorder onRecordingComplete={vi.fn()} />);

      const select = await screen.findByRole("combobox", { name: "Microphone" });
      const options = Array.from((select as HTMLSelectElement).options).map((o) => o.text);
      expect(options).toEqual(["Browser default", "Voicemeeter Out B1", "Microphone (Jabra PRO 9470)"]);
    });

    it("records from the chosen microphone and remembers it", async () => {
      const user = userEvent.setup();
      render(<Recorder onRecordingComplete={vi.fn()} />);
      await user.selectOptions(await screen.findByRole("combobox", { name: "Microphone" }), "jabra");
      await user.click(screen.getByRole("button", { name: "Start recording" }));

      expect(navigator.mediaDevices.getUserMedia).toHaveBeenCalledWith({
        audio: { deviceId: { exact: "jabra" } },
        video: false,
      });
      expect(localStorage.getItem("recording-mic")).toBe("jabra");
    });

    it("falls back to the browser default when the saved microphone is gone", async () => {
      localStorage.setItem("recording-mic", "unplugged");
      const user = userEvent.setup();
      render(<Recorder onRecordingComplete={vi.fn()} />);
      await screen.findByRole("combobox", { name: "Microphone" });
      await user.click(screen.getByRole("button", { name: "Start recording" }));

      expect(navigator.mediaDevices.getUserMedia).toHaveBeenCalledWith({ audio: true, video: false });
    });

    it("hides the picker until the browser reveals device names", async () => {
      (navigator.mediaDevices.enumerateDevices as ReturnType<typeof vi.fn>).mockResolvedValue([
        { kind: "audioinput", deviceId: "", label: "" },
      ]);
      render(<Recorder onRecordingComplete={vi.fn()} />);
      await act(async () => {});

      expect(screen.queryByRole("combobox", { name: "Microphone" })).not.toBeInTheDocument();
    });

    it("hides the picker when audio is off", async () => {
      const user = userEvent.setup();
      render(<Recorder onRecordingComplete={vi.fn()} />);
      await screen.findByRole("combobox", { name: "Microphone" });
      await user.click(screen.getByRole("button", { name: "Disable system audio" }));

      expect(screen.queryByRole("combobox", { name: "Microphone" })).not.toBeInTheDocument();
    });
  });

  describe("camera picker", () => {
    beforeEach(() => {
      Object.assign(navigator.mediaDevices, {
        enumerateDevices: vi.fn().mockResolvedValue([
          { kind: "videoinput", deviceId: "cam", label: "Integrated Webcam" },
          { kind: "videoinput", deviceId: "obs", label: "OBS Virtual Camera" },
          { kind: "audioinput", deviceId: "mic", label: "Headset" },
        ]),
      });
    });
    afterEach(() => {
      localStorage.removeItem("recording-camera");
    });

    it("shows the cameras once the camera is on", async () => {
      const user = userEvent.setup();
      const { rerender } = render(<Recorder onRecordingComplete={vi.fn()} />);
      await screen.findByRole("combobox", { name: "Microphone" });
      expect(screen.queryByRole("combobox", { name: "Camera" })).not.toBeInTheDocument();

      rerender(<Recorder withCamera onRecordingComplete={vi.fn()} />);

      const select = await screen.findByRole("combobox", { name: "Camera" });
      const options = Array.from((select as HTMLSelectElement).options).map((o) => o.text);
      expect(options).toEqual(["Browser default", "Integrated Webcam", "OBS Virtual Camera"]);
    });

    it("places the camera picker under the preview, not in the controls row", async () => {
      const user = userEvent.setup();
      render(<Recorder withCamera onRecordingComplete={vi.fn()} />);
      await flushWebcam();

      const select = await screen.findByRole("combobox", { name: "Camera" });
      expect(select.closest(".record-controls")).toBeNull();
      expect(select.closest(".pip-block")?.querySelector("video.pip-preview")).not.toBeNull();

      await user.click(screen.getByRole("button", { name: "Start recording" }));
      await user.click(await screen.findByTestId("countdown-overlay"));
      expect(screen.queryByRole("combobox", { name: "Camera" })).not.toBeInTheDocument();
    });

    it("switches to the chosen camera and remembers it", async () => {
      const user = userEvent.setup();
      render(<Recorder withCamera onRecordingComplete={vi.fn()} />);
      await flushWebcam();
      await user.selectOptions(await screen.findByRole("combobox", { name: "Camera" }), "obs");

      await vi.waitFor(() => {
        expect(navigator.mediaDevices.getUserMedia).toHaveBeenLastCalledWith({
          video: { deviceId: { exact: "obs" }, width: 320, height: 240 },
          audio: false,
        });
      });
      expect(localStorage.getItem("recording-camera")).toBe("obs");
      await vi.waitFor(() => expect(webcamPreview()).not.toBeNull());
    });

    it("keeps the picker reachable when the saved camera fails to open", async () => {
      localStorage.setItem("recording-camera", "obs");
      (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
        new DOMException("busy", "NotReadableError"),
      );
      const { rerender } = render(<Recorder onRecordingComplete={vi.fn()} />);
      await screen.findByRole("combobox", { name: "Microphone" });
      rerender(<Recorder withCamera onRecordingComplete={vi.fn()} />);
      await flushWebcam();

      expect(screen.getByText(/Could not access your camera/)).toBeInTheDocument();
      expect(screen.getByRole("combobox", { name: "Camera" })).toBeInTheDocument();
    });

    it("keeps one camera open, the chosen one, when switching faster than cameras open", async () => {
      const opened: { label: string; stop: ReturnType<typeof vi.fn> }[] = [];
      const pending: (() => void)[] = [];
      (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockImplementation(
        (c: { video: { deviceId?: { exact: string } } }) => new Promise((resolve) => {
          const track = { label: c.video.deviceId?.exact ?? "default", stop: vi.fn() };
          pending.push(() => { opened.push(track); resolve({ getTracks: () => [track] }); });
        }),
      );
      render(<Recorder withCamera onRecordingComplete={vi.fn()} />);
      await act(async () => { pending[0](); });
      const select = await screen.findByRole("combobox", { name: "Camera" });
      fireEvent.change(select, { target: { value: "obs" } });
      fireEvent.change(select, { target: { value: "cam" } });
      await act(async () => { pending.slice(1).forEach((open) => open()); });

      const live = opened.filter((t) => t.stop.mock.calls.length === 0);
      expect(live.map((t) => t.label)).toEqual(["cam"]);
      expect(webcamPreview()).not.toBeNull();
    });

    it("switches to the saved camera once devices are listed in Screen + Camera", async () => {
      localStorage.setItem("recording-camera", "obs");
      const tracks: { label: string; stop: ReturnType<typeof vi.fn> }[] = [];
      (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockImplementation(
        async (c: { video: { deviceId?: { exact: string } } }) => {
          const track = { label: c.video.deviceId?.exact ?? "default", stop: vi.fn() };
          tracks.push(track);
          return { getTracks: () => [track] };
        },
      );
      render(<Recorder withCamera onRecordingComplete={vi.fn()} />);

      await vi.waitFor(() => {
        expect(navigator.mediaDevices.getUserMedia).toHaveBeenLastCalledWith({
          video: { deviceId: { exact: "obs" }, width: 320, height: 240 },
          audio: false,
        });
      });
      await flushWebcam();
      expect(tracks.filter((t) => t.stop.mock.calls.length === 0).map((t) => t.label)).toEqual(["obs"]);
    });

    it("does not swap the camera when devices are listed mid-recording", async () => {
      localStorage.setItem("recording-camera", "obs");
      const pendingLists: ((devices: unknown[]) => void)[] = [];
      Object.assign(navigator.mediaDevices, {
        enumerateDevices: vi.fn().mockImplementation(() => new Promise((resolve) => { pendingLists.push(resolve); })),
      });
      const user = userEvent.setup();
      render(<Recorder withCamera onRecordingComplete={vi.fn()} />);
      await flushWebcam();
      await user.click(screen.getByRole("button", { name: "Start recording" }));
      await user.click(await screen.findByTestId("countdown-overlay"));
      const callsBefore = (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mock.calls.length;

      await act(async () => {
        pendingLists.forEach((list) => list([{ kind: "videoinput", deviceId: "obs", label: "OBS Virtual Camera" }]));
      });

      const later = (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mock.calls.slice(callsBefore);
      expect(later.filter(([c]) => c.video)).toHaveLength(0);
    });

    it("opens the saved camera when the camera is turned on", async () => {
      localStorage.setItem("recording-camera", "obs");
      const { rerender } = render(<Recorder onRecordingComplete={vi.fn()} />);
      await screen.findByRole("combobox", { name: "Microphone" });
      rerender(<Recorder withCamera onRecordingComplete={vi.fn()} />);
      await flushWebcam();

      expect(navigator.mediaDevices.getUserMedia).toHaveBeenCalledWith({
        video: { deviceId: { exact: "obs" }, width: 320, height: 240 },
        audio: false,
      });
    });
  });

  it("dismisses media error when dismiss button is clicked", async () => {
    (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
      new Error("Permission denied"),
    );

    const user = userEvent.setup();
    render(<Recorder withCamera onRecordingComplete={vi.fn()} />);
    await flushWebcam();

    expect(screen.getByRole("alert")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Dismiss error" }));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("clears media error when starting a new recording attempt", async () => {
    (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
      new Error("Permission denied"),
    );

    const user = userEvent.setup();
    render(<Recorder withCamera onRecordingComplete={vi.fn()} />);
    await flushWebcam();

    expect(screen.getByRole("alert")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
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

  it("stops the recorder when stopping from paused state", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: "Pause recording" }));

    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    expect(mediaRecorderInstances[0].stop).toHaveBeenCalledTimes(1);
  });

  it("resumes a paused webcam recorder before stopping it", async () => {
    const webcamStream = new MockMediaStream();
    (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>)
      .mockResolvedValue(webcamStream);
    const user = userEvent.setup();
    render(<Recorder withCamera onRecordingComplete={vi.fn()} />);
    await flushWebcam();
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));
    await user.click(screen.getByRole("button", { name: "Pause recording" }));

    const webcamRecorder = mediaRecorderInstances[1];
    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    expect(webcamRecorder.resume).toHaveBeenCalledTimes(1);
    expect(webcamRecorder.stop).toHaveBeenCalledTimes(1);
    expect(webcamRecorder.resume.mock.invocationCallOrder[0])
      .toBeLessThan(webcamRecorder.stop.mock.invocationCallOrder[0]);
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

    // The auto-stop should have stopped the recorder
    expect(mediaRecorderInstances[0].stop).toHaveBeenCalled();

    vi.useRealTimers();
  });

  it("shows webcam preview when camera is enabled in idle state", async () => {
    const user = userEvent.setup();
    render(<Recorder withCamera onRecordingComplete={vi.fn()} />);
    await flushWebcam();

    // Webcam preview video should appear (the pip-preview class video)
    const videos = document.querySelectorAll("video.pip-preview");
    expect(videos.length).toBe(1);
  });

  it("sets up webcam recorder when webcam is enabled before recording starts", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    // Return a proper mock stream from getUserMedia
    const webcamStop = vi.fn();
    const mockWebcamStream = new MockMediaStream();
    mockWebcamStream.getTracks.mockReturnValue([{ stop: webcamStop }]);
    (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockResolvedValue(mockWebcamStream);

    const onComplete = vi.fn();
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<Recorder withCamera onRecordingComplete={onComplete} />);
    await flushWebcam();
    expect(webcamPreview()).not.toBeNull();

    // Start recording with webcam enabled
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(await screen.findByTestId("countdown-overlay"));

    // Should be in recording state with webcam active
    expect(
      screen.getByRole("button", { name: "Stop recording" }),
    ).toBeInTheDocument();

    // Advance past MIN_RECORDING_SECONDS so validation passes
    await act(() => { vi.advanceTimersByTime(1500); });

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
    vi.useRealTimers();
  });

  describe("Screen + Camera on mount", () => {
    it("opens the webcam on mount without a click", async () => {
      render(<Recorder withCamera onRecordingComplete={vi.fn()} />);

      await vi.waitFor(() => {
        expect(navigator.mediaDevices.getUserMedia).toHaveBeenCalledWith({
          video: { width: 320, height: 240, facingMode: "user" },
          audio: false,
        });
      });
      await vi.waitFor(() => expect(webcamPreview()).not.toBeNull());
    });

    it("keeps one webcam open under StrictMode double mount", async () => {
      const tracks: { stop: ReturnType<typeof vi.fn> }[] = [];
      (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockImplementation(async () => {
        const track = { stop: vi.fn() };
        tracks.push(track);
        return { getTracks: () => [track] };
      });
      render(<StrictMode><Recorder withCamera onRecordingComplete={vi.fn()} /></StrictMode>);

      await vi.waitFor(() => expect(webcamPreview()).not.toBeNull());
      expect(tracks.length).toBe(2);
      expect(tracks.filter((t) => t.stop.mock.calls.length === 0)).toHaveLength(1);
    });

    it("does not open the webcam without withCamera", async () => {
      render(<Recorder onRecordingComplete={vi.fn()} />);
      await flushWebcam();

      expect(navigator.mediaDevices.getUserMedia).not.toHaveBeenCalledWith(
        expect.objectContaining({ video: expect.anything() }),
      );
    });

    it("records the webcam without toggling the camera", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const mockWebcamStream = new MockMediaStream();
      mockWebcamStream.getTracks.mockReturnValue([{ stop: vi.fn() }]);
      (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockResolvedValue(mockWebcamStream);

      const onComplete = vi.fn();
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      render(<Recorder withCamera onRecordingComplete={onComplete} />);
      await vi.waitFor(() => {
        expect(document.querySelector("video.pip-preview")).not.toBeNull();
      });

      await user.click(screen.getByRole("button", { name: "Start recording" }));
      await user.click(await screen.findByTestId("countdown-overlay"));
      await act(() => { vi.advanceTimersByTime(1500); });
      await user.click(screen.getByRole("button", { name: "Stop recording" }));

      await vi.waitFor(() => {
        expect(onComplete).toHaveBeenCalledTimes(1);
      });
      expect(onComplete).toHaveBeenCalledWith(
        expect.any(Blob),
        expect.any(Number),
        expect.any(Blob),
      );
      vi.useRealTimers();
    });

    it("shows no preview when the webcam fails to open on mount", async () => {
      (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
        new Error("Permission denied"),
      );
      render(<Recorder withCamera onRecordingComplete={vi.fn()} />);

      expect(
        await screen.findByText("Could not access your camera. Please allow camera access and try again."),
      ).toBeInTheDocument();
      expect(webcamPreview()).toBeNull();
    });
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
    act(() => {
      endedCallback!();
    });

    expect(mediaRecorderInstances[0].stop).toHaveBeenCalledTimes(1);
  });

  it("returns to idle when screen sharing ends during countdown", async () => {
    let endedCallback: (() => void) | undefined;
    const stopTrack = vi.fn();
    mockScreenStream.getVideoTracks.mockReturnValue([{
      getSettings: () => ({ width: 1920, height: 1080 }),
      addEventListener: vi.fn().mockImplementation((event: string, handler: () => void) => {
        if (event === "ended") endedCallback = handler;
      }),
      stop: stopTrack,
    }]);
    mockScreenStream.getTracks.mockReturnValue([{ stop: stopTrack }]);

    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    expect(screen.getByTestId("countdown-overlay")).toBeInTheDocument();

    act(() => endedCallback!());

    expect(screen.queryByTestId("countdown-overlay")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
    expect(stopTrack).toHaveBeenCalled();
    expect(mediaRecorderInstances[0].stop).not.toHaveBeenCalled();
  });

  it("falls back from a runtime MP4 encoder error to WebM", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    MockMediaRecorder.isTypeSupported = vi.fn().mockReturnValue(true);
    // MP4 is only chosen where no overlay is applied; compositing pins WebM up
    // front, so there is no MP4 encoder error to recover from there.
    mockCanRecordAnnotations = false;
    mockOverlayDrawingOnTrack.mockImplementation((track: unknown) => track);
    const onComplete = vi.fn();
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

    render(<Recorder onRecordingComplete={onComplete} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    act(() => mediaRecorderInstances[0].onerror?.(new Event("error")));
    expect(mediaRecorderInstances).toHaveLength(2);
    expect(mediaRecorderInstances[1].start).toHaveBeenCalledTimes(1);

    await act(() => vi.advanceTimersByTime(1500));
    await user.click(screen.getByRole("button", { name: "Stop recording" }));
    await vi.waitFor(() => expect(onComplete).toHaveBeenCalledTimes(1));

    expect((onComplete.mock.calls[0][0] as Blob).type).toBe("video/webm");
    vi.useRealTimers();
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

  it("shows Audio On button in idle state by default", () => {
    render(<Recorder onRecordingComplete={vi.fn()} />);
    expect(screen.getByText("Audio On")).toBeInTheDocument();
  });

  it("toggles system audio off when clicked", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Disable system audio" }));
    expect(screen.getByText("Audio Off")).toBeInTheDocument();
  });

  it("toggles system audio back on when clicked again", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Disable system audio" }));
    await user.click(screen.getByRole("button", { name: "Enable system audio" }));
    expect(screen.getByText("Audio On")).toBeInTheDocument();
  });

  it("calls getDisplayMedia with system audio constraints by default", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(navigator.mediaDevices.getDisplayMedia).toHaveBeenCalledWith(
      expect.objectContaining({
        video: true,
        audio: true,
        systemAudio: "include",
        suppressLocalAudioPlayback: true,
      }),
    );
  });

  it("calls onRecordingError when recording is shorter than 1 second", async () => {
    const onComplete = vi.fn();
    const onError = vi.fn();
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={onComplete} onRecordingError={onError} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    // Stop immediately — elapsed will be 0 seconds and blob will be tiny
    await user.click(screen.getByRole("button", { name: "Stop recording" }));

    await vi.waitFor(() => {
      expect(onError).toHaveBeenCalledWith("Recording too short. Please record for at least 1 second.");
    });
    expect(onComplete).not.toHaveBeenCalled();
  });

  it("calls getDisplayMedia with audio disabled when system audio is off", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Disable system audio" }));
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(navigator.mediaDevices.getDisplayMedia).toHaveBeenCalledWith(
      expect.objectContaining({
        video: true,
        audio: false,
      }),
    );
  });

  it("has no accessibility violations", async () => {
    const { container } = render(<Recorder onRecordingComplete={vi.fn()} />);
    await expectNoA11yViolations(container);
  });

  it("warns that drawing is preview-only where the browser cannot record it", async () => {
    mockCanRecordAnnotations = false;
    mockDrawMode = true;
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    expect(screen.getByTestId("draw-preview-only")).toBeInTheDocument();
  });

  it("stays quiet about drawing where the browser can record it", async () => {
    mockDrawMode = true;
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));
    await user.click(screen.getByTestId("countdown-overlay"));

    expect(screen.queryByTestId("draw-preview-only")).not.toBeInTheDocument();
  });

  it("records WebM when annotations are composited into the track", async () => {
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    // Chrome's MP4 muxer produces a black, silent file from the generator
    // track's canvas-derived frames.
    expect(mediaRecorderOptions.at(-1)?.mimeType).toMatch(/^video\/webm/);
  });

  it("keeps the preferred container where no overlay is applied", async () => {
    mockCanRecordAnnotations = false;
    mockOverlayDrawingOnTrack.mockImplementation((track: unknown) => track);
    const user = userEvent.setup();
    render(<Recorder onRecordingComplete={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Start recording" }));

    expect(mediaRecorderOptions.at(-1)?.mimeType).toBe("video/mp4");
  });

  describe("Safari screen capture warning", () => {
    const realUserAgent = navigator.userAgent;

    function setUserAgent(value: string) {
      Object.defineProperty(navigator, "userAgent", {
        value,
        configurable: true,
      });
    }

    afterEach(() => setUserAgent(realUserAgent));

    it("warns before recording that Safari stops capturing a hidden window", () => {
      setUserAgent(
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Safari/605.1.15",
      );
      render(<Recorder onRecordingComplete={vi.fn()} />);

      expect(screen.getByTestId("browser-capture-warning")).toHaveTextContent(
        /Chrome or Edge/i,
      );
    });

    it("stays quiet where screen capture survives a hidden window", () => {
      setUserAgent(
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36",
      );
      render(<Recorder onRecordingComplete={vi.fn()} />);

      expect(
        screen.queryByTestId("browser-capture-warning"),
      ).not.toBeInTheDocument();
    });

    it("keeps the warning out of the way once recording starts", async () => {
      setUserAgent(
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Safari/605.1.15",
      );
      const user = userEvent.setup();
      render(<Recorder onRecordingComplete={vi.fn()} />);
      await user.click(screen.getByRole("button", { name: "Start recording" }));
      await user.click(screen.getByTestId("countdown-overlay"));

      expect(
        screen.queryByTestId("browser-capture-warning"),
      ).not.toBeInTheDocument();
    });
  });

  describe("capture stalls", () => {
    // The browser reports a stalled capture by muting the track; a muted video
    // track repeats its last frame while the microphone keeps recording.
    function trackWithMuteCapture() {
      const handlers: Record<string, () => void> = {};
      mockScreenStream.getVideoTracks.mockReturnValue([
        {
          getSettings: () => ({ width: 1920, height: 1080 }),
          addEventListener: vi.fn().mockImplementation(
            (event: string, handler: () => void) => {
              handlers[event] = handler;
            },
          ),
          stop: vi.fn(),
        },
      ]);
      return handlers;
    }

    it("says so on screen while the capture is stalled", async () => {
      const handlers = trackWithMuteCapture();
      const user = userEvent.setup();
      render(<Recorder onRecordingComplete={vi.fn()} />);
      await user.click(screen.getByRole("button", { name: "Start recording" }));
      await user.click(screen.getByTestId("countdown-overlay"));

      act(() => handlers.mute());
      expect(screen.getByTestId("capture-stalled-warning")).toBeInTheDocument();

      act(() => handlers.unmute());
      expect(
        screen.queryByTestId("capture-stalled-warning"),
      ).not.toBeInTheDocument();
    });

    it("refuses a recording whose capture stalled", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const handlers = trackWithMuteCapture();
      const onComplete = vi.fn();
      const onError = vi.fn();
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

      render(
        <Recorder onRecordingComplete={onComplete} onRecordingError={onError} />,
      );
      await user.click(screen.getByRole("button", { name: "Start recording" }));
      await user.click(screen.getByTestId("countdown-overlay"));

      act(() => handlers.mute());
      await act(() => vi.advanceTimersByTime(4000));
      await user.click(screen.getByRole("button", { name: "Stop recording" }));

      await vi.waitFor(() => expect(onError).toHaveBeenCalledTimes(1));
      expect(onError.mock.calls[0][0]).toMatch(/stopped capturing/i);
      expect(onComplete).not.toHaveBeenCalled();
      vi.useRealTimers();
    });

    it("keeps a recording that only blipped", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const handlers = trackWithMuteCapture();
      const onComplete = vi.fn();
      const onError = vi.fn();
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

      render(
        <Recorder onRecordingComplete={onComplete} onRecordingError={onError} />,
      );
      await user.click(screen.getByRole("button", { name: "Start recording" }));
      await user.click(screen.getByTestId("countdown-overlay"));

      act(() => handlers.mute());
      await act(() => vi.advanceTimersByTime(500));
      act(() => handlers.unmute());
      await act(() => vi.advanceTimersByTime(1500));
      await user.click(screen.getByRole("button", { name: "Stop recording" }));

      await vi.waitFor(() => expect(onComplete).toHaveBeenCalledTimes(1));
      expect(onError).not.toHaveBeenCalled();
      vi.useRealTimers();
    });

    // A capture that never produces a frame is muted from the start, and a
    // track that starts muted fires no mute event — only its state says so.
    it("refuses a recording whose capture was muted from the start", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      mockScreenStream.getVideoTracks.mockReturnValue([
        {
          muted: true,
          getSettings: () => ({ width: 1920, height: 1080 }),
          addEventListener: vi.fn(),
          stop: vi.fn(),
        },
      ]);
      const onComplete = vi.fn();
      const onError = vi.fn();
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

      render(
        <Recorder onRecordingComplete={onComplete} onRecordingError={onError} />,
      );
      await user.click(screen.getByRole("button", { name: "Start recording" }));
      await user.click(screen.getByTestId("countdown-overlay"));

      expect(screen.getByTestId("capture-stalled-warning")).toBeInTheDocument();

      await act(() => vi.advanceTimersByTime(4000));
      await user.click(screen.getByRole("button", { name: "Stop recording" }));

      await vi.waitFor(() => expect(onError).toHaveBeenCalledTimes(1));
      expect(onComplete).not.toHaveBeenCalled();
      vi.useRealTimers();
    });

    it("does not count a capture muted while the recording is paused", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const handlers = trackWithMuteCapture();
      const onComplete = vi.fn();
      const onError = vi.fn();
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

      render(
        <Recorder onRecordingComplete={onComplete} onRecordingError={onError} />,
      );
      await user.click(screen.getByRole("button", { name: "Start recording" }));
      await user.click(screen.getByTestId("countdown-overlay"));
      await act(() => vi.advanceTimersByTime(1500));

      await user.click(screen.getByRole("button", { name: "Pause recording" }));
      act(() => handlers.mute());
      await act(() => vi.advanceTimersByTime(10_000));
      act(() => handlers.unmute());
      await user.click(screen.getByRole("button", { name: "Resume recording" }));
      await act(() => vi.advanceTimersByTime(1500));
      await user.click(screen.getByRole("button", { name: "Stop recording" }));

      await vi.waitFor(() => expect(onComplete).toHaveBeenCalledTimes(1));
      expect(onError).not.toHaveBeenCalled();
      vi.useRealTimers();
    });
  });
});
