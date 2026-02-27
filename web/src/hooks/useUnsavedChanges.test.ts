import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useUnsavedChanges } from "./useUnsavedChanges";

describe("useUnsavedChanges", () => {
  const addSpy = vi.spyOn(window, "addEventListener");
  const removeSpy = vi.spyOn(window, "removeEventListener");

  beforeEach(() => {
    addSpy.mockClear();
    removeSpy.mockClear();
  });

  afterEach(() => {
    addSpy.mockClear();
    removeSpy.mockClear();
  });

  it("adds beforeunload listener when dirty", () => {
    renderHook(() => useUnsavedChanges(true));
    expect(addSpy).toHaveBeenCalledWith("beforeunload", expect.any(Function));
  });

  it("does not add listener when not dirty", () => {
    renderHook(() => useUnsavedChanges(false));
    const beforeUnloadCalls = addSpy.mock.calls.filter(
      (call) => call[0] === "beforeunload",
    );
    expect(beforeUnloadCalls).toHaveLength(0);
  });

  it("removes listener on unmount", () => {
    const { unmount } = renderHook(() => useUnsavedChanges(true));
    unmount();
    expect(removeSpy).toHaveBeenCalledWith(
      "beforeunload",
      expect.any(Function),
    );
  });

  it("removes listener when isDirty changes to false", () => {
    const { rerender } = renderHook(
      ({ dirty }) => useUnsavedChanges(dirty),
      { initialProps: { dirty: true } },
    );
    rerender({ dirty: false });
    expect(removeSpy).toHaveBeenCalledWith(
      "beforeunload",
      expect.any(Function),
    );
  });
});
