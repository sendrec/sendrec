import { useCallback, useEffect, useRef, useState } from "react";

export const MIN_RECORDING_SECONDS = 1;
export const MIN_RECORDING_BYTES = 1024;

export type RecordingState = "idle" | "countdown" | "recording" | "paused" | "stopped";

export interface UseRecordingResult {
  state: RecordingState;
  setState: (state: RecordingState) => void;
  elapsed: number;
  countdown: number;
  setCountdown: (value: number) => void;
  countdownTimerRef: React.MutableRefObject<ReturnType<typeof setInterval>>;
  elapsedSeconds: () => number;
  startTimer: () => void;
  stopTimer: () => void;
  pauseStartRef: React.MutableRefObject<number>;
  totalPausedRef: React.MutableRefObject<number>;
  startTimeRef: React.MutableRefObject<number>;
  isIdle: boolean;
  isCountdown: boolean;
  isRecording: boolean;
  isPaused: boolean;
  isActive: boolean;
  isStopped: boolean;
  remaining: number | null;
  reset: () => void;
}

export function useRecording(
  maxDurationSeconds: number,
  onCountdownComplete: () => void,
  onMaxDurationReached: () => void,
): UseRecordingResult {
  const [state, setStateRaw] = useState<RecordingState>("idle");
  const [elapsed, setElapsed] = useState(0);
  const [countdown, setCountdown] = useState(3);

  const startTimeRef = useRef<number>(0);
  const timerRef = useRef<ReturnType<typeof setInterval>>(0 as unknown as ReturnType<typeof setInterval>);
  const countdownTimerRef = useRef<ReturnType<typeof setInterval>>(0 as unknown as ReturnType<typeof setInterval>);
  const pauseStartRef = useRef<number>(0);
  const totalPausedRef = useRef<number>(0);
  const onCountdownCompleteRef = useRef(onCountdownComplete);
  const onMaxDurationReachedRef = useRef(onMaxDurationReached);
  onCountdownCompleteRef.current = onCountdownComplete;
  onMaxDurationReachedRef.current = onMaxDurationReached;

  const setState = useCallback((next: RecordingState) => {
    setStateRaw(next);
  }, []);

  const elapsedSeconds = useCallback(() => {
    return Math.floor((Date.now() - startTimeRef.current - totalPausedRef.current) / 1000);
  }, []);

  const stopTimer = useCallback(() => {
    clearInterval(timerRef.current);
  }, []);

  const startTimer = useCallback(() => {
    timerRef.current = setInterval(() => {
      setElapsed(elapsedSeconds());
    }, 1000);
  }, [elapsedSeconds]);

  const reset = useCallback(() => {
    stopTimer();
    clearInterval(countdownTimerRef.current);
    setStateRaw("idle");
    setElapsed(0);
    setCountdown(3);
    startTimeRef.current = 0;
    pauseStartRef.current = 0;
    totalPausedRef.current = 0;
  }, [stopTimer]);

  useEffect(() => {
    if (state !== "countdown") return;
    countdownTimerRef.current = setInterval(() => {
      setCountdown((prev) => {
        if (prev <= 1) {
          onCountdownCompleteRef.current();
          return 3;
        }
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(countdownTimerRef.current);
  }, [state]);

  useEffect(() => {
    if (
      state === "recording" &&
      maxDurationSeconds > 0 &&
      elapsed >= maxDurationSeconds
    ) {
      onMaxDurationReachedRef.current();
    }
  }, [elapsed, maxDurationSeconds, state]);

  useEffect(() => {
    return () => {
      clearInterval(timerRef.current);
      clearInterval(countdownTimerRef.current);
    };
  }, []);

  const isIdle = state === "idle";
  const isCountdown = state === "countdown";
  const isPaused = state === "paused";
  const isStopped = state === "stopped";
  const isActive = !isIdle && !isStopped;
  const isRecording = state === "recording" || isPaused;
  const remaining = maxDurationSeconds > 0 ? maxDurationSeconds - elapsed : null;

  return {
    state,
    setState,
    elapsed,
    countdown,
    setCountdown,
    countdownTimerRef,
    elapsedSeconds,
    startTimer,
    stopTimer,
    pauseStartRef,
    totalPausedRef,
    startTimeRef,
    isIdle,
    isCountdown,
    isRecording,
    isPaused,
    isActive,
    isStopped,
    remaining,
    reset,
  };
}
