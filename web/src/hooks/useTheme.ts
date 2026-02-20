import { useCallback, useEffect, useSyncExternalStore } from "react";

type Theme = "dark" | "light" | "system";
type ResolvedTheme = "dark" | "light";

const STORAGE_KEY = "theme";
const MEDIA_QUERY = "(prefers-color-scheme: dark)";

function getStoredTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === "dark" || stored === "light" || stored === "system") {
    return stored;
  }
  return "system";
}

function getSystemPreference(): ResolvedTheme {
  if (typeof window.matchMedia === "function") {
    return window.matchMedia(MEDIA_QUERY).matches ? "dark" : "light";
  }
  return "dark";
}

function resolveTheme(theme: Theme): ResolvedTheme {
  if (theme === "system") return getSystemPreference();
  return theme;
}

function applyTheme(resolved: ResolvedTheme) {
  document.documentElement.setAttribute("data-theme", resolved);
}

let currentTheme: Theme = getStoredTheme();
let currentResolved: ResolvedTheme = resolveTheme(currentTheme);
const listeners = new Set<() => void>();

function subscribe(callback: () => void) {
  listeners.add(callback);
  return () => {
    listeners.delete(callback);
  };
}

function notifyListeners() {
  listeners.forEach((cb) => cb());
}

let snapshotRef = { theme: currentTheme, resolvedTheme: currentResolved };

function getSnapshot() {
  return snapshotRef;
}

function updateSnapshot() {
  snapshotRef = { theme: currentTheme, resolvedTheme: currentResolved };
}

export function useTheme() {
  const snapshot = useSyncExternalStore(subscribe, getSnapshot, getSnapshot);

  useEffect(() => {
    applyTheme(snapshot.resolvedTheme);
  }, [snapshot.resolvedTheme]);

  useEffect(() => {
    const mql = window.matchMedia(MEDIA_QUERY);
    function handleChange() {
      if (currentTheme === "system") {
        currentResolved = getSystemPreference();
        applyTheme(currentResolved);
        updateSnapshot();
        notifyListeners();
      }
    }
    mql.addEventListener("change", handleChange);
    return () => mql.removeEventListener("change", handleChange);
  }, []);

  const setTheme = useCallback((newTheme: Theme) => {
    currentTheme = newTheme;
    currentResolved = resolveTheme(newTheme);
    localStorage.setItem(STORAGE_KEY, newTheme);
    applyTheme(currentResolved);
    updateSnapshot();
    notifyListeners();
  }, []);

  return {
    theme: snapshot.theme,
    resolvedTheme: snapshot.resolvedTheme,
    setTheme,
  };
}
