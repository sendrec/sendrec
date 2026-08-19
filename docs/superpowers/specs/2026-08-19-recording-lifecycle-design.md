# Recording lifecycle design

## Goal

Deepen the shared Recording lifecycle module without changing user-visible behavior. Screen recording and camera recording must keep the same controls, countdown, elapsed time, pause accounting, maximum-duration stop, output validation, errors, and completion callbacks.

This is the first of five architecture improvements from the 2026-08-19 review. The other four remain separate subprojects so each can have its own design, tests, and passing commit.

## Current problem

`useRecording` exposes a shallow interface with mutable timing refs, timer controls, state setters, and derived flags. `Recorder` and `CameraRecorder` both coordinate the same transition order:

- reset pause accounting before a new Recording
- optionally count down from three
- start the media adapter and elapsed timer
- stop the timer while paused
- exclude paused time after resume or stop
- stop at the configured maximum duration
- clear timers during reset and unmount

The callers need to understand the implementation to use it correctly. Tests repeat the same lifecycle cases through both callers.

## Chosen design

Replace `useRecording` with a deeper `useRecordingLifecycle` module. Its interface has three parts:

```ts
type RecordingCommand = "start" | "pause" | "resume" | "stop";

type RecordingEvent =
  | { type: "request-start"; countdown: boolean }
  | { type: "start-now" }
  | { type: "pause" }
  | { type: "resume" }
  | { type: "stop" }
  | { type: "cancel-countdown" };

interface RecordingLifecycle {
  snapshot: {
    state: "idle" | "countdown" | "recording" | "paused" | "stopped";
    elapsed: number;
    countdown: number;
    remaining: number | null;
  };
  dispatch(event: RecordingEvent): void;
  elapsedSeconds(): number;
}

function useRecordingLifecycle(options: {
  maxDurationSeconds: number;
  perform(command: RecordingCommand): void;
}): RecordingLifecycle;
```

Each caller supplies a `perform(command)` adapter. The screen adapter operates its screen, audio, compositing, and optional webcam recorders. The camera adapter operates its camera recorder. These are two real adapters at one seam.

The public interface does not expose timer handles, timestamps, pause totals, state setters, timer methods, or redundant boolean flags. Callers derive display flags from `snapshot.state` when rendering.

`dispatch` and `elapsedSeconds` remain stable across renders. The implementation stores the latest `perform` adapter and maximum duration in refs, and keeps a synchronous phase ref beside rendered state. Timers use the latest values without restarting, and two events fired before React renders cannot issue the same command twice.

## Responsibilities

The Recording lifecycle implementation owns:

- legal phase transitions
- the three-second countdown and click-to-skip behavior
- elapsed-time updates at the current one-second interval
- start time, pause start time, and accumulated paused time
- maximum-duration detection and stop dispatch
- timer cleanup during countdown cancellation and unmount
- command ordering relative to lifecycle state updates

The screen and camera callers continue to own:

- media permission requests and previews
- `MediaRecorder` construction and MIME fallback
- media chunks and Blob construction
- compositing, drawing, screen audio, and webcam details
- minimum duration and Blob size validation
- the existing error text and completion callbacks
- stopping media tracks when their caller unmounts

Validation remains in the callers because it depends on the media output delivered after `MediaRecorder.stop()`. Moving it would enlarge this refactor without improving the lifecycle interface.

## Data flow

1. A caller prepares its media streams and recorder, then dispatches `request-start` with the stored countdown preference.
2. The lifecycle either enters `countdown` or asks the adapter to perform `start` and enters `recording`.
3. The countdown timer asks the adapter to perform `start` after the same three ticks. `start-now` follows the same path immediately.
4. Pause, resume, and stop events ask the adapter to perform the matching command while the lifecycle follows the transition order below.
5. The adapter's existing `onstop` handler builds and validates the Blob, obtains the final duration through `elapsedSeconds()`, and invokes the existing caller callback.
6. Countdown cancellation or unmount clears lifecycle timers. The caller separately cleans up its media resources as it does now.

## Transition order

The implementation preserves these exact synchronous sequences:

