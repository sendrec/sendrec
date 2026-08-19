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

    act(() => { vi.advanceTimersByTime(1000); });
    expect(result.current.snapshot.countdown).toBe(2);
    act(() => { vi.advanceTimersByTime(1000); });
    expect(result.current.snapshot.countdown).toBe(1);
    act(() => { vi.advanceTimersByTime(1000); });

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
    act(() => { vi.advanceTimersByTime(3000); });

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
    act(() => { vi.advanceTimersByTime(2500); });
    act(() => result.current.dispatch({ type: "pause" }));
    act(() => { vi.advanceTimersByTime(4000); });
    act(() => result.current.dispatch({ type: "resume" }));
    act(() => { vi.advanceTimersByTime(1500); });
    act(() => result.current.dispatch({ type: "pause" }));
    act(() => { vi.advanceTimersByTime(3000); });
    act(() => result.current.dispatch({ type: "stop" }));

    expect(result.current.elapsedSeconds()).toBe(4);
    expect(perform.mock.calls.map(([command]) => command)).toEqual([
      "start",
      "pause",
      "resume",
      "pause",
      "stop",
    ]);
    expect(result.current.snapshot.state).toBe("stopped");
  });

  it("does not reach the maximum duration while paused", () => {
    const perform = vi.fn();
    const { result } = renderHook(() =>
      useRecordingLifecycle({ maxDurationSeconds: 2, perform }),
    );
    act(() => result.current.dispatch({ type: "request-start", countdown: false }));
    act(() => { vi.advanceTimersByTime(1000); });
    act(() => result.current.dispatch({ type: "pause" }));
    act(() => { vi.advanceTimersByTime(10_000); });
    expect(perform).not.toHaveBeenCalledWith("stop");

    act(() => result.current.dispatch({ type: "resume" }));
    act(() => { vi.advanceTimersByTime(1000); });
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
    act(() => { vi.advanceTimersByTime(5000); });

    expect(perform).not.toHaveBeenCalled();
    expect(result.current.snapshot).toMatchObject({
      state: "idle",
      elapsed: 0,
      countdown: 3,
    });
  });

  it("uses the latest adapter after rerender without restarting countdown", () => {
    const first = vi.fn();
    const second = vi.fn();
    const { result, rerender } = renderHook(
      ({ perform }) => useRecordingLifecycle({ maxDurationSeconds: 0, perform }),
      { initialProps: { perform: first } },
    );
    act(() => result.current.dispatch({ type: "request-start", countdown: true }));
    act(() => { vi.advanceTimersByTime(1000); });
    rerender({ perform: second });
    act(() => { vi.advanceTimersByTime(2000); });

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
    act(() => { vi.advanceTimersByTime(5000); });
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
    act(() => { vi.advanceTimersByTime(1000); });

    expect(perform.mock.calls.map(([command]) => command)).toEqual(["start", "stop"]);
    expect(result.current.snapshot.state).toBe("stopped");
  });

  it("stops immediately when the maximum is lowered below elapsed time", () => {
    const perform = vi.fn();
    const { result, rerender } = renderHook(
      ({ maximum }) => useRecordingLifecycle({
        maxDurationSeconds: maximum,
        perform,
      }),
      { initialProps: { maximum: 10 } },
    );
    act(() => result.current.dispatch({ type: "request-start", countdown: false }));
    act(() => { vi.advanceTimersByTime(3000); });

    rerender({ maximum: 2 });

    expect(perform.mock.calls.map(([command]) => command)).toEqual(["start", "stop"]);
    expect(result.current.snapshot.state).toBe("stopped");
  });

  it("keeps recording when the adapter rejects pause", () => {
    const perform = vi.fn((command: string) => command !== "pause");
    const { result } = renderHook(() =>
      useRecordingLifecycle({ maxDurationSeconds: 0, perform }),
    );
    act(() => result.current.dispatch({ type: "request-start", countdown: false }));

    act(() => result.current.dispatch({ type: "pause" }));

    expect(result.current.snapshot.state).toBe("recording");
  });

  it("keeps paused when the adapter rejects resume", () => {
    let rejectResume = false;
    const perform = vi.fn((command: string) => command !== "resume" || !rejectResume);
    const { result } = renderHook(() =>
      useRecordingLifecycle({ maxDurationSeconds: 0, perform }),
    );
    act(() => result.current.dispatch({ type: "request-start", countdown: false }));
    act(() => result.current.dispatch({ type: "pause" }));
    rejectResume = true;

    act(() => result.current.dispatch({ type: "resume" }));

    expect(result.current.snapshot.state).toBe("paused");
  });
});
