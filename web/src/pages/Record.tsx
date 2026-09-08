import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { apiFetch } from "../api/client";
import { CameraRecorder } from "../components/CameraRecorder";
import { Recorder } from "../components/Recorder";
import { LimitsResponse } from "../types/limits";
import { Upload } from "./Upload";
import { useI18n } from "../i18n/I18nContext";

interface CreateVideoResponse {
  id: string;
  uploadUrl: string;
  shareToken: string;
  webcamUploadUrl?: string;
}

function uploadWithProgress(
  url: string,
  blob: Blob,
  contentType: string,
  onProgress: (pct: number) => void
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", url);
    xhr.setRequestHeader("Content-Type", contentType);
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        onProgress(Math.round((e.loaded / e.total) * 100));
      }
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve();
      else reject(new Error("Upload fehlgeschlagen"));
    };
    xhr.onerror = () => reject(new Error("Upload fehlgeschlagen"));
    xhr.send(blob);
  });
}

export function Record() {
  const { t } = useI18n();
  const [searchParams] = useSearchParams();
  const [tab, setTab] = useState<"record" | "upload">(() =>
    searchParams.get("tab") === "upload" ? "upload" : "record"
  );
  const [uploading, setUploading] = useState(false);
  const [uploadStep, setUploadStep] = useState("");
  const [uploadPercent, setUploadPercent] = useState(0);
  const [shareUrl, setShareUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [limits, setLimits] = useState<LimitsResponse | null>(null);
  const [loadingLimits, setLoadingLimits] = useState(true);

  useEffect(() => {
    async function fetchLimits() {
      try {
        const result = await apiFetch<LimitsResponse>("/api/videos/limits");
        setLimits(result ?? null);
      } catch {
        setLimits(null);
      } finally {
        setLoadingLimits(false);
      }
    }
    fetchLimits();
  }, []);

  async function handleRecordingComplete(blob: Blob, duration: number, webcamBlob?: Blob, cameraPosition = "bottom-right") {
    setUploading(true);
    setError(null);
    let videoId: string | null = null;

    try {
      setUploadStep("Video wird erstellt...");
      const now = new Date();
      const title = `Aufnahme ${now.toLocaleDateString("de-DE")} ${now.toLocaleTimeString("de-DE")}`;

      const contentType = blob.type || "video/webm";
      const createBody: Record<string, unknown> = { title, duration, fileSize: blob.size, contentType };
      if (webcamBlob) {
        createBody.webcamFileSize = webcamBlob.size;
        createBody.webcamContentType = webcamBlob.type || "video/webm";
      }

      const result = await apiFetch<CreateVideoResponse>("/api/videos", {
        method: "POST",
        body: JSON.stringify(createBody),
      });

      if (!result) {
        throw new Error("Video konnte nicht erstellt werden");
      }

      videoId = result.id;

      setUploadStep("Aufnahme wird hochgeladen...");
      setUploadPercent(0);
      await uploadWithProgress(result.uploadUrl, blob, contentType, setUploadPercent);

      if (webcamBlob && result.webcamUploadUrl) {
        setUploadStep("Kamera-Aufnahme wird hochgeladen...");
        setUploadPercent(0);
        await uploadWithProgress(result.webcamUploadUrl, webcamBlob, webcamBlob.type || "video/webm", setUploadPercent);
      }

      setUploadStep("Wird fertiggestellt...");
      await apiFetch(`/api/videos/${result.id}`, {
        method: "PATCH",
        body: JSON.stringify({ status: "ready", cameraPosition }),
      });

      setShareUrl(`${window.location.origin}/watch/${result.shareToken}`);
    } catch (err) {
      if (videoId) {
        apiFetch(`/api/videos/${videoId}`, { method: "DELETE" }).catch(() => {});
      }
      setError(err instanceof Error ? err.message : "Upload fehlgeschlagen");
    } finally {
      setUploading(false);
    }
  }

  function handleRecordingError(message: string) {
    setError(message);
  }

  useEffect(() => {
    if (!uploading) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [uploading]);

  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (shareUrl) {
      navigator.clipboard.writeText(shareUrl).catch(() => {});
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }, [shareUrl]);

  async function copyShareUrl() {
    if (!shareUrl) return;
    try {
      await navigator.clipboard.writeText(shareUrl);
    } catch {
      const textArea = document.createElement("textarea");
      textArea.value = shareUrl;
      textArea.style.position = "fixed";
      textArea.style.opacity = "0";
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand("copy");
      document.body.removeChild(textArea);
    }
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  function recordAnother() {
    setShareUrl(null);
    setError(null);
  }

  if (loadingLimits) {
    return (
      <div className="page-container page-container--centered">
        <p className="max-duration-label">Wird geladen...</p>
      </div>
    );
  }

  if (uploading) {
    return (
      <div className="page-container page-container--centered">
        <div style={{ textAlign: "center" }}>
          <p className="max-duration-label" style={{ marginBottom: 8 }}>{uploadStep || "Wird hochgeladen..."}</p>
          {uploadStep.includes("hochgeladen") && (
            <>
              <div className="upload-progress-bar">
                <div className="upload-progress-fill" style={{ width: `${uploadPercent}%` }} />
              </div>
              <p className="upload-progress-percent">{uploadPercent}%</p>
            </>
          )}
          <p className="max-duration-label" style={{ opacity: 0.7 }}>Bitte diese Seite nicht schließen</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="page-container page-container--centered">
        <p className="error-message">{error}</p>
        <button className="btn-record" onClick={recordAnother}>Erneut versuchen</button>
      </div>
    );
  }

  const screenRecordingSupported =
    typeof navigator.mediaDevices?.getDisplayMedia === "function";
  const cameraSupported =
    typeof navigator.mediaDevices?.getUserMedia === "function";
  const preferredMode = localStorage.getItem("recording-mode") || "screen";
  const useCameraOnly = preferredMode === "camera" && cameraSupported;

  const quotaReached =
    limits !== null &&
    limits.maxVideosPerMonth > 0 &&
    limits.videosUsedThisMonth >= limits.maxVideosPerMonth;

  if (!screenRecordingSupported && !cameraSupported) {
    return (
      <div className="page-container page-container--centered">
        <h1 className="page-heading">Aufnahme ist nicht verfügbar</h1>
        <p className="quota-submessage">
          Auf diesem Gerät wird die Aufnahme nicht unterstützt. Bitte verwende einen aktuellen Browser oder lade stattdessen{" "}
          <button onClick={() => setTab("upload")} style={{ color: "var(--color-accent)", background: "none", border: "none", cursor: "pointer", font: "inherit", textDecoration: "underline", padding: 0 }}>ein Video hoch</button>.
        </p>
        <button className="btn-record" onClick={() => setTab("upload")}>Zum Upload</button>
      </div>
    );
  }

  if (quotaReached) {
    return (
      <div className="page-container page-container--centered">
        <div className="usage-bar" style={{ maxWidth: 300, margin: "0 auto 16px" }}>
          <div className="usage-bar-fill usage-bar-fill--warning" style={{ width: "100%" }} />
        </div>
        <p className="quota-message">
          Du hast dein Limit von {limits!.maxVideosPerMonth} Videos für diesen Monat erreicht.
        </p>
        <p className="quota-submessage">Lösche nicht benötigte Aufnahmen oder warte bis zum nächsten Monat.</p>
        <div className="quota-actions">
          <Link to="/library" className="btn-primary">Zur Bibliothek</Link>
          <Link to="/settings" className="btn-outline">Auf Pro upgraden</Link>
        </div>
      </div>
    );
  }

  if (shareUrl) {
    return (
      <div className="page-container page-container--centered">
        <div className="share-container">
          <div className="share-checkmark">
            <svg viewBox="0 0 48 48" fill="none">
              <circle cx="24" cy="24" r="24" fill="rgba(230, 70, 122, 0.12)" />
              <path d="M15 25l6 6 12-12" stroke="#E6467A" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </div>
          <h2 className="share-heading">Dein Video ist fertig!</h2>
          <div className="share-link-row">
            <input
              type="text"
              readOnly
              className="share-link-input"
              value={shareUrl}
              onClick={(e) => (e.target as HTMLInputElement).select()}
            />
            <button className="btn-copy" onClick={copyShareUrl}>
              {copied ? "Kopiert!" : "Link kopieren"}
            </button>
          </div>
          <div className="share-actions">
            <a href={shareUrl} target="_blank" rel="noopener noreferrer" className="btn-primary">
              Video ansehen
            </a>
            <button className="btn-outline" onClick={recordAnother}>Weitere Aufnahme</button>
            <Link to="/library" className="btn-ghost">Zur Bibliothek</Link>
          </div>
        </div>
      </div>
    );
  }

  const hasLimits = limits !== null && limits.maxVideosPerMonth > 0;
  const usagePercent = hasLimits
    ? (limits.videosUsedThisMonth / limits.maxVideosPerMonth) * 100
    : 0;

  return (
    <div className="page-container page-container--centered">
      <h1 className="page-heading">{t("record.new")}</h1>
      <div className="record-tabs">
        <button
          className={`record-tab${tab === "record" ? " record-tab--active" : ""}`}
          onClick={() => setTab("record")}
        >
          {t("nav.record")}
        </button>
        <button
          className={`record-tab${tab === "upload" ? " record-tab--active" : ""}`}
          onClick={() => setTab("upload")}
        >
          {t("record.upload")}
        </button>
      </div>
      {hasLimits && (
        <div className="usage-section">
          <p className="usage-label">
            {t("record.monthUsage", { used: limits.videosUsedThisMonth, max: limits.maxVideosPerMonth })}
          </p>
          <div
            className="usage-bar"
            role="progressbar"
            aria-valuenow={limits.videosUsedThisMonth}
            aria-valuemin={0}
            aria-valuemax={limits.maxVideosPerMonth}
          >
            <div
              className={`usage-bar-fill${usagePercent >= 80 ? " usage-bar-fill--warning" : ""}`}
              style={{ width: `${Math.min(usagePercent, 100)}%` }}
            />
          </div>
        </div>
      )}
      {tab === "record" && limits && limits.videosUsedThisMonth === 0 && (
        <div className="onboarding-card">
          <p className="onboarding-title">{t("record.getStarted")}</p>
          <div className="onboarding-steps">
            <div className="onboarding-step">
              <span className="onboarding-step-num">1.</span>
              <span className="onboarding-step-text">{t("record.step1")}</span>
            </div>
            <div className="onboarding-step">
              <span className="onboarding-step-num">2.</span>
              <span className="onboarding-step-text">{t("record.step2")}</span>
            </div>
            <div className="onboarding-step">
              <span className="onboarding-step-num">3.</span>
              <span className="onboarding-step-text">{t("record.step3")}</span>
            </div>
          </div>
        </div>
      )}
      {tab === "record" ? (
        screenRecordingSupported && !useCameraOnly ? (
          <Recorder
            onRecordingComplete={handleRecordingComplete}
            onRecordingError={handleRecordingError}
            maxDurationSeconds={limits?.maxVideoDurationSeconds ?? 0}
          />
        ) : (
          <CameraRecorder
            onRecordingComplete={handleRecordingComplete}
            onRecordingError={handleRecordingError}
            maxDurationSeconds={limits?.maxVideoDurationSeconds ?? 0}
          />
        )
      ) : (
        <Upload />
      )}
    </div>
  );
}
