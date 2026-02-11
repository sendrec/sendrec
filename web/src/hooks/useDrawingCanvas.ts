import { useCallback, useRef, useState } from "react";
import type { RefObject, PointerEvent } from "react";

interface UseDrawingCanvasOptions {
  canvasRef: RefObject<HTMLCanvasElement | null>;
  captureWidth: number;
  captureHeight: number;
}

interface UseDrawingCanvasResult {
  drawMode: boolean;
  drawColor: string;
  lineWidth: number;
  toggleDrawMode: () => void;
  setDrawColor: (color: string) => void;
  setLineWidth: (width: number) => void;
  clearCanvas: () => void;
  handlePointerDown: (e: PointerEvent) => void;
  handlePointerMove: (e: PointerEvent) => void;
  handlePointerUp: () => void;
  handlePointerLeave: () => void;
}

export function useDrawingCanvas({
  canvasRef,
  captureWidth,
  captureHeight,
}: UseDrawingCanvasOptions): UseDrawingCanvasResult {
  const [drawMode, setDrawMode] = useState(false);
  const [drawColor, setDrawColor] = useState("#ff0000");
  const [lineWidth, setLineWidth] = useState(3);
  const isDrawing = useRef(false);
  const lastPosition = useRef({ x: 0, y: 0 });

  const scalePointerToCanvas = useCallback(
    (e: PointerEvent) => {
      const canvas = canvasRef.current;
      if (!canvas) return { x: 0, y: 0 };
      const rect = canvas.getBoundingClientRect();
      const scaleX = captureWidth / rect.width;
      const scaleY = captureHeight / rect.height;
      return {
        x: (e.clientX - rect.left) * scaleX,
        y: (e.clientY - rect.top) * scaleY,
      };
    },
    [canvasRef, captureWidth, captureHeight],
  );

  const toggleDrawMode = useCallback(() => {
    setDrawMode((prev) => !prev);
  }, []);

  const clearCanvas = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (ctx) ctx.clearRect(0, 0, captureWidth, captureHeight);
  }, [canvasRef, captureWidth, captureHeight]);

  const handlePointerDown = useCallback(
    (e: PointerEvent) => {
      if (!drawMode) return;
      isDrawing.current = true;
      lastPosition.current = scalePointerToCanvas(e);
    },
    [drawMode, scalePointerToCanvas],
  );

  const handlePointerMove = useCallback(
    (e: PointerEvent) => {
      if (!drawMode || !isDrawing.current) return;
      const canvas = canvasRef.current;
      if (!canvas) return;
      const ctx = canvas.getContext("2d");
      if (!ctx) return;

      const pos = scalePointerToCanvas(e);
      ctx.lineWidth = lineWidth;
      ctx.lineCap = "round";
      ctx.lineJoin = "round";
      ctx.strokeStyle = drawColor;
      ctx.beginPath();
      ctx.moveTo(lastPosition.current.x, lastPosition.current.y);
      ctx.lineTo(pos.x, pos.y);
      ctx.stroke();
      lastPosition.current = pos;
    },
    [drawMode, drawColor, lineWidth, canvasRef, scalePointerToCanvas],
  );

  const handlePointerUp = useCallback(() => {
    isDrawing.current = false;
  }, []);

  const handlePointerLeave = useCallback(() => {
    isDrawing.current = false;
  }, []);

  return {
    drawMode,
    drawColor,
    lineWidth,
    toggleDrawMode,
    setDrawColor,
    setLineWidth,
    clearCanvas,
    handlePointerDown,
    handlePointerMove,
    handlePointerUp,
    handlePointerLeave,
  };
}
