import { useCallback, useRef, useState } from "react";

/**
 * Watches a capture track for the stalls that turn a recording into sound over
 * a frozen image.
 *
 * A browser mutes a capture track when it stops delivering frames — WebKit does
 * it whenever the page loses visibility, which is every time the user switches
 * to the app they are demoing — and a muted video track repeats its last frame
 * while the microphone keeps recording. The track's mute event is the only
 * notice the page gets, so the recorders listen for it and stop before handing
 * the user an unusable video.
 *
 * Only time the recorder spends actually recording counts: a capture muted
 * while the recording is paused loses nothing.
 */
export function useCaptureStallWatch() {
  const [stalled, setStalled] = useState(false);
  const mutedRef = useRef(false);
  const countingRef = useRef(false);
  const stalledSinceRef = useRef<number | null>(null);
  const totalRef = useRef(0);

  const closeInterval = useCallback(() => {
    if (stalledSinceRef.current !== null) {
      totalRef.current += Date.now() - stalledSinceRef.current;
      stalledSinceRef.current = null;
    }
  }, []);

  /** Starts a fresh tally on a new capture. Listeners die with the track. */
  const watch = useCallback(
    (track: MediaStreamTrack | undefined) => {
      // Start from the track's own state: a capture that never produced a frame
      // is muted from the outset, and a track that starts muted fires no event.
      const startsMuted = track?.muted ?? false;
      mutedRef.current = startsMuted;
      countingRef.current = false;
      stalledSinceRef.current = null;
      totalRef.current = 0;
      setStalled(startsMuted);
      if (!track) return;

      track.addEventListener("mute", () => {
        mutedRef.current = true;
        setStalled(true);
        if (countingRef.current && stalledSinceRef.current === null) {
          stalledSinceRef.current = Date.now();
        }
      });
      track.addEventListener("unmute", () => {
        mutedRef.current = false;
        setStalled(false);
        closeInterval();
      });
    },
    [closeInterval],
  );

  /** True while the recorder is running: start and resume on, pause and stop off. */
  const count = useCallback(
    (on: boolean) => {
      if (on) {
        countingRef.current = true;
        // A capture already muted when recording starts loses the first frame
        // onwards — that is the case where the whole video is one still image.
        if (mutedRef.current && stalledSinceRef.current === null) {
          stalledSinceRef.current = Date.now();
        }
        return;
      }
      closeInterval();
      countingRef.current = false;
    },
    [closeInterval],
  );

  const stalledMs = useCallback(() => {
    const open =
      stalledSinceRef.current === null ? 0 : Date.now() - stalledSinceRef.current;
    return totalRef.current + open;
  }, []);

  return { stalled, watch, count, stalledMs };
}
