import { useCallback, useEffect, useRef } from "react";
import type { RefObject } from "react";

interface UseCanvasCompositingOptions {
  compositingCanvasRef: RefObject<HTMLCanvasElement | null>;
  screenVideoRef: RefObject<HTMLVideoElement | null>;
  drawingCanvasRef: RefObject<HTMLCanvasElement | null>;
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

    ctx.drawImage(video, 0, 0);
    if (drawing) {
      ctx.drawImage(drawing, 0, 0);
    }

    animFrameRef.current = requestAnimationFrame(compositeFrame);
  }, [compositingCanvasRef, screenVideoRef, drawingCanvasRef]);

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

      // Draw an initial frame before capturing so the stream starts with video content.
      // Without this, Chrome's MP4 MediaRecorder may fail to initialize the video track
      // if captureStream() is called before requestAnimationFrame paints the first frame.
      const video = screenVideoRef.current;
      if (video) {
        const ctx = canvas.getContext("2d");
        if (ctx) {
          ctx.drawImage(video, 0, 0);
        }
      }

      // Use a fixed framerate to ensure consistent video frame production.
      // captureStream() without args only captures on canvas changes, which can miss
      // frames if the compositing loop hasn't started yet.
      const stream = canvas.captureStream(30);
      audioTracks.forEach((track) => stream.addTrack(track));
      return stream;
    },
    [compositingCanvasRef, screenVideoRef],
  );

  useEffect(() => {
    return () => {
      isRunning.current = false;
      cancelAnimationFrame(animFrameRef.current);
    };
  }, []);

  return { startCompositing, stopCompositing, getCompositedStream };
}
