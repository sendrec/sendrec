import { renderHook, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

function mockMatchMedia(matches: boolean) {
  const listeners: Array<(e: { matches: boolean }) => void> = [];
  const mql = {
    matches,
    addEventListener: vi.fn(
      (_event: string, cb: (e: { matches: boolean }) => void) => {
        listeners.push(cb);
      },
    ),
    removeEventListener: vi.fn(
      (_event: string, cb: (e: { matches: boolean }) => void) => {
        const idx = listeners.indexOf(cb);
        if (idx >= 0) listeners.splice(idx, 1);
      },
    ),
  };
  window.matchMedia = vi.fn().mockReturnValue(mql);
  return {
    mql,
    trigger: (m: boolean) => {
      mql.matches = m;
      listeners.forEach((l) => l({ matches: m }));
    },
  };
}

describe("useTheme", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
    vi.resetModules();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  async function loadHook() {
    const mod = await import("./useTheme");
    return mod.useTheme;
  }

  it("defaults to system when no localStorage value", async () => {
    mockMatchMedia(true);
    const useTheme = await loadHook();
    const { result } = renderHook(() => useTheme());
    expect(result.current.theme).toBe("system");
  });

  it("resolves system to dark when OS prefers dark", async () => {
    mockMatchMedia(true);
    const useTheme = await loadHook();
    const { result } = renderHook(() => useTheme());
    expect(result.current.resolvedTheme).toBe("dark");
  });

  it("resolves system to light when OS prefers light", async () => {
    mockMatchMedia(false);
    const useTheme = await loadHook();
    const { result } = renderHook(() => useTheme());
    expect(result.current.resolvedTheme).toBe("light");
  });

  it("reads theme from localStorage", async () => {
    localStorage.setItem("theme", "light");
    mockMatchMedia(true);
    const useTheme = await loadHook();
    const { result } = renderHook(() => useTheme());
    expect(result.current.theme).toBe("light");
    expect(result.current.resolvedTheme).toBe("light");
  });

  it("sets data-theme attribute on documentElement", async () => {
    localStorage.setItem("theme", "dark");
    mockMatchMedia(false);
    const useTheme = await loadHook();
    renderHook(() => useTheme());
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
  });

  it("updates theme and persists to localStorage", async () => {
    mockMatchMedia(true);
    const useTheme = await loadHook();
    const { result } = renderHook(() => useTheme());
    act(() => result.current.setTheme("light"));
    expect(result.current.theme).toBe("light");
    expect(result.current.resolvedTheme).toBe("light");
    expect(localStorage.getItem("theme")).toBe("light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
  });

  it("responds to OS preference change when theme is system", async () => {
    const { trigger } = mockMatchMedia(true);
    const useTheme = await loadHook();
    const { result } = renderHook(() => useTheme());
    expect(result.current.resolvedTheme).toBe("dark");

    act(() => trigger(false));
    expect(result.current.resolvedTheme).toBe("light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
  });

  it("ignores OS preference change when theme is explicit", async () => {
    const { trigger } = mockMatchMedia(true);
    localStorage.setItem("theme", "dark");
    const useTheme = await loadHook();
    const { result } = renderHook(() => useTheme());

    act(() => trigger(false));
    expect(result.current.resolvedTheme).toBe("dark");
  });

  it("cleans up media query listener on unmount", async () => {
    const { mql } = mockMatchMedia(true);
    const useTheme = await loadHook();
    const { unmount } = renderHook(() => useTheme());
    unmount();
    expect(mql.removeEventListener).toHaveBeenCalledWith(
      "change",
      expect.any(Function),
    );
  });
});
