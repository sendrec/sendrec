import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { App } from "./App";
import { I18nProvider } from "./i18n/I18nContext";
import { LegacyI18nBridge } from "./i18n/LegacyI18nBridge";
import "./styles.css";


if (import.meta.env.PROD && "serviceWorker" in navigator) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js").catch((error) => {
      console.error("Service Worker registration failed:", error);
    });
  });
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <I18nProvider>
      <LegacyI18nBridge />
      <ErrorBoundary>
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </ErrorBoundary>
    </I18nProvider>
  </StrictMode>
);
