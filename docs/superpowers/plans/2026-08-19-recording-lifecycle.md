# Recording Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the shallow recording hook with a deep Recording lifecycle module while preserving every observable screen and camera recording behavior.

**Architecture:** Add one event-driven React hook with a stable `dispatch` interface and a single `perform(command)` adapter seam. Keep browser media operations in `Recorder` and `CameraRecorder`; move only phase transitions, countdown, elapsed timing, pause accounting, maximum-duration stopping, and timer cleanup into the hook.

**Tech Stack:** React 19, TypeScript 5.9, Vitest 4, React Testing Library, jsdom

**Spec:** `docs/superpowers/specs/2026-08-19-recording-lifecycle-design.md`

## Global Constraints

- No intended route, persistence, network, UI, copy, accessibility, media-format, quality, capture-constraint, or browser-support changes.
- Keep `MIN_RECORDING_SECONDS` at `1` and `MIN_RECORDING_BYTES` at `1024`.
- Keep the countdown at three one-second ticks and preserve click-to-start-now.
- Keep output validation, MIME fallback, webcam handling, stream cleanup, errors, and completion callbacks in their current callers.
- Add no dependency, class, factory, or extra adapter seam.
- Follow TDD and keep every commit passing its focused tests and TypeScript type check.

---

## File map

- Create `web/src/hooks/useRecordingLifecycle.ts`: the Recording lifecycle interface and implementation.
- Create `web/src/hooks/useRecordingLifecycle.test.ts`: tests only through the lifecycle interface.
- Create `web/src/utils/recordingLimits.ts`: the two shared output-validation constants currently misplaced in the shallow hook.
- Modify `web/src/components/CameraRecorder.tsx`: camera `perform(command)` adapter and event dispatches.
- Modify `web/src/components/CameraRecorder.test.tsx`: retain camera-specific behavior coverage after migration.
- Modify `web/src/components/Recorder.tsx`: screen `perform(command)` adapter, event dispatches, and countdown cancellation.
- Modify `web/src/components/Recorder.test.tsx`: characterize screen-share cancellation and runtime encoder fallback before migration.
- Delete `web/src/hooks/useRecording.ts`: remove the shallow interface after both callers migrate.

### Task 1: Characterize uncovered screen recording behavior

**Files:**
- Modify: `web/src/components/Recorder.test.tsx:64-91`
- Modify: `web/src/components/Recorder.test.tsx:588-617`

**Interfaces:**
- Consumes: current `Recorder` behavior and the existing `MockMediaRecorder`.
- Produces: regression coverage for countdown cancellation and runtime MP4-to-WebM fallback.

- [ ] **Step 1: Track recorder instances and expose encoder errors in the test fake**

Add instance tracking before `MockMediaRecorder` and extend the fake without changing production code:

```ts
const mediaRecorderInstances: MockMediaRecorder[] = [];

class MockMediaRecorder {
  static isTypeSupported = vi.fn().mockReturnValue(true);
  state = "inactive";
  ondataavailable: ((event: { data: Blob }) => void) | null = null;
  onstop: (() => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  constructor() {
    mediaRecorderInstances.push(this);
  }

  // Keep the existing start, stop, pause, and resume fakes unchanged.
}
```

Reset the list in `beforeEach`:

```ts
mediaRecorderInstances.length = 0;
```

- [ ] **Step 2: Add the countdown-cancellation characterization test**

Place this beside the existing screen-track-ended test:

```ts
it("returns to idle when screen sharing ends during countdown", async () => {
  let endedCallback: (() => void) | undefined;
  const stopTrack = vi.fn();
  mockScreenStream.getVideoTracks.mockReturnValue([{
    getSettings: () => ({ width: 1920, height: 1080 }),
    addEventListener: vi.fn((event: string, handler: () => void) => {
      if (event === "ended") endedCallback = handler;
    }),
    stop: stopTrack,
  }]);
  mockScreenStream.getTracks.mockReturnValue([{ stop: stopTrack }]);

  const user = userEvent.setup();
  render(<Recorder onRecordingComplete={vi.fn()} />);
  await user.click(screen.getByRole("button", { name: "Start recording" }));
  expect(screen.getByTestId("countdown-overlay")).toBeInTheDocument();

  act(() => endedCallback!());

  expect(screen.queryByTestId("countdown-overlay")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Start recording" })).toBeInTheDocument();
  expect(mockStopCompositing).toHaveBeenCalled();
  expect(stopTrack).toHaveBeenCalled();
  expect(mediaRecorderInstances[0].stop).not.toHaveBeenCalled();
});
```

