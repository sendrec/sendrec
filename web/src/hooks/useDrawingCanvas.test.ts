import { renderHook, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { useDrawingCanvas } from "./useDrawingCanvas";
import type { RefObject } from "react";

function createMockContext() {
  return {
    beginPath: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    stroke: vi.fn(),
    clearRect: vi.fn(),
    lineWidth: 0,
    lineCap: "butt" as CanvasLineCap,
    lineJoin: "miter" as CanvasLineJoin,
    strokeStyle: "",
  };
}

function createMockCanvas(
  ctx: ReturnType<typeof createMockContext>,
  displayWidth: number,
  displayHeight: number,
) {
  return {
    getContext: vi.fn().mockReturnValue(ctx),
    getBoundingClientRect: vi.fn().mockReturnValue({
      left: 0,
      top: 0,
      width: displayWidth,
      height: displayHeight,
    }),
    width: 1920,
    height: 1080,
  } as unknown as HTMLCanvasElement;
}

function pointerEvent(clientX: number, clientY: number) {
  return { clientX, clientY } as React.PointerEvent;
}

describe("useDrawingCanvas", () => {
  let ctx: ReturnType<typeof createMockContext>;
  let canvas: HTMLCanvasElement;
  let canvasRef: RefObject<HTMLCanvasElement | null>;

  beforeEach(() => {
    ctx = createMockContext();
    canvas = createMockCanvas(ctx, 640, 360);
    canvasRef = { current: canvas };
    vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => {
      cb(0);
      return 0;
    });
    vi.stubGlobal("cancelAnimationFrame", vi.fn());
  });

  it("has drawMode false and drawColor red initially", () => {
    const { result } = renderHook(() =>
      useDrawingCanvas({ canvasRef, captureWidth: 1920, captureHeight: 1080 }),
    );
    expect(result.current.drawMode).toBe(false);
    expect(result.current.drawColor).toBe("#ff0000");
  });

  it("toggles drawMode on and off", () => {
    const { result } = renderHook(() =>
      useDrawingCanvas({ canvasRef, captureWidth: 1920, captureHeight: 1080 }),
    );
    act(() => result.current.toggleDrawMode());
    expect(result.current.drawMode).toBe(true);
    act(() => result.current.toggleDrawMode());
    expect(result.current.drawMode).toBe(false);
  });

  it("updates drawColor", () => {
    const { result } = renderHook(() =>
      useDrawingCanvas({ canvasRef, captureWidth: 1920, captureHeight: 1080 }),
    );
    act(() => result.current.setDrawColor("#00ff00"));
    expect(result.current.drawColor).toBe("#00ff00");
  });

  it("draws a line on pointerdown then pointermove", () => {
    const { result } = renderHook(() =>
      useDrawingCanvas({ canvasRef, captureWidth: 1920, captureHeight: 1080 }),
    );
    act(() => result.current.toggleDrawMode());

    act(() => result.current.handlePointerDown(pointerEvent(100, 50)));
    act(() => result.current.handlePointerMove(pointerEvent(200, 100)));

    expect(ctx.beginPath).toHaveBeenCalled();
    expect(ctx.moveTo).toHaveBeenCalled();
    expect(ctx.lineTo).toHaveBeenCalled();
    expect(ctx.stroke).toHaveBeenCalled();
  });

  it("does not draw on pointermove when not drawing", () => {
    const { result } = renderHook(() =>
      useDrawingCanvas({ canvasRef, captureWidth: 1920, captureHeight: 1080 }),
    );
    act(() => result.current.toggleDrawMode());

    act(() => result.current.handlePointerMove(pointerEvent(200, 100)));

    expect(ctx.beginPath).not.toHaveBeenCalled();
  });

  it("stops drawing on pointerup", () => {
    const { result } = renderHook(() =>
      useDrawingCanvas({ canvasRef, captureWidth: 1920, captureHeight: 1080 }),
    );
    act(() => result.current.toggleDrawMode());
    act(() => result.current.handlePointerDown(pointerEvent(100, 50)));
    act(() => result.current.handlePointerUp());

    ctx.beginPath.mockClear();
    act(() => result.current.handlePointerMove(pointerEvent(200, 100)));
    expect(ctx.beginPath).not.toHaveBeenCalled();
  });

  it("stops drawing on pointerleave", () => {
    const { result } = renderHook(() =>
      useDrawingCanvas({ canvasRef, captureWidth: 1920, captureHeight: 1080 }),
    );
    act(() => result.current.toggleDrawMode());
    act(() => result.current.handlePointerDown(pointerEvent(100, 50)));
    act(() => result.current.handlePointerLeave());

    ctx.beginPath.mockClear();
    act(() => result.current.handlePointerMove(pointerEvent(200, 100)));
    expect(ctx.beginPath).not.toHaveBeenCalled();
  });

  it("clears the canvas", () => {
    const { result } = renderHook(() =>
      useDrawingCanvas({ canvasRef, captureWidth: 1920, captureHeight: 1080 }),
    );
    act(() => result.current.clearCanvas());
    expect(ctx.clearRect).toHaveBeenCalledWith(0, 0, 1920, 1080);
  });

  it("scales coordinates from display to capture resolution", () => {
    const { result } = renderHook(() =>
      useDrawingCanvas({ canvasRef, captureWidth: 1920, captureHeight: 1080 }),
    );
    act(() => result.current.toggleDrawMode());

    // Display: 640x360, Capture: 1920x1080, scale = 3x
    // CSS point (320, 180) → canvas (960, 540)
    act(() => result.current.handlePointerDown(pointerEvent(0, 0)));
    act(() => result.current.handlePointerMove(pointerEvent(320, 180)));

    expect(ctx.lineTo).toHaveBeenCalledWith(960, 540);
  });

  it("sets stroke properties correctly", () => {
    const { result } = renderHook(() =>
      useDrawingCanvas({ canvasRef, captureWidth: 1920, captureHeight: 1080 }),
    );
    act(() => result.current.toggleDrawMode());
    act(() => result.current.handlePointerDown(pointerEvent(0, 0)));
    act(() => result.current.handlePointerMove(pointerEvent(100, 50)));

    expect(ctx.lineWidth).toBe(2);
    expect(ctx.lineCap).toBe("round");
    expect(ctx.lineJoin).toBe("round");
    expect(ctx.strokeStyle).toBe("#ff0000");
  });

  it("has default lineWidth of 2", () => {
    const { result } = renderHook(() =>
      useDrawingCanvas({ canvasRef, captureWidth: 1920, captureHeight: 1080 }),
    );
    expect(result.current.lineWidth).toBe(2);
  });

  it("updates lineWidth via setLineWidth", () => {
    const { result } = renderHook(() =>
      useDrawingCanvas({ canvasRef, captureWidth: 1920, captureHeight: 1080 }),
    );
    act(() => result.current.setLineWidth(8));
    expect(result.current.lineWidth).toBe(8);
  });

  it("uses updated lineWidth when drawing", () => {
    const { result } = renderHook(() =>
      useDrawingCanvas({ canvasRef, captureWidth: 1920, captureHeight: 1080 }),
    );
    act(() => result.current.toggleDrawMode());
    act(() => result.current.setLineWidth(6));
    act(() => result.current.handlePointerDown(pointerEvent(0, 0)));
    act(() => result.current.handlePointerMove(pointerEvent(100, 50)));

    expect(ctx.lineWidth).toBe(6);
  });

  it("does not draw when drawMode is off", () => {
    const { result } = renderHook(() =>
      useDrawingCanvas({ canvasRef, captureWidth: 1920, captureHeight: 1080 }),
    );
    // drawMode is false by default
    act(() => result.current.handlePointerDown(pointerEvent(100, 50)));
    act(() => result.current.handlePointerMove(pointerEvent(200, 100)));

    expect(ctx.beginPath).not.toHaveBeenCalled();
  });
});