| Event | Current phase | Ordered work |
| --- | --- | --- |
| `request-start` with countdown | `idle` | clear old timers, reset elapsed and pause accounting, set countdown to three, enter `countdown`, start the countdown timer |
| `request-start` without countdown | `idle` | clear old timers, reset elapsed and pause accounting, perform `start`, set the start timestamp, enter `recording`, start the elapsed timer |
| countdown completion or `start-now` | `countdown` | clear the countdown timer, perform `start`, set the start timestamp, enter `recording`, start the elapsed timer |
| `pause` | `recording` | perform `pause`, set the pause timestamp, clear the elapsed timer, enter `paused` |
| `resume` | `paused` | add the paused interval to accumulated paused time, perform `resume`, start the elapsed timer, enter `recording` |
| `stop` | `recording` | perform `stop`, clear the elapsed timer, enter `stopped` |
| `stop` | `paused` | add the final paused interval, perform `stop`, clear the elapsed timer, enter `stopped` |
| `cancel-countdown` | `countdown` | clear the countdown timer, reset elapsed and pause accounting, enter `idle` without performing a command |

If `perform` throws, the exception propagates before the later lifecycle mutations in that row. This matches the current command-first behavior.

Screen compositing still starts while media is prepared, before `request-start`. The screen adapter's `start` command starts only the already prepared main and webcam recorders.

## Behavior contract

The refactor must preserve these observations:

- countdown starts at three and can be skipped by clicking the overlay
- elapsed duration uses whole seconds and excludes all paused time
- stopping while paused excludes the final paused interval
- a positive maximum duration stops a Recording when elapsed time reaches it
- zero maximum duration means unlimited recording
- remaining time stays `null` for unlimited recording
- recordings under one second or 1024 bytes use the existing error
- the existing screen and camera controls appear in the same phases
- screen recording keeps its optional webcam output and MP4 fallback behavior
- all current cleanup and completion behavior remains intact

There are no intended route, persistence, network, UI, copy, or accessibility changes.

## Invalid transitions and errors

Events that do not apply to the current phase are ignored. `request-start` applies only in `idle`, `start-now` and `cancel-countdown` only in `countdown`, `pause` only in `recording`, `resume` only in `paused`, and `stop` only in `recording` or `paused`. This matches the current guarded controls and avoids creating new user-visible failures.

If screen sharing ends during countdown, the caller checks the live `MediaRecorder.state`, dispatches `cancel-countdown`, and performs its existing compositing, preview, stream, and recorder cleanup. Once the media recorder is active, the same handler dispatches `stop` instead. It does not branch on a captured React snapshot.

The lifecycle does not translate adapter failures. Existing caller behavior remains responsible for media preparation errors and completion errors. No new retries, messages, or recovery states are introduced.

## Testing

Implementation follows TDD.

1. Add lifecycle tests with fake timers and a recording adapter spy. The first test must fail against the current shallow interface.
2. Cover countdown completion and skipping, countdown cancellation, start, pause, resume, multiple pause intervals, exact final duration when stopped while paused, maximum duration while running, maximum duration not advancing while paused, unlimited duration, invalid transitions, unmount cleanup, and adapter call order through the new interface.
3. Add a rerender test proving timers use the newest adapter without restarting and duplicate events before a render issue at most one command.
4. Adapt the existing `Recorder` and `CameraRecorder` tests to the new interface. Keep tests for media-specific behavior in those files. Add caller regression tests for screen sharing ending during countdown and runtime MP4-to-WebM encoder fallback.
5. Remove lifecycle assertions duplicated by both caller suites once the new interface tests cover them. Keep user-visible integration assertions in at least one caller test when they verify rendered controls.
6. Run the focused lifecycle and caller tests, the full frontend suite, TypeScript type checking, and the frontend build.

## Scope limits

This subproject will not:

- change media formats, quality, capture constraints, or browser support
- move media output validation into the lifecycle module
- add a state-machine dependency
- introduce classes, factories, or extra seams
- redesign the recording UI
- implement the other four architecture recommendations

## Completion criteria

- both callers use `useRecordingLifecycle`
- no caller reads or mutates lifecycle timer refs or timestamps
- lifecycle behavior is tested through its interface
- existing screen and camera behavior tests pass
- the full frontend suite, type check, and build pass
- a review of the diff finds no intended user-visible behavior change