- [ ] **Step 3: Add the runtime encoder-fallback characterization test**

```ts
it("falls back from a runtime MP4 encoder error to WebM", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  MockMediaRecorder.isTypeSupported = vi.fn().mockReturnValue(true);
  const onComplete = vi.fn();
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

  render(<Recorder onRecordingComplete={onComplete} />);
  await user.click(screen.getByRole("button", { name: "Start recording" }));
  await user.click(screen.getByTestId("countdown-overlay"));

  act(() => mediaRecorderInstances[0].onerror?.(new Event("error")));
  expect(mediaRecorderInstances).toHaveLength(2);
  expect(mediaRecorderInstances[1].start).toHaveBeenCalledTimes(1);

  await act(() => vi.advanceTimersByTime(1500));
  await user.click(screen.getByRole("button", { name: "Stop recording" }));
  await vi.waitFor(() => expect(onComplete).toHaveBeenCalledTimes(1));

  expect((onComplete.mock.calls[0][0] as Blob).type).toBe("video/webm");
  vi.useRealTimers();
});
```

- [ ] **Step 4: Run the focused tests and verify they pass against current behavior**

Run:

```bash
cd web && pnpm test -- src/components/Recorder.test.tsx
```

Expected: both new characterization tests and the existing Recorder tests pass. If either new test exposes an existing bug, preserve the current production behavior, record the discrepancy, and do not fold a bug fix into this refactor.

- [ ] **Step 5: Commit the characterization tests**

```bash
git add web/src/components/Recorder.test.tsx
git commit -m "test: characterize recording edge cases"
```

### Task 2: Build the Recording lifecycle module with TDD

**Files:**
- Create: `web/src/hooks/useRecordingLifecycle.test.ts`
- Create: `web/src/hooks/useRecordingLifecycle.ts`
- Create: `web/src/utils/recordingLimits.ts`

**Interfaces:**
- Consumes: React hooks, `Date.now`, `setInterval`, and a caller-supplied `perform(command)` adapter.
- Produces: `RecordingCommand`, `RecordingEvent`, `RecordingState`, `RecordingLifecycle`, and `useRecordingLifecycle(options)` exactly as specified in the design.

- [ ] **Step 1: Write the failing lifecycle interface tests**

Create `web/src/hooks/useRecordingLifecycle.test.ts` with fake timers and tests through the public interface:

