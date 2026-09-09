import { createContext, type ReactNode, useContext, useEffect, useMemo, useState } from "react";
import { SUPPORTED_UI_LANGUAGES, translations, type UiLanguage } from "./translations";

const STORAGE_KEY = "99tools-ui-language";

function detectLanguage(): UiLanguage {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === "de" || stored === "en") return stored;
  const browser = navigator.language.toLowerCase();
  return browser.startsWith("de") ? "de" : "en";
}

function interpolate(value: string, vars?: Record<string, string | number>): string {
  if (!vars) return value;
  return Object.entries(vars).reduce(
    (result, [key, replacement]) => result.replaceAll(`{${key}}`, String(replacement)),
    value,
  );
}

interface I18nValue {
  language: UiLanguage;
  languages: typeof SUPPORTED_UI_LANGUAGES;
  setLanguage: (language: UiLanguage) => void;
  t: (key: string, vars?: Record<string, string | number>) => string;
}

const I18nContext = createContext<I18nValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [language, setLanguageState] = useState<UiLanguage>(detectLanguage);

  function setLanguage(next: UiLanguage) {
    localStorage.setItem(STORAGE_KEY, next);
    setLanguageState(next);
  }

  useEffect(() => {
    document.documentElement.lang = language;
  }, [language]);

  const value = useMemo<I18nValue>(() => ({
    language,
    languages: SUPPORTED_UI_LANGUAGES,
    setLanguage,
    t: (key, vars) => interpolate(translations[language][key] ?? translations.de[key] ?? key, vars),
  }), [language]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nValue {
  const value = useContext(I18nContext);
  if (!value) throw new Error("useI18n must be used inside I18nProvider");
  return value;
}
