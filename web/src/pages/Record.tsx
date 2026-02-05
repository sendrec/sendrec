import { useState } from "react";
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

      await fetch(result.uploadUrl, {
        method: "PUT",
        body: blob,
        headers: { "Content-Type": "video/webm" },
      });

      await apiFetch(`/api/videos/${result.id}`, {
        method: "PATCH",
        body: JSON.stringify({ status: "ready" }),
      });

      setShareUrl(`${window.location.origin}/watch/${result.shareToken}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Upload failed");
    } finally {
      setUploading(false);
    }
  }

  function copyShareUrl() {
    if (shareUrl) {
      navigator.clipboard.writeText(shareUrl);
    }
  }

  function recordAnother() {
    setShareUrl(null);
    setError(null);
  }

  if (uploading) {
    return (
      <div style={{ maxWidth: 800, margin: "80px auto", padding: 24, textAlign: "center" }}>
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16 }}>Uploading...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div style={{ maxWidth: 800, margin: "80px auto", padding: 24, textAlign: "center" }}>
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
      <div style={{ maxWidth: 800, margin: "80px auto", padding: 24, textAlign: "center" }}>
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
            Copy link
          </button>
        </div>

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
      </div>
    );
  }

  return (
    <div style={{ maxWidth: 800, margin: "80px auto", padding: 24 }}>
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