```ts
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useRecordingLifecycle } from "./useRecordingLifecycle";

describe("useRecordingLifecycle", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-01-01T00:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("counts down from three and starts through the adapter", () => {
    const perform = vi.fn();
    const { result } = renderHook(() =>
      useRecordingLifecycle({ maxDurationSeconds: 0, perform }),
    );

    act(() => result.current.dispatch({ type: "request-start", countdown: true }));
    expect(result.current.snapshot).toMatchObject({ state: "countdown", countdown: 3 });

    act(() => vi.advanceTimersByTime(1000));
    expect(result.current.snapshot.countdown).toBe(2);
    act(() => vi.advanceTimersByTime(1000));
    expect(result.current.snapshot.countdown).toBe(1);
    act(() => vi.advanceTimersByTime(1000));

    expect(perform).toHaveBeenCalledTimes(1);
    expect(perform).toHaveBeenCalledWith("start");
    expect(result.current.snapshot).toMatchObject({
      state: "recording",
      countdown: 3,
      remaining: null,
    });
  });

  it("starts immediately when countdown is skipped", () => {
    const perform = vi.fn();
    const { result } = renderHook(() =>
      useRecordingLifecycle({ maxDurationSeconds: 0, perform }),
    );
    act(() => result.current.dispatch({ type: "request-start", countdown: true }));
    act(() => result.current.dispatch({ type: "start-now" }));
    act(() => vi.advanceTimersByTime(3000));

    expect(perform).toHaveBeenCalledTimes(1);
    expect(perform).toHaveBeenCalledWith("start");
    expect(result.current.snapshot.state).toBe("recording");
  });

  it("excludes multiple paused intervals from final duration", () => {
    const perform = vi.fn();
    const { result } = renderHook(() =>
      useRecordingLifecycle({ maxDurationSeconds: 0, perform }),
    );
    act(() => result.current.dispatch({ type: "request-start", countdown: false }));
    act(() => vi.advanceTimersByTime(2500));
    act(() => result.current.dispatch({ type: "pause" }));
    act(() => vi.advanceTimersByTime(4000));
    act(() => result.current.dispatch({ type: "resume" }));
    act(() => vi.advanceTimersByTime(1500));
    act(() => result.current.dispatch({ type: "pause" }));
    act(() => vi.advanceTimersByTime(3000));
    act(() => result.current.dispatch({ type: "stop" }));

    expect(result.current.elapsedSeconds()).toBe(4);
    expect(perform.mock.calls.map(([command]) => command)).toEqual([
      "start", "pause", "resume", "pause", "stop",
    ]);
    expect(result.current.snapshot.state).toBe("stopped");
  });

  it("does not reach the maximum duration while paused", () => {
    const perform = vi.fn();
    const { result } = renderHook(() =>
      useRecordingLifecycle({ maxDurationSeconds: 2, perform }),
    );
    act(() => result.current.dispatch({ type: "request-start", countdown: false }));
    act(() => vi.advanceTimersByTime(1000));
    act(() => result.current.dispatch({ type: "pause" }));
    act(() => vi.advanceTimersByTime(10_000));
    expect(perform).not.toHaveBeenCalledWith("stop");

    act(() => result.current.dispatch({ type: "resume" }));
    act(() => vi.advanceTimersByTime(1000));
    expect(perform).toHaveBeenLastCalledWith("stop");
    expect(result.current.snapshot.state).toBe("stopped");
  });

  it("cancels countdown without issuing a media command", () => {
    const perform = vi.fn();
    const { result } = renderHook(() =>
      useRecordingLifecycle({ maxDurationSeconds: 0, perform }),
    );
    act(() => result.current.dispatch({ type: "request-start", countdown: true }));
    act(() => result.current.dispatch({ type: "cancel-countdown" }));
    act(() => vi.advanceTimersByTime(5000));

    expect(perform).not.toHaveBeenCalled();
    expect(result.current.snapshot).toMatchObject({ state: "idle", elapsed: 0, countdown: 3 });
  });

  it("uses the latest adapter after rerender without restarting countdown", () => {
    const first = vi.fn();
    const second = vi.fn();
    const { result, rerender } = renderHook(
      ({ perform }) => useRecordingLifecycle({ maxDurationSeconds: 0, perform }),
      { initialProps: { perform: first } },
    );
    act(() => result.current.dispatch({ type: "request-start", countdown: true }));
    act(() => vi.advanceTimersByTime(1000));
    rerender({ perform: second });
    act(() => vi.advanceTimersByTime(2000));

    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledTimes(1);
    expect(second).toHaveBeenCalledWith("start");
  });

  it("ignores duplicate and invalid events before a render", () => {
    const perform = vi.fn();
    const { result } = renderHook(() =>
      useRecordingLifecycle({ maxDurationSeconds: 0, perform }),
    );
    act(() => {
      result.current.dispatch({ type: "request-start", countdown: false });
      result.current.dispatch({ type: "request-start", countdown: false });
      result.current.dispatch({ type: "resume" });
    });
    expect(perform.mock.calls).toEqual([["start"]]);
  });

  it("stops timers on unmount", () => {
    const perform = vi.fn();
    const { result, unmount } = renderHook(() =>
      useRecordingLifecycle({ maxDurationSeconds: 1, perform }),
    );
    act(() => result.current.dispatch({ type: "request-start", countdown: false }));
    unmount();
    act(() => vi.advanceTimersByTime(5000));
    expect(perform).not.toHaveBeenCalledWith("stop");
  });

  it("keeps the current phase when the adapter throws", () => {
    const perform = vi.fn((command: string) => {
      if (command === "pause") throw new Error("pause failed");
    });
    const { result } = renderHook(() =>
      useRecordingLifecycle({ maxDurationSeconds: 0, perform }),
    );
    act(() => result.current.dispatch({ type: "request-start", countdown: false }));

    expect(() => {
      act(() => result.current.dispatch({ type: "pause" }));
    }).toThrow("pause failed");
    expect(result.current.snapshot.state).toBe("recording");
  });

  it("uses the latest maximum duration after rerender", () => {
    const perform = vi.fn();
    const { result, rerender } = renderHook(
      ({ maximum }) => useRecordingLifecycle({
        maxDurationSeconds: maximum,
        perform,
      }),
      { initialProps: { maximum: 10 } },
    );
    act(() => result.current.dispatch({ type: "request-start", countdown: false }));
    rerender({ maximum: 1 });
    act(() => vi.advanceTimersByTime(1000));

    expect(perform.mock.calls.map(([command]) => command)).toEqual(["start", "stop"]);
    expect(result.current.snapshot.state).toBe("stopped");
  });
});
```

