// Chrome-only "breakout box" types (mediacapture-transform). Not in lib.dom.
type TrackProcessor = new (init: { track: MediaStreamTrack }) => {
  readable: ReadableStream<VideoFrame>;
};
type TrackGenerator = new (init: { kind: "video" }) => MediaStreamTrack & {
  writable: WritableStream<VideoFrame>;
};

function breakoutBox() {
  return globalThis as unknown as {
    MediaStreamTrackProcessor?: TrackProcessor;
    MediaStreamTrackGenerator?: TrackGenerator;
  };
}

/** Whether this browser can put annotations into the recording, not just the preview. */
export function canRecordAnnotations(): boolean {
  const api = breakoutBox();
  return Boolean(api.MediaStreamTrackProcessor && api.MediaStreamTrackGenerator);
}

/**
 * Burns the drawing canvas into the frames of a screen-capture track.
 *
 * The recorder records the raw display track rather than a canvas, because
 * canvas compositing freezes when the recording tab is backgrounded — both
 * rAF and setInterval are throttled there. That is why annotations never
 * reached the recording. Transforming the track itself is driven by frame
 * arrival instead of a timer, so it survives a hidden tab.
 *
 * Returns the original track unchanged where the API is missing (Firefox,
 * Safari): annotations stay preview-only there rather than breaking recording.
 */
export function overlayDrawingOnTrack(
  track: MediaStreamTrack,
  drawingCanvas: HTMLCanvasElement,
  hasDrawing: () => boolean,
): MediaStreamTrack {
  const api = breakoutBox();
  if (!api.MediaStreamTrackProcessor || !api.MediaStreamTrackGenerator) return track;

  const scratch = document.createElement("canvas");
  const ctx = scratch.getContext("2d");
  if (!ctx) return track;

  const generator = new api.MediaStreamTrackGenerator({ kind: "video" });

  let warned = false;

  // Returns null when this frame cannot be composited — a frame allocation can
  // fail under memory pressure at capture resolution.
  function composite(frame: VideoFrame): VideoFrame | null {
    try {
      if (scratch.width !== frame.displayWidth || scratch.height !== frame.displayHeight) {
        scratch.width = frame.displayWidth;
        scratch.height = frame.displayHeight;
      }
      ctx!.drawImage(frame, 0, 0, scratch.width, scratch.height);
      ctx!.drawImage(drawingCanvas, 0, 0, scratch.width, scratch.height);
      return new VideoFrame(scratch, {
        timestamp: frame.timestamp,
        ...(frame.duration == null ? {} : { duration: frame.duration }),
      });
    } catch (err) {
      // Once, not per frame: a persistent failure runs at the capture framerate.
      if (!warned) {
        warned = true;
        console.warn("Annotation compositing failed, recording without it", err);
      }
      return null;
    }
  }

  new api.MediaStreamTrackProcessor({ track }).readable
    .pipeThrough(
      new TransformStream<VideoFrame, VideoFrame>({
        transform(frame, controller) {
          // Nothing drawn yet: hand the frame straight through, so a recording
          // without annotations costs no per-frame copy at capture resolution.
          if (!hasDrawing()) {
            controller.enqueue(frame);
            return;
          }
          const composited = composite(frame);
          if (!composited) {
            // Record the plain capture for this frame. Throwing instead would
            // end the generated track and leave the rest of the recording
            // audio-only — and the source track stays live, so nothing in
            // Recorder would notice.
            controller.enqueue(frame);
            return;
          }
          frame.close();
          controller.enqueue(composited);
        },
      }),
    )
    .pipeTo(generator.writable)
    .catch(() => {
      // The source track ending on stop rejects the pipe; nothing to do.
    });

  return generator;
}
