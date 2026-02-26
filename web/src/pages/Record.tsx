import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { apiFetch } from "../api/client";
import { CameraRecorder } from "../components/CameraRecorder";
import { Recorder } from "../components/Recorder";
import { Upload } from "./Upload";

interface CreateVideoResponse {
  id: string;
  uploadUrl: string;
  shareToken: string;
  webcamUploadUrl?: string;
}

interface LimitsResponse {
  maxVideosPerMonth: number;
  maxVideoDurationSeconds: number;
  videosUsedThisMonth: number;
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
      else reject(new Error("Upload failed"));
    };
    xhr.onerror = () => reject(new Error("Upload failed"));
    xhr.send(blob);
  });
}

export function Record() {
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

  async function handleRecordingComplete(blob: Blob, duration: number, webcamBlob?: Blob) {
    setUploading(true);
    setError(null);
    let videoId: string | null = null;

    try {
      setUploadStep("Creating video...");
      const now = new Date();
      const title = `Recording ${now.toLocaleDateString("en-GB")} ${now.toLocaleTimeString("en-GB")}`;

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
        throw new Error("Failed to create video");
      }

      videoId = result.id;

      setUploadStep("Uploading recording...");
      setUploadPercent(0);
      await uploadWithProgress(result.uploadUrl, blob, contentType, setUploadPercent);

      if (webcamBlob && result.webcamUploadUrl) {
        setUploadStep("Uploading camera...");
        setUploadPercent(0);
        await uploadWithProgress(result.webcamUploadUrl, webcamBlob, webcamBlob.type || "video/webm", setUploadPercent);
      }

      setUploadStep("Finalizing...");
      await apiFetch(`/api/videos/${result.id}`, {
        method: "PATCH",
        body: JSON.stringify({ status: "ready" }),
      });

      setShareUrl(`${window.location.origin}/watch/${result.shareToken}`);
    } catch (err) {
      if (videoId) {
        apiFetch(`/api/videos/${videoId}`, { method: "DELETE" }).catch(() => {});
      }
      setError(err instanceof Error ? err.message : "Upload failed");
    } finally {
      setUploading(false);
    }
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
        <p className="max-duration-label">Loading...</p>
      </div>
    );
  }

  if (uploading) {
    return (
      <div className="page-container page-container--centered">
        <div style={{ textAlign: "center" }}>
          <p className="max-duration-label" style={{ marginBottom: 8 }}>{uploadStep || "Uploading..."}</p>
          {uploadStep.includes("Uploading") && (
            <>
              <div className="upload-progress-bar">
                <div className="upload-progress-fill" style={{ width: `${uploadPercent}%` }} />
              </div>
              <p className="upload-progress-percent">{uploadPercent}%</p>
            </>
          )}
          <p className="max-duration-label" style={{ opacity: 0.7 }}>Please don't close this page</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="page-container page-container--centered">
        <p className="error-message">{error}</p>
        <button className="btn-record" onClick={recordAnother}>Try again</button>
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
        <h1 className="page-heading">Recording is not available</h1>
        <p className="quota-submessage">
          Recording is not supported on this device. Please use a modern browser, or{" "}
          <button onClick={() => setTab("upload")} style={{ color: "var(--color-accent)", background: "none", border: "none", cursor: "pointer", font: "inherit", textDecoration: "underline", padding: 0 }}>upload a video</button> instead.
        </p>
        <button className="btn-record" onClick={() => setTab("upload")}>Go to Upload</button>
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
          You've reached your limit of {limits!.maxVideosPerMonth} videos this month.
        </p>
        <p className="quota-submessage">Delete unused recordings or wait until next month.</p>
        <div className="quota-actions">
          <Link to="/library" className="btn-primary">Go to Library</Link>
          <Link to="/settings" className="btn-outline">Upgrade to Pro</Link>
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
              <circle cx="24" cy="24" r="24" fill="rgba(0, 182, 122, 0.12)" />
              <path d="M15 25l6 6 12-12" stroke="#00b67a" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </div>
          <h2 className="share-heading">Your video is ready!</h2>
          <div className="share-link-row">
            <input
              type="text"
              readOnly
              className="share-link-input"
              value={shareUrl}
              onClick={(e) => (e.target as HTMLInputElement).select()}
            />
            <button className="btn-copy" onClick={copyShareUrl}>
              {copied ? "Copied!" : "Copy link"}
            </button>
          </div>
          <div className="share-actions">
            <a href={shareUrl} target="_blank" rel="noopener noreferrer" className="btn-primary">
              Watch video
            </a>
            <button className="btn-outline" onClick={recordAnother}>Record another</button>
            <Link to="/library" className="btn-ghost">Go to Library</Link>
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
      <h1 className="page-heading">New Recording</h1>
      <div className="record-tabs">
        <button
          className={`record-tab${tab === "record" ? " record-tab--active" : ""}`}
          onClick={() => setTab("record")}
        >
          Record
        </button>
        <button
          className={`record-tab${tab === "upload" ? " record-tab--active" : ""}`}
          onClick={() => setTab("upload")}
        >
          Upload
        </button>
      </div>
      {hasLimits && (
        <div className="usage-section">
          <p className="usage-label">
            {limits.videosUsedThisMonth} / {limits.maxVideosPerMonth} videos this month
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
          <p className="onboarding-title">Get started in 3 steps</p>
          <div className="onboarding-steps">
            <div className="onboarding-step">
              <span className="onboarding-step-num">1.</span>
              <span className="onboarding-step-text">Record your screen or upload a video</span>
            </div>
            <div className="onboarding-step">
              <span className="onboarding-step-num">2.</span>
              <span className="onboarding-step-text">Share the link with anyone</span>
            </div>
            <div className="onboarding-step">
              <span className="onboarding-step-num">3.</span>
              <span className="onboarding-step-text">Track views and get feedback</span>
            </div>
          </div>
        </div>
      )}
      {tab === "record" ? (
        screenRecordingSupported && !useCameraOnly ? (
          <Recorder
            onRecordingComplete={handleRecordingComplete}
            maxDurationSeconds={limits?.maxVideoDurationSeconds ?? 0}
          />
        ) : (
          <CameraRecorder
            onRecordingComplete={handleRecordingComplete}
            maxDurationSeconds={limits?.maxVideoDurationSeconds ?? 0}
          />
        )
      ) : (
        <Upload />
      )}
    </div>
  );
}