- [ ] **Step 2: Run the lifecycle test and verify RED**

Run:

```bash
cd web && pnpm test -- src/hooks/useRecordingLifecycle.test.ts
```

Expected: FAIL because `./useRecordingLifecycle` does not exist.

- [ ] **Step 3: Add the shared recording limits**

Create `web/src/utils/recordingLimits.ts`:

```ts
export const MIN_RECORDING_SECONDS = 1;
export const MIN_RECORDING_BYTES = 1024;
```

- [ ] **Step 4: Implement the minimal lifecycle interface**

Create `web/src/hooks/useRecordingLifecycle.ts`. Use the exact exported types from the spec and these implementation rules:

```ts
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
  perform(command: RecordingCommand): void;
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
  const maxDurationRef = useRef(maxDurationSeconds);
  const elapsedTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const countdownTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const startTimeRef = useRef(0);
  const pauseStartRef = useRef(0);
  const totalPausedRef = useRef(0);
  const countdownValueRef = useRef(3);
  const dispatchRef = useRef<(event: RecordingEvent) => void>(() => {});

  performRef.current = perform;
  maxDurationRef.current = maxDurationSeconds;

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
      const next = elapsedSeconds();
      setElapsed(next);
      const maximum = maxDurationRef.current;
      if (maximum > 0 && next >= maximum) {
        dispatchRef.current({ type: "stop" });
      }
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
      performRef.current("pause");
      pauseStartRef.current = Date.now();
      clearElapsedTimer();
      enter("paused");
      return;
    }

    if (event.type === "resume") {
      if (phase !== "paused") return;
      totalPausedRef.current += Date.now() - pauseStartRef.current;
      performRef.current("resume");
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
```

- [ ] **Step 5: Run the lifecycle tests and verify GREEN**

Run:

```bash
cd web && pnpm test -- src/hooks/useRecordingLifecycle.test.ts
cd web && pnpm typecheck
```

Expected: all lifecycle tests pass and TypeScript reports no errors.

- [ ] **Step 6: Commit the deep lifecycle module**

```bash
git add web/src/hooks/useRecordingLifecycle.ts web/src/hooks/useRecordingLifecycle.test.ts web/src/utils/recordingLimits.ts
git commit -m "refactor: add recording lifecycle module"
```

### Task 3: Migrate the camera adapter

**Files:**
- Modify: `web/src/components/CameraRecorder.tsx:1-153`
- Modify: `web/src/components/CameraRecorder.test.tsx:181-326`

**Interfaces:**
- Consumes: `useRecordingLifecycle({ maxDurationSeconds, perform })`, `RecordingCommand`, and shared recording limits.
- Produces: a camera adapter that maps lifecycle commands to one `MediaRecorder` while preserving rendered controls and completion behavior.

- [ ] **Step 1: Run the camera suite as the green refactor baseline**

