import { renderHook, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { useCanvasCompositing } from "./useCanvasCompositing";
import type { RefObject } from "react";

function createMockContext() {
  return {
    drawImage: vi.fn(),
  };
}

describe("useCanvasCompositing", () => {
  let compositingCtx: ReturnType<typeof createMockContext>;
  let compositingCanvas: HTMLCanvasElement;
  let compositingCanvasRef: RefObject<HTMLCanvasElement | null>;
  let screenVideo: HTMLVideoElement;
  let screenVideoRef: RefObject<HTMLVideoElement | null>;
  let drawingCanvas: HTMLCanvasElement;
  let drawingCanvasRef: RefObject<HTMLCanvasElement | null>;

  beforeEach(() => {
    vi.useFakeTimers();

    compositingCtx = createMockContext();
    compositingCanvas = {
      getContext: vi.fn().mockReturnValue(compositingCtx),
      captureStream: vi.fn().mockReturnValue({
        addTrack: vi.fn(),
        getVideoTracks: vi.fn().mockReturnValue([{ kind: "video" }]),
        getAudioTracks: vi.fn().mockReturnValue([]),
      }),
      width: 1920,
      height: 1080,
    } as unknown as HTMLCanvasElement;
    compositingCanvasRef = { current: compositingCanvas };

    screenVideo = { videoWidth: 1920, videoHeight: 1080 } as HTMLVideoElement;
    screenVideoRef = { current: screenVideo };

    drawingCanvas = { width: 1920, height: 1080 } as HTMLCanvasElement;
    drawingCanvasRef = { current: drawingCanvas };
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("starts interval loop on startCompositing", () => {
    const { result } = renderHook(() =>
      useCanvasCompositing({
        compositingCanvasRef,
        screenVideoRef,
        drawingCanvasRef,
      }),
    );

    act(() => result.current.startCompositing());

    // Advance one interval tick (~33ms for 30fps)
    act(() => vi.advanceTimersByTime(34));

    expect(compositingCtx.drawImage).toHaveBeenCalled();
  });

  it("draws video then drawing canvas each frame", () => {
    const { result } = renderHook(() =>
      useCanvasCompositing({
        compositingCanvasRef,
        screenVideoRef,
        drawingCanvasRef,
      }),
    );

    act(() => result.current.startCompositing());
    act(() => vi.advanceTimersByTime(34));

    expect(compositingCtx.drawImage).toHaveBeenCalledTimes(2);
    expect(compositingCtx.drawImage).toHaveBeenNthCalledWith(
      1,
      screenVideo,
      0,
      0,
    );
    expect(compositingCtx.drawImage).toHaveBeenNthCalledWith(
      2,
      drawingCanvas,
      0,
      0,
    );
  });

  it("clears interval on stopCompositing", () => {
    const { result } = renderHook(() =>
      useCanvasCompositing({
        compositingCanvasRef,
        screenVideoRef,
        drawingCanvasRef,
      }),
    );

    act(() => result.current.startCompositing());
    act(() => result.current.stopCompositing());

    compositingCtx.drawImage.mockClear();
    act(() => vi.advanceTimersByTime(100));

    // No more draws after stop
    expect(compositingCtx.drawImage).not.toHaveBeenCalled();
  });

  it("returns composited stream with audio tracks", () => {
    const mockAudioTrack = { kind: "audio" } as MediaStreamTrack;
    const { result } = renderHook(() =>
      useCanvasCompositing({
        compositingCanvasRef,
        screenVideoRef,
        drawingCanvasRef,
      }),
    );

    const stream = result.current.getCompositedStream([mockAudioTrack]);

    expect(stream).not.toBeNull();
    // Draws an initial frame before capturing
    expect(compositingCtx.drawImage).toHaveBeenCalledWith(screenVideo, 0, 0);
    // Uses fixed 30fps for reliable frame capture
    expect(
      (compositingCanvas.captureStream as ReturnType<typeof vi.fn>),
    ).toHaveBeenCalledWith(30);
    expect(stream!.addTrack).toHaveBeenCalledWith(mockAudioTrack);
  });

  it("clears interval on unmount", () => {
    const { result, unmount } = renderHook(() =>
      useCanvasCompositing({
        compositingCanvasRef,
        screenVideoRef,
        drawingCanvasRef,
      }),
    );

    act(() => result.current.startCompositing());
    unmount();

    compositingCtx.drawImage.mockClear();
    act(() => vi.advanceTimersByTime(100));

    expect(compositingCtx.drawImage).not.toHaveBeenCalled();
  });
});
