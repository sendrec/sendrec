import { useRef, useState } from "react";
import { Link } from "react-router-dom";
import { apiFetch } from "../api/client";

interface UploadResponse {
  id: string;
  uploadUrl: string;
  shareToken: string;
}

export function Upload() {
  const [file, setFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  const [uploading, setUploading] = useState(false);
  const [shareUrl, setShareUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const selected = e.target.files?.[0];
    if (!selected) return;
    setFile(selected);
    const nameWithoutExt = selected.name.replace(/\.[^.]+$/, "");
    setTitle(nameWithoutExt);
    setError(null);
  }

  async function handleUpload() {
    if (!file) return;

    setUploading(true);
    setError(null);
    let videoId: string | null = null;

    try {
      const fileContentType = file.type === "video/webm" ? "video/webm" : "video/mp4";

      const result = await apiFetch<UploadResponse>("/api/videos/upload", {
        method: "POST",
        body: JSON.stringify({
          title: title || file.name.replace(/\.[^.]+$/, ""),
          fileSize: file.size,
          contentType: fileContentType,
        }),
      });

      if (!result) {
        throw new Error("Failed to create upload");
      }

      videoId = result.id;

      const uploadResp = await fetch(result.uploadUrl, {
        method: "PUT",
        body: file,
        headers: { "Content-Type": fileContentType },
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

  function uploadAnother() {
    setFile(null);
    setTitle("");
    setShareUrl(null);
    setError(null);
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
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
          onClick={uploadAnother}
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
        <h2 style={{ color: "var(--color-text)", fontSize: 20, marginBottom: 16 }}>
          Upload complete
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
          <span style={{ color: "var(--color-text-secondary)", fontSize: 14, wordBreak: "break-all" }}>
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
            onClick={uploadAnother}
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
            Upload another
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
      <h1 style={{ color: "var(--color-text)", fontSize: 24, marginBottom: 24, textAlign: "center" }}>
        Upload Video
      </h1>

      <div style={{ width: "100%", maxWidth: 480, display: "flex", flexDirection: "column", gap: 16 }}>
        <input
          ref={fileInputRef}
          type="file"
          accept="video/mp4,video/webm"
          onChange={handleFileSelect}
          style={{ fontSize: 14 }}
          data-testid="file-input"
        />

        {file && (
          <>
            <label style={{ color: "var(--color-text-secondary)", fontSize: 13 }}>
              Title
              <input
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                maxLength={500}
                style={{
                  display: "block",
                  width: "100%",
                  marginTop: 4,
                  padding: "8px 12px",
                  fontSize: 14,
                  borderRadius: 6,
                  border: "1px solid var(--color-border)",
                  background: "var(--color-surface)",
                  color: "var(--color-text)",
                  boxSizing: "border-box",
                }}
              />
            </label>

            <button
              onClick={handleUpload}
              style={{
                background: "var(--color-accent)",
                color: "var(--color-text)",
                borderRadius: 8,
                padding: "10px 24px",
                fontSize: 14,
                fontWeight: 600,
                alignSelf: "center",
              }}
            >
              Upload
            </button>
          </>
        )}
      </div>
    </div>
  );
}