```bash
cd web && pnpm test -- src/components/CameraRecorder.test.tsx
```

Expected: PASS before production edits.

- [ ] **Step 2: Replace mutable lifecycle access with a camera command adapter**

Replace imports from `useRecording` with:

```ts
import {
  useRecordingLifecycle,
  type RecordingCommand,
} from "../hooks/useRecordingLifecycle";
import { MIN_RECORDING_BYTES, MIN_RECORDING_SECONDS } from "../utils/recordingLimits";
```

Replace callback refs and the old hook call with:

```ts
const performRecordingCommand = useCallback((command: RecordingCommand) => {
  const recorder = mediaRecorderRef.current;
  if (!recorder) return;

  if (command === "start") recorder.start(1000);
  if (command === "pause" && recorder.state === "recording") recorder.pause();
  if (command === "resume" && recorder.state === "paused") recorder.resume();
  if (command === "stop" && recorder.state !== "inactive") recorder.stop();
}, []);

const recording = useRecordingLifecycle({
  maxDurationSeconds,
  perform: performRecordingCommand,
});
```

Delete `stopRecordingRef`, `beginRecordingRef`, timer cleanup, timestamp mutation, and timer controls. Dispatch `request-start` after the recorder handlers are ready. Dispatch `start-now`, `pause`, `resume`, and `stop` from the existing UI callbacks.

Derive render flags from the snapshot:

```ts
const { state, elapsed: duration, countdown: countdownValue, remaining } = recording.snapshot;
const isIdle = state === "idle";
const isCountdown = state === "countdown";
const isPaused = state === "paused";
const isActive = state !== "idle" && state !== "stopped";
const isRecording = state === "recording" || isPaused;
```

Keep `recording.elapsedSeconds()` in the existing `onstop` handler.

- [ ] **Step 3: Run camera tests and type checking after the migration**

```bash
cd web && pnpm test -- src/hooks/useRecordingLifecycle.test.ts src/components/CameraRecorder.test.tsx
cd web && pnpm typecheck
```

Expected: all tests pass with no type errors. Camera recorder `start` still receives `1000`; pause, resume, stop, completion, short-output rejection, countdown, maximum duration, MIME selection, and stream cleanup remain unchanged.

- [ ] **Step 4: Commit the camera adapter**

```bash
git add web/src/components/CameraRecorder.tsx web/src/components/CameraRecorder.test.tsx
git commit -m "refactor: migrate camera recording lifecycle"
```

### Task 4: Migrate the screen adapter and remove the shallow hook

**Files:**
- Modify: `web/src/components/Recorder.tsx:1-405`
- Modify: `web/src/components/Recorder.test.tsx`
- Delete: `web/src/hooks/useRecording.ts`

**Interfaces:**
- Consumes: the same lifecycle interface as the camera adapter.
- Produces: a screen adapter that maps commands to main and webcam recorders and cancels countdown through the lifecycle interface.

- [ ] **Step 1: Run the screen suite as the green refactor baseline**

```bash
cd web && pnpm test -- src/components/Recorder.test.tsx
```

Expected: PASS, including the two Task 1 characterization tests.

- [ ] **Step 2: Replace mutable lifecycle access with a screen command adapter**

Use the lifecycle and recording-limit imports from Task 3. Replace `beginRecording`, pause/resume media work, and the media portion of `stopRecording` with one callback:

```ts
const performRecordingCommand = useCallback((command: RecordingCommand) => {
  const recorder = mediaRecorderRef.current;

  if (command === "start") {
    recorder?.start();
    webcamRecorderRef.current?.start(1000);
    return;
  }

  if (command === "pause") {
    if (recorder?.state === "recording") recorder.pause();
    if (webcamRecorderRef.current?.state === "recording") webcamRecorderRef.current.pause();
    return;
  }

  if (command === "resume") {
    if (recorder?.state === "paused") recorder.resume();
    if (webcamRecorderRef.current?.state === "paused") webcamRecorderRef.current.resume();
    return;
  }

  const hasActiveRecorder = Boolean(recorder && recorder.state !== "inactive");
  if (hasActiveRecorder) recorder!.stop();
  if (webcamRecorderRef.current && webcamRecorderRef.current.state !== "inactive") {
    if (webcamRecorderRef.current.state === "paused") webcamRecorderRef.current.resume();
    webcamRecorderRef.current.stop();
  }
  stopCompositing();
  if (screenVideoRef.current) screenVideoRef.current.srcObject = null;
  if (!hasActiveRecorder) stopAllStreams();
}, [stopAllStreams, stopCompositing]);
```

