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
  let rafCallbacks: FrameRequestCallback[];
  let rafId: number;

  beforeEach(() => {
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

    rafCallbacks = [];
    rafId = 0;
    vi.spyOn(globalThis, "requestAnimationFrame").mockImplementation((cb) => {
      rafCallbacks.push(cb);
      return ++rafId;
    });
    vi.spyOn(globalThis, "cancelAnimationFrame").mockImplementation(vi.fn());
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("starts rAF loop on startCompositing", () => {
    const { result } = renderHook(() =>
      useCanvasCompositing({
        compositingCanvasRef,
        screenVideoRef,
        drawingCanvasRef,
        captureWidth: 1920,
        captureHeight: 1080,
      }),
    );

    act(() => result.current.startCompositing());

    expect(requestAnimationFrame).toHaveBeenCalled();
  });

  it("draws video then drawing canvas each frame", () => {
    const { result } = renderHook(() =>
      useCanvasCompositing({
        compositingCanvasRef,
        screenVideoRef,
        drawingCanvasRef,
        captureWidth: 1920,
        captureHeight: 1080,
      }),
    );

    act(() => result.current.startCompositing());
    // Execute the rAF callback
    act(() => rafCallbacks[0](0));

    expect(compositingCtx.drawImage).toHaveBeenCalledTimes(2);
    expect(compositingCtx.drawImage).toHaveBeenNthCalledWith(
      1,
      screenVideo,
      0,
      0,
      1920,
      1080,
    );
    expect(compositingCtx.drawImage).toHaveBeenNthCalledWith(
      2,
      drawingCanvas,
      0,
      0,
      1920,
      1080,
    );
  });

  it("cancels rAF on stopCompositing", () => {
    const { result } = renderHook(() =>
      useCanvasCompositing({
        compositingCanvasRef,
        screenVideoRef,
        drawingCanvasRef,
        captureWidth: 1920,
        captureHeight: 1080,
      }),
    );

    act(() => result.current.startCompositing());
    act(() => result.current.stopCompositing());

    expect(cancelAnimationFrame).toHaveBeenCalled();
  });

  it("returns composited stream with audio tracks", () => {
    const mockAudioTrack = { kind: "audio" } as MediaStreamTrack;
    const { result } = renderHook(() =>
      useCanvasCompositing({
        compositingCanvasRef,
        screenVideoRef,
        drawingCanvasRef,
        captureWidth: 1920,
        captureHeight: 1080,
      }),
    );

    const stream = result.current.getCompositedStream([mockAudioTrack]);

    expect(stream).not.toBeNull();
    expect(
      (compositingCanvas.captureStream as ReturnType<typeof vi.fn>),
    ).toHaveBeenCalledWith(30);
    expect(stream!.addTrack).toHaveBeenCalledWith(mockAudioTrack);
  });

  it("cancels rAF on unmount", () => {
    const { result, unmount } = renderHook(() =>
      useCanvasCompositing({
        compositingCanvasRef,
        screenVideoRef,
        drawingCanvasRef,
        captureWidth: 1920,
        captureHeight: 1080,
      }),
    );

    act(() => result.current.startCompositing());
    unmount();

    expect(cancelAnimationFrame).toHaveBeenCalled();
  });
});
