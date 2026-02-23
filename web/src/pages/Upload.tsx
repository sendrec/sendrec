import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { apiFetch } from "../api/client";

interface UploadResponse {
  id: string;
  uploadUrl: string;
  shareToken: string;
}

interface LimitsResponse {
  maxVideosPerMonth: number;
  videosUsedThisMonth: number;
}

interface FileEntry {
  file: File;
  title: string;
}

interface UploadResult {
  fileName: string;
  shareUrl: string;
  error?: string;
}

const MAX_FILES = 10;
const SUPPORTED_TYPES = ["video/mp4", "video/webm", "video/quicktime"];

export function Upload() {
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [uploading, setUploading] = useState(false);
  const [currentFileIndex, setCurrentFileIndex] = useState(0);
  const [progress, setProgress] = useState(0);
  const [results, setResults] = useState<UploadResult[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const dragCounter = useRef(0);
  const [limits, setLimits] = useState<LimitsResponse | null>(null);

  useEffect(() => {
    apiFetch<LimitsResponse>("/api/videos/limits")
      .then((result) => setLimits(result ?? null))
      .catch(() => {});
  }, []);

  function acceptFiles(selected: File[]) {
    const valid = selected.filter((f) => SUPPORTED_TYPES.includes(f.type));
    if (valid.length === 0) {
      setError("Only MP4, WebM, and MOV files are supported");
      return;
    }
    if (valid.length < selected.length) {
      setError(`${selected.length - valid.length} unsupported file(s) skipped`);
    } else {
      setError(null);
    }

    const total = files.length + valid.length;
    if (total > MAX_FILES) {
      const allowed = valid.slice(0, MAX_FILES - files.length);
      if (allowed.length === 0) {
        setError(`Maximum ${MAX_FILES} files allowed`);
        return;
      }
      setError(`Only ${allowed.length} of ${valid.length} files added (maximum ${MAX_FILES})`);
      setFiles((prev) => [
        ...prev,
        ...allowed.map((f) => ({ file: f, title: f.name.replace(/\.[^.]+$/, "") })),
      ]);
      return;
    }

    setFiles((prev) => [
      ...prev,
      ...valid.map((f) => ({ file: f, title: f.name.replace(/\.[^.]+$/, "") })),
    ]);
  }

  function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const selected = Array.from(e.target.files || []);
    if (selected.length === 0) return;
    acceptFiles(selected);
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

    const dropped = Array.from(e.dataTransfer.files);
    if (dropped.length === 0) return;
    acceptFiles(dropped);
  }

  function removeFile(index: number) {
    setFiles((prev) => prev.filter((_, i) => i !== index));
    setError(null);
  }

  function updateTitle(index: number, title: string) {
    setFiles((prev) => prev.map((f, i) => (i === index ? { ...f, title } : f)));
  }

  async function handleUpload() {
    if (files.length === 0) return;

    setError(null);

    // Check monthly quota upfront
    try {
      const limits = await apiFetch<LimitsResponse>("/api/videos/limits");
      if (limits && limits.maxVideosPerMonth > 0) {
        const remaining = limits.maxVideosPerMonth - limits.videosUsedThisMonth;
        if (files.length > remaining) {
          setError(
            remaining <= 0
              ? "Monthly video limit reached"
              : `You can only upload ${remaining} more video${remaining === 1 ? "" : "s"} this month`
          );
          return;
        }
      }
    } catch {
      // If limits check fails, proceed and let backend enforce
    }

    setUploading(true);
    setCurrentFileIndex(0);
    setProgress(0);

    const uploadResults: UploadResult[] = [];

    for (let i = 0; i < files.length; i++) {
      setCurrentFileIndex(i);
      setProgress(0);
      const entry = files[i];
      let videoId: string | null = null;

      try {
        const fileContentType =
          entry.file.type === "video/webm"
            ? "video/webm"
            : entry.file.type === "video/quicktime"
              ? "video/quicktime"
              : "video/mp4";

        setProgress(10);

        const result = await apiFetch<UploadResponse>("/api/videos/upload", {
          method: "POST",
          body: JSON.stringify({
            title: entry.title || entry.file.name.replace(/\.[^.]+$/, ""),
            fileSize: entry.file.size,
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
          body: entry.file,
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
        uploadResults.push({
          fileName: entry.file.name,
          shareUrl: `${window.location.origin}/watch/${result.shareToken}`,
        });
      } catch (err) {
        if (videoId) {
          apiFetch(`/api/videos/${videoId}`, { method: "DELETE" }).catch(() => {});
        }
        uploadResults.push({
          fileName: entry.file.name,
          shareUrl: "",
          error: err instanceof Error ? err.message : "Upload failed",
        });
      }
    }

    setUploading(false);
    setResults(uploadResults);
  }

  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);

  async function copyShareUrl(url: string, index: number) {
    try {
      await navigator.clipboard.writeText(url);
    } catch {
      const textArea = document.createElement("textarea");
      textArea.value = url;
      textArea.style.position = "fixed";
      textArea.style.opacity = "0";
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand("copy");
      document.body.removeChild(textArea);
    }
    setCopiedIndex(index);
    setTimeout(() => setCopiedIndex(null), 2000);
  }

  function uploadAnother() {
    setFiles([]);
    setResults(null);
    setError(null);
    setProgress(0);
    setCopiedIndex(null);
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  }

  function formatFileSize(bytes: number): string {
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
  }

  function formatType(type: string): string {
    if (type === "video/webm") return "WebM";
    if (type === "video/quicktime") return "MOV";
    return "MP4";
  }

  if (uploading) {
    const currentFile = files[currentFileIndex];
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
              Uploading {currentFileIndex + 1} of {files.length}...
            </p>
            <p style={{ color: "var(--color-text-secondary)", fontSize: 13, marginBottom: 16 }}>
              {currentFile?.file.name}
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

  if (results) {
    const succeeded = results.filter((r) => !r.error);
    const failed = results.filter((r) => r.error);

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
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke={failed.length > 0 ? "var(--color-warning, #f59e0b)" : "var(--color-accent)"} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ marginBottom: 16 }}>
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
              <polyline points="22 4 12 14.01 9 11.01" />
            </svg>
            <h2 style={{ color: "var(--color-text)", fontSize: 20, fontWeight: 600, marginBottom: 16 }}>
              {failed.length === 0
                ? results.length === 1
                  ? "Upload complete"
                  : `${succeeded.length} videos uploaded`
                : `${succeeded.length} of ${results.length} uploaded`}
            </h2>

            {succeeded.map((result, i) => (
              <div key={i} style={{
                background: "var(--color-bg)",
                borderRadius: 8,
                padding: "10px 16px",
                marginBottom: 8,
                display: "flex",
                alignItems: "center",
                gap: 8,
              }}>
                <span style={{ color: "var(--color-text-secondary)", fontSize: 13, wordBreak: "break-all", flex: 1, textAlign: "left" }}>
                  {result.shareUrl}
                </span>
                <button
                  onClick={() => copyShareUrl(result.shareUrl, i)}
                  data-testid={`copy-btn-${i}`}
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
                  {copiedIndex === i ? "Copied!" : "Copy"}
                </button>
              </div>
            ))}

            {failed.map((result, i) => (
              <div key={`err-${i}`} style={{
                background: "var(--color-bg)",
                borderRadius: 8,
                padding: "10px 16px",
                marginBottom: 8,
                display: "flex",
                alignItems: "center",
                gap: 8,
              }}>
                <span style={{ color: "var(--color-error)", fontSize: 13, flex: 1, textAlign: "left" }}>
                  {result.fileName}: {result.error}
                </span>
              </div>
            ))}

            <div style={{ display: "flex", gap: 10, justifyContent: "center", flexWrap: "wrap", marginTop: 16 }}>
              <button
                onClick={uploadAnother}
                style={{
                  background: "var(--color-accent)",
                  color: "var(--color-text)",
                  borderRadius: 8,
                  padding: "10px 20px",
                  fontSize: 14,
                  fontWeight: 600,
                }}
              >
                Upload more
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

      <div style={{ width: "100%", maxWidth: 480, display: "flex", flexDirection: "column", gap: 16 }}>
        <input
          ref={fileInputRef}
          type="file"
          accept="video/mp4,video/webm,video/quicktime,.mov"
          multiple
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
            padding: files.length > 0 ? "20px 24px" : "48px 24px",
            textAlign: "center",
            cursor: "pointer",
            background: dragging ? "var(--color-drag-highlight)" : "var(--color-surface)",
            transition: "border-color 0.2s, background 0.2s",
          }}
        >
          {files.length > 0 ? (
            <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
              <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="var(--color-accent)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <polygon points="23 7 16 12 23 17 23 7" />
                <rect x="1" y="5" width="15" height="14" rx="2" ry="2" />
              </svg>
              <div style={{ flex: 1, textAlign: "left", minWidth: 0 }}>
                <p style={{ color: "var(--color-text)", fontSize: 14, fontWeight: 500 }}>
                  {files.length} file{files.length !== 1 ? "s" : ""} selected
                </p>
                <p style={{ color: "var(--color-text-secondary)", fontSize: 12 }}>
                  Click or drop to add more
                </p>
              </div>
            </div>
          ) : (
            <>
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke={dragging ? "var(--color-accent)" : "var(--color-text-secondary)"} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" style={{ marginBottom: 16, transition: "stroke 0.2s" }}>
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                <polyline points="17 8 12 3 7 8" />
                <line x1="12" y1="3" x2="12" y2="15" />
              </svg>
              <p style={{ color: "var(--color-text)", fontSize: 15, fontWeight: 500, marginBottom: 4 }}>
                Drag and drop your videos here
              </p>
              <p style={{ color: "var(--color-text-secondary)", fontSize: 13, marginBottom: 16 }}>
                or click to browse (up to {MAX_FILES} files)
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

        {error && (
          <p style={{ color: "var(--color-error)", fontSize: 13, textAlign: "center" }}>
            {error}
          </p>
        )}

        {files.length > 0 && (
          <>
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              {files.map((entry, i) => (
                <div key={i} style={{
                  background: "var(--color-surface)",
                  border: "1px solid var(--color-border)",
                  borderRadius: 8,
                  padding: "10px 12px",
                  display: "flex",
                  alignItems: "center",
                  gap: 10,
                }}>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <input
                      type="text"
                      value={entry.title}
                      onChange={(e) => updateTitle(i, e.target.value)}
                      maxLength={500}
                      aria-label={`Title for ${entry.file.name}`}
                      style={{
                        display: "block",
                        width: "100%",
                        padding: "4px 8px",
                        fontSize: 13,
                        borderRadius: 4,
                        border: "1px solid var(--color-border)",
                        background: "var(--color-bg)",
                        color: "var(--color-text)",
                        boxSizing: "border-box",
                      }}
                    />
                    <p style={{ color: "var(--color-text-secondary)", fontSize: 11, marginTop: 2 }}>
                      {formatFileSize(entry.file.size)} &middot; {formatType(entry.file.type)}
                    </p>
                  </div>
                  <button
                    onClick={() => removeFile(i)}
                    aria-label={`Remove ${entry.file.name}`}
                    style={{
                      background: "transparent",
                      color: "var(--color-text-secondary)",
                      fontSize: 18,
                      padding: "2px 6px",
                      borderRadius: 4,
                      border: "none",
                      cursor: "pointer",
                      lineHeight: 1,
                    }}
                  >
                    &times;
                  </button>
                </div>
              ))}
            </div>

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
              Upload {files.length} video{files.length !== 1 ? "s" : ""}
            </button>
          </>
        )}
      </div>
    </div>
  );
}