Construct `useRecordingLifecycle` with this adapter. Dispatch `request-start` after media setup, and dispatch lifecycle events from the existing controls. Preserve the no-timeslice main recorder start and the `1000` webcam timeslice.

- [ ] **Step 3: Preserve the screen-track-ended branch with live media state**

Replace the old callback calls with:

```ts
screenStream.getVideoTracks()[0].addEventListener("ended", () => {
  if (mediaRecorderRef.current && mediaRecorderRef.current.state !== "inactive") {
    recording.dispatch({ type: "stop" });
    return;
  }

  recording.dispatch({ type: "cancel-countdown" });
  stopCompositing();
  if (screenVideoRef.current) screenVideoRef.current.srcObject = null;
  stopAllStreams();
  mediaRecorderRef.current = null;
  webcamRecorderRef.current = null;
  webcamBlobPromiseRef.current = null;
});
```

Do not branch on `recording.snapshot.state` inside this browser event handler.

- [ ] **Step 4: Remove the shallow hook and verify no leaked timing interface remains**

Delete `web/src/hooks/useRecording.ts`, then run:

```bash
rg -n "useRecording\b|countdownTimerRef|pauseStartRef|totalPausedRef|startTimeRef|startTimer|stopTimer|setCountdown|setState" web/src/components/Recorder.tsx web/src/components/CameraRecorder.tsx web/src/hooks
```

Expected: no old-hook import and no caller access to lifecycle timer refs, timestamps, state setters, or timer controls. Matches for ordinary React state setters outside the Recording lifecycle are acceptable only when they are unrelated to this list.

- [ ] **Step 5: Run focused tests and type checking**

```bash
cd web && pnpm test -- src/hooks/useRecordingLifecycle.test.ts src/components/Recorder.test.tsx src/components/CameraRecorder.test.tsx
cd web && pnpm typecheck
```

Expected: all focused tests pass and TypeScript reports no errors.

- [ ] **Step 6: Commit the screen adapter and shallow-hook removal**

```bash
git add web/src/components/Recorder.tsx web/src/components/Recorder.test.tsx web/src/hooks/useRecording.ts
git commit -m "refactor: deepen recording lifecycle"
```

### Task 5: Verify behavior preservation

**Files:**
- Verify only: all changed files from Tasks 1 through 4.

**Interfaces:**
- Consumes: the completed Recording lifecycle module and both media adapters.
- Produces: evidence that the refactor is ready for code review.

- [ ] **Step 1: Run the complete frontend test suite**

```bash
cd web && pnpm test
```

Expected: the full Vitest suite passes.

- [ ] **Step 2: Run coverage, type checking, and production build**

```bash
cd web && pnpm test:coverage
cd web && pnpm typecheck
cd web && pnpm build
```

Expected: all commands exit successfully. Coverage includes `useRecordingLifecycle.ts` through its interface tests.

- [ ] **Step 3: Review the final diff for behavior drift**

```bash
git diff 69a983c247bd30b848e1e7223b63bc1619395222..HEAD -- web/src/hooks web/src/components/Recorder.tsx web/src/components/CameraRecorder.tsx web/src/utils/recordingLimits.ts
git diff --check 69a983c247bd30b848e1e7223b63bc1619395222..HEAD
git status --short --branch
```

Confirm the diff changes only lifecycle ownership and tests. The UI text, media constraints, MIME choices, timeslices, validation thresholds, cleanup actions, and callback arguments must match the pre-refactor code.

- [ ] **Step 4: Request independent code review**

Use `superpowers:requesting-code-review` with base `69a983c247bd30b848e1e7223b63bc1619395222` and the final implementation HEAD. Fix Critical and Important findings before declaring this subproject complete.
