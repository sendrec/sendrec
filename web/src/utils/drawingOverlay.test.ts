import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { overlayDrawingOnTrack, canRecordAnnotations } from "./drawingOverlay";

class MockVideoFrame {
  displayWidth = 1920;
  displayHeight = 1080;
  timestamp = 0;
  duration: number | null = 33_333;
  close = vi.fn();
  constructor(_source?: unknown, init?: { timestamp: number; duration?: number }) {
    if (init) this.timestamp = init.timestamp;
  }
}

function makeTrack() {
  return { kind: "video", stop: vi.fn() } as unknown as MediaStreamTrack;
}

/** Feeds the given frames through the processor readable and returns what the generator received. */
function installBreakoutBox(frames: MockVideoFrame[]) {
  const written: unknown[] = [];
  vi.stubGlobal(
    "MediaStreamTrackProcessor",
    class {
      readable = new ReadableStream({
        start(controller) {
          frames.forEach((f) => controller.enqueue(f));
          controller.close();
        },
      });
    },
  );
  vi.stubGlobal(
    "MediaStreamTrackGenerator",
    class {
      kind = "video";
      writable = new WritableStream({
        write(chunk) {
          written.push(chunk);
        },
      });
    },
  );
  return written;
}

describe("overlayDrawingOnTrack", () => {
  let ctx: { drawImage: ReturnType<typeof vi.fn> };
  let drawingCanvas: HTMLCanvasElement;

  beforeEach(() => {
    ctx = { drawImage: vi.fn() };
    HTMLCanvasElement.prototype.getContext = vi
      .fn()
      .mockReturnValue(ctx) as unknown as HTMLCanvasElement["getContext"];
    drawingCanvas = document.createElement("canvas");
    vi.stubGlobal("VideoFrame", MockVideoFrame);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("returns the original track when the browser has no breakout box support", () => {
    vi.stubGlobal("MediaStreamTrackProcessor", undefined);
    vi.stubGlobal("MediaStreamTrackGenerator", undefined);
    const track = makeTrack();

    expect(overlayDrawingOnTrack(track, drawingCanvas, () => true)).toBe(track);
  });

  it("returns a generator track when the breakout box is available", () => {
    installBreakoutBox([]);
    const track = makeTrack();

    expect(overlayDrawingOnTrack(track, drawingCanvas, () => false)).not.toBe(track);
  });

  it("passes frames through untouched while nothing has been drawn", async () => {
    const frame = new MockVideoFrame();
    const written = installBreakoutBox([frame]);

    overlayDrawingOnTrack(makeTrack(), drawingCanvas, () => false);
    await vi.waitFor(() => expect(written).toHaveLength(1));

    expect(written[0]).toBe(frame);
    expect(ctx.drawImage).not.toHaveBeenCalled();
    expect(frame.close).not.toHaveBeenCalled();
  });

  it("composites the drawing canvas over each frame once the user has drawn", async () => {
    const frame = new MockVideoFrame();
    frame.timestamp = 12_345;
    const written = installBreakoutBox([frame]);

    overlayDrawingOnTrack(makeTrack(), drawingCanvas, () => true);
    await vi.waitFor(() => expect(written).toHaveLength(1));

    // Source frame first, annotations on top.
    expect(ctx.drawImage.mock.calls[0][0]).toBe(frame);
    expect(ctx.drawImage.mock.calls[1][0]).toBe(drawingCanvas);
    expect(written[0]).not.toBe(frame);
    expect((written[0] as MockVideoFrame).timestamp).toBe(12_345);
    // The source frame must be released or the encoder starves.
    expect(frame.close).toHaveBeenCalled();
  });

  it("keeps recording the plain capture when compositing a frame fails", async () => {
    const frames = [new MockVideoFrame(), new MockVideoFrame()];
    frames[0].timestamp = 1;
    frames[1].timestamp = 2;
    const written = installBreakoutBox(frames);
    // One allocation failure must not end the video track, or the recording
    // silently continues as audio-only.
    let calls = 0;
    vi.stubGlobal(
      "VideoFrame",
      class extends MockVideoFrame {
        constructor(source?: unknown, init?: { timestamp: number; duration?: number }) {
          super(source, init);
          if (++calls === 1) throw new DOMException("out of memory", "OperationError");
        }
      },
    );
    vi.spyOn(console, "warn").mockImplementation(() => {});

    overlayDrawingOnTrack(makeTrack(), drawingCanvas, () => true);
    await vi.waitFor(() => expect(written).toHaveLength(2));

    // The failed frame is passed through untouched rather than dropped or leaked.
    expect(written[0]).toBe(frames[0]);
    expect(frames[0].close).not.toHaveBeenCalled();
    // The next frame still composites.
    expect(written[1]).not.toBe(frames[1]);
    expect(frames[1].close).toHaveBeenCalled();
  });

  it("warns once rather than per frame when compositing keeps failing", async () => {
    const written = installBreakoutBox([new MockVideoFrame(), new MockVideoFrame()]);
    vi.stubGlobal(
      "VideoFrame",
      class {
        constructor() {
          throw new DOMException("out of memory", "OperationError");
        }
      },
    );
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});

    overlayDrawingOnTrack(makeTrack(), drawingCanvas, () => true);
    await vi.waitFor(() => expect(written).toHaveLength(2));

    expect(warn).toHaveBeenCalledTimes(1);
  });
});

describe("canRecordAnnotations", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("is false where the breakout box is missing", () => {
    vi.stubGlobal("MediaStreamTrackProcessor", undefined);
    vi.stubGlobal("MediaStreamTrackGenerator", undefined);
    expect(canRecordAnnotations()).toBe(false);
  });

  it("is true where both constructors exist", () => {
    vi.stubGlobal("MediaStreamTrackProcessor", class {});
    vi.stubGlobal("MediaStreamTrackGenerator", class {});
    expect(canRecordAnnotations()).toBe(true);
  });
});
