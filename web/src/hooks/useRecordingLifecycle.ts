import { useCallback, useEffect, useRef, useState } from "react";

export type RecordingCommand = "start" | "pause" | "resume" | "stop";
export type RecordingState = "idle" | "countdown" | "recording" | "paused" | "stopped";
export type RecordingEvent =
  | { type: "request-start"; countdown: boolean }
  | { type: "start-now" }
  | { type: "pause" }
  | { type: "resume" }
  | { type: "stop" }
  | { type: "cancel-countdown" };

export interface RecordingLifecycle {
  snapshot: {
    state: RecordingState;
    elapsed: number;
    countdown: number;
    remaining: number | null;
  };
  dispatch(event: RecordingEvent): void;
  elapsedSeconds(): number;
}

interface UseRecordingLifecycleOptions {
  maxDurationSeconds: number;
  perform(command: RecordingCommand): boolean | void;
}

export function useRecordingLifecycle({
  maxDurationSeconds,
  perform,
}: UseRecordingLifecycleOptions): RecordingLifecycle {
  const [state, setState] = useState<RecordingState>("idle");
  const [elapsed, setElapsed] = useState(0);
  const [countdown, setCountdown] = useState(3);

  const phaseRef = useRef<RecordingState>("idle");
  const performRef = useRef(perform);
  const elapsedTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const countdownTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const startTimeRef = useRef(0);
  const pauseStartRef = useRef(0);
  const totalPausedRef = useRef(0);
  const countdownValueRef = useRef(3);
  const dispatchRef = useRef<(event: RecordingEvent) => void>(() => {});

  performRef.current = perform;

  const clearElapsedTimer = useCallback(() => {
    if (elapsedTimerRef.current !== null) clearInterval(elapsedTimerRef.current);
    elapsedTimerRef.current = null;
  }, []);

  const clearCountdownTimer = useCallback(() => {
    if (countdownTimerRef.current !== null) clearInterval(countdownTimerRef.current);
    countdownTimerRef.current = null;
  }, []);

  const elapsedSeconds = useCallback(() => (
    Math.floor((Date.now() - startTimeRef.current - totalPausedRef.current) / 1000)
  ), []);

  const enter = useCallback((next: RecordingState) => {
    phaseRef.current = next;
    setState(next);
  }, []);

  const startElapsedTimer = useCallback(() => {
    clearElapsedTimer();
    elapsedTimerRef.current = setInterval(() => {
      setElapsed(elapsedSeconds());
    }, 1000);
  }, [clearElapsedTimer, elapsedSeconds]);

  const beginRecording = useCallback(() => {
    clearCountdownTimer();
    performRef.current("start");
    startTimeRef.current = Date.now();
    countdownValueRef.current = 3;
    setCountdown(3);
    enter("recording");
    startElapsedTimer();
  }, [clearCountdownTimer, enter, startElapsedTimer]);

  const dispatch = useCallback((event: RecordingEvent) => {
    const phase = phaseRef.current;

    if (event.type === "request-start") {
      if (phase !== "idle") return;
      clearElapsedTimer();
      clearCountdownTimer();
      startTimeRef.current = 0;
      pauseStartRef.current = 0;
      totalPausedRef.current = 0;
      countdownValueRef.current = 3;
      setElapsed(0);
      setCountdown(3);

      if (!event.countdown) {
        beginRecording();
        return;
      }

      enter("countdown");
      countdownTimerRef.current = setInterval(() => {
        const next = countdownValueRef.current - 1;
        if (next <= 0) {
          dispatchRef.current({ type: "start-now" });
          return;
        }
        countdownValueRef.current = next;
        setCountdown(next);
      }, 1000);
      return;
    }

    if (event.type === "start-now") {
      if (phase !== "countdown") return;
      beginRecording();
      return;
    }

    if (event.type === "pause") {
      if (phase !== "recording") return;
      if (performRef.current("pause") === false) return;
      pauseStartRef.current = Date.now();
      clearElapsedTimer();
      enter("paused");
      return;
    }

    if (event.type === "resume") {
      if (phase !== "paused") return;
      if (performRef.current("resume") === false) return;
      totalPausedRef.current += Date.now() - pauseStartRef.current;
      startElapsedTimer();
      enter("recording");
      return;
    }

    if (event.type === "stop") {
      if (phase !== "recording" && phase !== "paused") return;
      if (phase === "paused") {
        totalPausedRef.current += Date.now() - pauseStartRef.current;
      }
      performRef.current("stop");
      clearElapsedTimer();
      enter("stopped");
      return;
    }

    if (phase !== "countdown") return;
    clearCountdownTimer();
    clearElapsedTimer();
    startTimeRef.current = 0;
    pauseStartRef.current = 0;
    totalPausedRef.current = 0;
    countdownValueRef.current = 3;
    setElapsed(0);
    setCountdown(3);
    enter("idle");
  }, [beginRecording, clearCountdownTimer, clearElapsedTimer, enter, startElapsedTimer]);

  dispatchRef.current = dispatch;

  useEffect(() => {
    if (state === "recording" && maxDurationSeconds > 0 && elapsed >= maxDurationSeconds) {
      dispatch({ type: "stop" });
    }
  }, [dispatch, elapsed, maxDurationSeconds, state]);

  useEffect(() => () => {
    clearElapsedTimer();
    clearCountdownTimer();
  }, [clearCountdownTimer, clearElapsedTimer]);

  return {
    snapshot: {
      state,
      elapsed,
      countdown,
      remaining: maxDurationSeconds > 0 ? maxDurationSeconds - elapsed : null,
    },
    dispatch,
    elapsedSeconds,
  };
}
