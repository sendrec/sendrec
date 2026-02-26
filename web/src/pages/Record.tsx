import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
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

export function Record() {
  const [tab, setTab] = useState<"record" | "upload">("record");
  const [uploading, setUploading] = useState(false);
  const [uploadStep, setUploadStep] = useState("");
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
      const uploadResp = await fetch(result.uploadUrl, {
        method: "PUT",
        body: blob,
        headers: { "Content-Type": contentType },
      });

      if (!uploadResp.ok) {
        throw new Error("Upload failed");
      }

      if (webcamBlob && result.webcamUploadUrl) {
        setUploadStep("Uploading camera...");
        const webcamResp = await fetch(result.webcamUploadUrl, {
          method: "PUT",
          body: webcamBlob,
          headers: { "Content-Type": webcamBlob.type || "video/webm" },
        });

        if (!webcamResp.ok) {
          throw new Error("Webcam upload failed");
        }
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
          <h2 className="share-heading">Recording complete</h2>
          <div className="share-link-row">
            <span className="share-link-input">{shareUrl}</span>
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
      {limits && limits.videosUsedThisMonth === 0 && (
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
        screenRecordingSupported ? (
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
