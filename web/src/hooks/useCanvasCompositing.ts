import { useCallback, useEffect, useRef } from "react";
import type { RefObject } from "react";

interface UseCanvasCompositingOptions {
  compositingCanvasRef: RefObject<HTMLCanvasElement | null>;
  screenVideoRef: RefObject<HTMLVideoElement | null>;
  drawingCanvasRef: RefObject<HTMLCanvasElement | null>;
  captureWidth: number;
  captureHeight: number;
}

interface UseCanvasCompositingResult {
  startCompositing: () => void;
  stopCompositing: () => void;
  getCompositedStream: (audioTracks: MediaStreamTrack[]) => MediaStream | null;
}

export function useCanvasCompositing({
  compositingCanvasRef,
  screenVideoRef,
  drawingCanvasRef,
  captureWidth,
  captureHeight,
}: UseCanvasCompositingOptions): UseCanvasCompositingResult {
  const animFrameRef = useRef(0);
  const isRunning = useRef(false);

  const compositeFrame = useCallback(() => {
    if (!isRunning.current) return;
    const canvas = compositingCanvasRef.current;
    const video = screenVideoRef.current;
    const drawing = drawingCanvasRef.current;
    if (!canvas || !video) return;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    ctx.drawImage(video, 0, 0, captureWidth, captureHeight);
    if (drawing) {
      ctx.drawImage(drawing, 0, 0, captureWidth, captureHeight);
    }

    animFrameRef.current = requestAnimationFrame(compositeFrame);
  }, [compositingCanvasRef, screenVideoRef, drawingCanvasRef, captureWidth, captureHeight]);

  const startCompositing = useCallback(() => {
    isRunning.current = true;
    animFrameRef.current = requestAnimationFrame(compositeFrame);
  }, [compositeFrame]);

  const stopCompositing = useCallback(() => {
    isRunning.current = false;
    cancelAnimationFrame(animFrameRef.current);
  }, []);

  const getCompositedStream = useCallback(
    (audioTracks: MediaStreamTrack[]) => {
      const canvas = compositingCanvasRef.current;
      if (!canvas) return null;

      const stream = canvas.captureStream(30);
      audioTracks.forEach((track) => stream.addTrack(track));
      return stream;
    },
    [compositingCanvasRef],
  );

  useEffect(() => {
    return () => {
      isRunning.current = false;
      cancelAnimationFrame(animFrameRef.current);
    };
  }, []);

  return { startCompositing, stopCompositing, getCompositedStream };
}
