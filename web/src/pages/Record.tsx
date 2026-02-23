import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { apiFetch } from "../api/client";
import { CameraRecorder } from "../components/CameraRecorder";
import { Recorder } from "../components/Recorder";

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
  const [uploading, setUploading] = useState(false);
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
      const now = new Date();
      const title = `Recording ${now.toLocaleDateString()} ${now.toLocaleTimeString()}`;

      const contentType = blob.type || "video/webm";
      const createBody: Record<string, unknown> = { title, duration, fileSize: blob.size, contentType };
      if (webcamBlob) {
        createBody.webcamFileSize = webcamBlob.size;
      }

      const result = await apiFetch<CreateVideoResponse>("/api/videos", {
        method: "POST",
        body: JSON.stringify(createBody),
      });

      if (!result) {
        throw new Error("Failed to create video");
      }

      videoId = result.id;

      const uploadResp = await fetch(result.uploadUrl, {
        method: "PUT",
        body: blob,
        headers: { "Content-Type": contentType },
      });

      if (!uploadResp.ok) {
        throw new Error("Upload failed");
      }

      if (webcamBlob && result.webcamUploadUrl) {
        const webcamResp = await fetch(result.webcamUploadUrl, {
          method: "PUT",
          body: webcamBlob,
          headers: { "Content-Type": "video/webm" },
        });

        if (!webcamResp.ok) {
          throw new Error("Webcam upload failed");
        }
      }

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
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16 }}>Loading...</p>
      </div>
    );
  }

  if (uploading) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16 }}>Uploading...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-error)", fontSize: 16, marginBottom: 16 }}>{error}</p>
        <button
          onClick={recordAnother}
          style={{
            background: "var(--color-accent)",
            color: "var(--color-text)",
            borderRadius: 8,
            padding: "10px 24px",
            fontSize: 14,
            fontWeight: 600,
          }}
        >
          Try again
        </button>
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
        <h1
          style={{
            color: "var(--color-text)",
            fontSize: 24,
            marginBottom: 16,
            textAlign: "center",
          }}
        >
          Recording is not available
        </h1>
        <p style={{ color: "var(--color-text-secondary)", fontSize: 14, marginBottom: 24, maxWidth: 400, margin: "0 auto 24px" }}>
          Recording is not supported on this device. Please use a modern browser, or{" "}
          <Link to="/upload" style={{ color: "var(--color-accent)" }}>upload a video</Link> instead.
        </p>
        <Link
          to="/upload"
          style={{
            background: "var(--color-accent)",
            color: "var(--color-text)",
            borderRadius: 8,
            padding: "10px 24px",
            fontSize: 14,
            fontWeight: 600,
            textDecoration: "none",
          }}
        >
          Go to Upload
        </Link>
      </div>
    );
  }

  if (quotaReached) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-error)", fontSize: 16, marginBottom: 16 }}>
          You've reached your limit of {limits!.maxVideosPerMonth} videos this month.
          Delete unused recordings or wait until next month.
        </p>
        <div style={{ display: "flex", gap: 12, flexWrap: "wrap", justifyContent: "center" }}>
          <Link
            to="/library"
            style={{
              background: "var(--color-accent)",
              color: "var(--color-text)",
              borderRadius: 8,
              padding: "10px 24px",
              fontSize: 14,
              fontWeight: 600,
              textDecoration: "none",
            }}
          >
            Go to Library
          </Link>
          <Link
            to="/settings"
            style={{
              background: "transparent",
              color: "var(--color-accent)",
              border: "1px solid var(--color-accent)",
              borderRadius: 8,
              padding: "10px 24px",
              fontSize: 14,
              fontWeight: 600,
              textDecoration: "none",
            }}
          >
            Upgrade to Pro
          </Link>
        </div>
      </div>
    );
  }

  if (shareUrl) {
    return (
      <div className="page-container page-container--centered">
        <h2
          style={{
            color: "var(--color-text)",
            fontSize: 20,
            marginBottom: 16,
          }}
        >
          Recording complete
        </h2>

        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            marginBottom: 16,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: 12,
          }}
        >
          <span
            style={{
              color: "var(--color-text-secondary)",
              fontSize: 14,
              wordBreak: "break-all",
            }}
          >
            {shareUrl}
          </span>
          <button
            onClick={copyShareUrl}
            style={{
              background: "var(--color-accent)",
              color: "var(--color-text)",
              borderRadius: 4,
              padding: "6px 16px",
              fontSize: 14,
              fontWeight: 600,
              whiteSpace: "nowrap",
            }}
          >
            {copied ? "Copied!" : "Copy link"}
          </button>
        </div>

        <div style={{ display: "flex", gap: 12, justifyContent: "center", flexWrap: "wrap" }}>
          <a
            href={shareUrl}
            target="_blank"
            rel="noopener noreferrer"
            style={{
              background: "var(--color-accent)",
              color: "var(--color-text)",
              borderRadius: 8,
              padding: "10px 24px",
              fontSize: 14,
              fontWeight: 600,
              textDecoration: "none",
            }}
          >
            Watch video
          </a>

          <button
            onClick={recordAnother}
            style={{
              background: "transparent",
              color: "var(--color-accent)",
              border: "1px solid var(--color-border)",
              borderRadius: 8,
              padding: "10px 24px",
              fontSize: 14,
              fontWeight: 600,
            }}
          >
            Record another
          </button>

          <Link
            to="/library"
            style={{
              background: "transparent",
              color: "var(--color-text-secondary)",
              border: "1px solid var(--color-border)",
              borderRadius: 8,
              padding: "10px 24px",
              fontSize: 14,
              fontWeight: 600,
              textDecoration: "none",
            }}
          >
            Go to Library
          </Link>
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
      <h1
        style={{
          color: "var(--color-text)",
          fontSize: 24,
          marginBottom: 24,
          textAlign: "center",
        }}
      >
        New Recording
      </h1>
      {hasLimits && (
        <div style={{ marginBottom: 16, maxWidth: 300, margin: "0 auto 16px" }}>
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13, marginBottom: 6 }}>
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
        <div style={{
          maxWidth: 400,
          margin: "0 auto 24px",
          padding: "20px 24px",
          background: "var(--color-surface)",
          borderRadius: 12,
          textAlign: "left",
        }}>
          <p style={{ color: "var(--color-text)", fontSize: 15, fontWeight: 600, marginBottom: 12 }}>
            Get started in 3 steps
          </p>
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span style={{ color: "var(--color-accent)", fontWeight: 700, fontSize: 16 }}>1.</span>
              <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Record your screen or upload a video</span>
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span style={{ color: "var(--color-accent)", fontWeight: 700, fontSize: 16 }}>2.</span>
              <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Share the link with anyone</span>
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span style={{ color: "var(--color-accent)", fontWeight: 700, fontSize: 16 }}>3.</span>
              <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Track views and get feedback</span>
            </div>
          </div>
        </div>
      )}
      {screenRecordingSupported ? (
        <Recorder
          onRecordingComplete={handleRecordingComplete}
          maxDurationSeconds={limits?.maxVideoDurationSeconds ?? 0}
        />
      ) : (
        <CameraRecorder
          onRecordingComplete={handleRecordingComplete}
          maxDurationSeconds={limits?.maxVideoDurationSeconds ?? 0}
        />
      )}
    </div>
  );
}
