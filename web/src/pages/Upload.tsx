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
  const [progress, setProgress] = useState(0);
  const [shareUrl, setShareUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const dragCounter = useRef(0);

  function acceptFile(selected: File) {
    setFile(selected);
    const nameWithoutExt = selected.name.replace(/\.[^.]+$/, "");
    setTitle(nameWithoutExt);
    setError(null);
  }

  function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const selected = e.target.files?.[0];
    if (!selected) return;
    acceptFile(selected);
  }

  function handleDragEnter(e: React.DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    dragCounter.current++;
    setDragging(true);
  }

  function handleDragLeave(e: React.DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    dragCounter.current--;
    if (dragCounter.current === 0) {
      setDragging(false);
    }
  }

  function handleDragOver(e: React.DragEvent) {
    e.preventDefault();
    e.stopPropagation();
  }

  function handleDrop(e: React.DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    dragCounter.current = 0;
    setDragging(false);

    const dropped = e.dataTransfer.files[0];
    if (!dropped) return;

    if (dropped.type !== "video/mp4" && dropped.type !== "video/webm" && dropped.type !== "video/quicktime") {
      setError("Only MP4, WebM, and MOV files are supported");
      return;
    }

    acceptFile(dropped);
  }

  async function handleUpload() {
    if (!file) return;

    setUploading(true);
    setError(null);
    setProgress(0);
    let videoId: string | null = null;

    try {
      const fileContentType = file.type === "video/webm" ? "video/webm" : file.type === "video/quicktime" ? "video/quicktime" : "video/mp4";

      setProgress(10);

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
      setProgress(20);

      const uploadResp = await fetch(result.uploadUrl, {
        method: "PUT",
        body: file,
        headers: { "Content-Type": fileContentType },
      });

      if (!uploadResp.ok) {
        throw new Error("Upload failed");
      }

      setProgress(80);

      await apiFetch(`/api/videos/${result.id}`, {
        method: "PATCH",
        body: JSON.stringify({ status: "ready" }),
      });

      setProgress(100);
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
    setProgress(0);
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  }

  function formatFileSize(bytes: number): string {
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
  }

  if (uploading) {
    return (
      <div style={{ display: "flex", flexDirection: "column", alignItems: "center", minHeight: "calc(100vh - 56px)", padding: 24 }}>
        <div style={{ width: "100%", maxWidth: 480 }}>
          <div style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 12,
            padding: 32,
            textAlign: "center",
          }}>
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--color-accent)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ marginBottom: 16, animation: "pulse 1.5s ease-in-out infinite" }}>
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="17 8 12 3 7 8" />
              <line x1="12" y1="3" x2="12" y2="15" />
            </svg>
            <p style={{ color: "var(--color-text)", fontSize: 16, fontWeight: 600, marginBottom: 8 }}>
              Uploading...
            </p>
            <p style={{ color: "var(--color-text-secondary)", fontSize: 13, marginBottom: 16 }}>
              {file?.name}
            </p>
            <div style={{
              background: "var(--color-bg)",
              borderRadius: 4,
              height: 6,
              overflow: "hidden",
            }}>
              <div style={{
                background: "var(--color-accent)",
                height: "100%",
                width: `${progress}%`,
                borderRadius: 4,
                transition: "width 0.3s ease",
              }} />
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div style={{ display: "flex", flexDirection: "column", alignItems: "center", minHeight: "calc(100vh - 56px)", padding: 24 }}>
        <div style={{ width: "100%", maxWidth: 480 }}>
          <div style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 12,
            padding: 32,
            textAlign: "center",
          }}>
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--color-error)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ marginBottom: 16 }}>
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="8" x2="12" y2="12" />
              <line x1="12" y1="16" x2="12.01" y2="16" />
            </svg>
            <p style={{ color: "var(--color-error)", fontSize: 16, fontWeight: 600, marginBottom: 8 }}>
              Upload failed
            </p>
            <p style={{ color: "var(--color-text-secondary)", fontSize: 14, marginBottom: 24 }}>
              {error}
            </p>
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
        </div>
      </div>
    );
  }

  if (shareUrl) {
    return (
      <div style={{ display: "flex", flexDirection: "column", alignItems: "center", minHeight: "calc(100vh - 56px)", padding: 24 }}>
        <div style={{ width: "100%", maxWidth: 480 }}>
          <div style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 12,
            padding: 32,
            textAlign: "center",
          }}>
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--color-accent)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ marginBottom: 16 }}>
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
              <polyline points="22 4 12 14.01 9 11.01" />
            </svg>
            <h2 style={{ color: "var(--color-text)", fontSize: 20, fontWeight: 600, marginBottom: 16 }}>
              Upload complete
            </h2>

            <div style={{
              background: "var(--color-bg)",
              borderRadius: 8,
              padding: "10px 16px",
              marginBottom: 24,
              display: "flex",
              alignItems: "center",
              gap: 8,
            }}>
              <span style={{ color: "var(--color-text-secondary)", fontSize: 13, wordBreak: "break-all", flex: 1, textAlign: "left" }}>
                {shareUrl}
              </span>
              <button
                onClick={copyShareUrl}
                style={{
                  background: "var(--color-accent)",
                  color: "var(--color-text)",
                  borderRadius: 6,
                  padding: "6px 14px",
                  fontSize: 13,
                  fontWeight: 600,
                  whiteSpace: "nowrap",
                }}
              >
                {copied ? "Copied!" : "Copy"}
              </button>
            </div>

            <div style={{ display: "flex", gap: 10, justifyContent: "center", flexWrap: "wrap" }}>
              <a
                href={shareUrl}
                target="_blank"
                rel="noopener noreferrer"
                style={{
                  background: "var(--color-accent)",
                  color: "var(--color-text)",
                  borderRadius: 8,
                  padding: "10px 20px",
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
                  padding: "10px 20px",
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
                  padding: "10px 20px",
                  fontSize: 14,
                  fontWeight: 600,
                  textDecoration: "none",
                }}
              >
                Go to Library
              </Link>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", alignItems: "center", minHeight: "calc(100vh - 56px)", padding: 24 }}>
      <h1 style={{ color: "var(--color-text)", fontSize: 24, fontWeight: 600, marginBottom: 24, textAlign: "center" }}>
        Upload Video
      </h1>

      <div style={{ width: "100%", maxWidth: 480, display: "flex", flexDirection: "column", gap: 16 }}>
        <input
          ref={fileInputRef}
          type="file"
          accept="video/mp4,video/webm,video/quicktime,.mov"
          onChange={handleFileSelect}
          data-testid="file-input"
          style={{ display: "none" }}
        />

        <div
          onClick={() => fileInputRef.current?.click()}
          onDragEnter={handleDragEnter}
          onDragLeave={handleDragLeave}
          onDragOver={handleDragOver}
          onDrop={handleDrop}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") fileInputRef.current?.click(); }}
          style={{
            border: `2px dashed ${dragging ? "var(--color-accent)" : "var(--color-border)"}`,
            borderRadius: 12,
            padding: file ? "20px 24px" : "48px 24px",
            textAlign: "center",
            cursor: "pointer",
            background: dragging ? "rgba(0, 182, 122, 0.05)" : "var(--color-surface)",
            transition: "border-color 0.2s, background 0.2s",
          }}
        >
          {file ? (
            <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
              <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="var(--color-accent)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <polygon points="23 7 16 12 23 17 23 7" />
                <rect x="1" y="5" width="15" height="14" rx="2" ry="2" />
              </svg>
              <div style={{ flex: 1, textAlign: "left", minWidth: 0 }}>
                <p style={{ color: "var(--color-text)", fontSize: 14, fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {file.name}
                </p>
                <p style={{ color: "var(--color-text-secondary)", fontSize: 12 }}>
                  {formatFileSize(file.size)} &middot; {file.type === "video/webm" ? "WebM" : file.type === "video/quicktime" ? "MOV" : "MP4"}
                </p>
              </div>
              <button
                onClick={(e) => { e.stopPropagation(); uploadAnother(); }}
                style={{
                  background: "transparent",
                  color: "var(--color-text-secondary)",
                  fontSize: 12,
                  padding: "4px 8px",
                  borderRadius: 4,
                  border: "1px solid var(--color-border)",
                }}
              >
                Change
              </button>
            </div>
          ) : (
            <>
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke={dragging ? "var(--color-accent)" : "var(--color-text-secondary)"} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" style={{ marginBottom: 16, transition: "stroke 0.2s" }}>
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                <polyline points="17 8 12 3 7 8" />
                <line x1="12" y1="3" x2="12" y2="15" />
              </svg>
              <p style={{ color: "var(--color-text)", fontSize: 15, fontWeight: 500, marginBottom: 4 }}>
                Drag and drop your video here
              </p>
              <p style={{ color: "var(--color-text-secondary)", fontSize: 13, marginBottom: 16 }}>
                or click to browse
              </p>
              <span style={{
                display: "inline-block",
                background: "var(--color-bg)",
                color: "var(--color-text-secondary)",
                fontSize: 12,
                padding: "4px 12px",
                borderRadius: 4,
              }}>
                MP4, WebM, MOV
              </span>
            </>
          )}
        </div>

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
                  padding: "10px 12px",
                  fontSize: 14,
                  borderRadius: 8,
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
                padding: "12px 32px",
                fontSize: 15,
                fontWeight: 600,
                alignSelf: "center",
                width: "100%",
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
