import { useState } from "react";
import { Link } from "react-router-dom";
import { apiFetch } from "../api/client";
import { Recorder } from "../components/Recorder";

interface CreateVideoResponse {
  id: string;
  uploadUrl: string;
  shareToken: string;
}

export function Record() {
  const [uploading, setUploading] = useState(false);
  const [shareUrl, setShareUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function handleRecordingComplete(blob: Blob, duration: number) {
    setUploading(true);
    setError(null);
    let videoId: string | null = null;

    try {
      const now = new Date();
      const title = `Recording ${now.toLocaleDateString()} ${now.toLocaleTimeString()}`;

      const result = await apiFetch<CreateVideoResponse>("/api/videos", {
        method: "POST",
        body: JSON.stringify({ title, duration, fileSize: blob.size }),
      });

      if (!result) {
        throw new Error("Failed to create video");
      }

      videoId = result.id;

      const uploadResp = await fetch(result.uploadUrl, {
        method: "PUT",
        body: blob,
        headers: { "Content-Type": "video/webm" },
      });

      if (!uploadResp.ok) {
        throw new Error("Upload failed");
      }

      await apiFetch(`/api/videos/${result.id}`, {
        method: "PATCH",
        body: JSON.stringify({ status: "ready" }),
      });

      setShareUrl(`${window.location.origin}/watch/${result.shareToken}`);
    } catch (err) {
      if (videoId) {
        // Best-effort cleanup so “uploading” rows don’t accumulate.
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
      <Recorder onRecordingComplete={handleRecordingComplete} />
    </div>
  );
}
