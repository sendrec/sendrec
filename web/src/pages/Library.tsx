import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { apiFetch } from "../api/client";
import { TrimModal } from "../components/TrimModal";

interface Video {
  id: string;
  title: string;
  status: string;
  duration: number;
  shareToken: string;
  shareUrl: string;
  createdAt: string;
  shareExpiresAt: string | null;
  viewCount: number;
  uniqueViewCount: number;
  thumbnailUrl?: string;
  hasPassword: boolean;
  commentMode: string;
  commentCount: number;
  transcriptStatus: string;
  viewNotification: string | null;
  downloadEnabled: boolean;
}

interface LimitsResponse {
  maxVideosPerMonth: number;
  maxVideoDurationSeconds: number;
  videosUsedThisMonth: number;
  brandingEnabled: boolean;
}

interface VideoBranding {
  companyName: string | null;
  colorBackground: string | null;
  colorSurface: string | null;
  colorText: string | null;
  colorAccent: string | null;
  footerText: string | null;
}

function formatDuration(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = Math.floor(seconds % 60);
  return `${minutes}:${String(remainingSeconds).padStart(2, "0")}`;
}

function formatDate(isoDate: string): string {
  return new Date(isoDate).toLocaleDateString();
}

const commentModeLabels: Record<string, string> = {
  disabled: "Comments off",
  anonymous: "Comments: anonymous",
  name_required: "Comments: name required",
  name_email_required: "Comments: name + email",
};

const commentModeOrder = ["disabled", "anonymous", "name_required", "name_email_required"];

function expiryLabel(shareExpiresAt: string | null): { text: string; expired: boolean } {
  if (shareExpiresAt === null) {
    return { text: "Never expires", expired: false };
  }
  const expiry = new Date(shareExpiresAt);
  const now = new Date();
  if (expiry <= now) {
    return { text: "Expired", expired: true };
  }
  const diffMs = expiry.getTime() - now.getTime();
  const diffDays = Math.ceil(diffMs / (1000 * 60 * 60 * 24));
  if (diffDays === 1) {
    return { text: "Expires tomorrow", expired: false };
  }
  return { text: `Expires in ${diffDays} days`, expired: false };
}

export function Library() {
  const [videos, setVideos] = useState<Video[]>([]);
  const [loading, setLoading] = useState(true);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [extendingId, setExtendingId] = useState<string | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [copiedEmbedId, setCopiedEmbedId] = useState<string | null>(null);
  const [downloadingId, setDownloadingId] = useState<string | null>(null);
  const [limits, setLimits] = useState<LimitsResponse | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editTitle, setEditTitle] = useState("");
  const [trimmingId, setTrimmingId] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [brandingVideoId, setBrandingVideoId] = useState<string | null>(null);
  const [videoBranding, setVideoBranding] = useState<VideoBranding>({
    companyName: null, colorBackground: null, colorSurface: null, colorText: null, colorAccent: null, footerText: null,
  });
  const [savingBranding, setSavingBranding] = useState(false);
  const [brandingMessage, setBrandingMessage] = useState("");
  const [uploadingThumbnailId, setUploadingThumbnailId] = useState<string | null>(null);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  const fetchVideosAndLimits = useCallback(async (query = "") => {
    const searchParam = query ? `?q=${encodeURIComponent(query)}` : "";
    const [videosResult, limitsResult] = await Promise.all([
      apiFetch<Video[]>(`/api/videos${searchParam}`),
      apiFetch<LimitsResponse>("/api/videos/limits"),
    ]);
    setVideos(videosResult ?? []);
    setLimits(limitsResult ?? null);
  }, []);

  useEffect(() => {
    async function fetchData() {
      try {
        await fetchVideosAndLimits();
      } catch {
        setVideos([]);
      } finally {
        setLoading(false);
      }
    }

    fetchData();
  }, [fetchVideosAndLimits]);

  useEffect(() => {
    const hasProcessing = videos.some(
      (v) => v.status === "processing" || v.transcriptStatus === "processing" || v.transcriptStatus === "pending"
    );
    if (!hasProcessing) return;

    const interval = setInterval(async () => {
      try {
        await fetchVideosAndLimits(searchQuery);
      } catch {
        // ignore poll errors
      }
    }, 5000);

    return () => clearInterval(interval);
  }, [videos, searchQuery, fetchVideosAndLimits]);

  useEffect(() => {
    if (!openMenuId) return;
    function handleClick(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setOpenMenuId(null);
      }
    }
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") setOpenMenuId(null);
    }
    document.addEventListener("mousedown", handleClick);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handleClick);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [openMenuId]);

  function handleSearchChange(value: string) {
    setSearchQuery(value);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      fetchVideosAndLimits(value);
    }, 300);
  }

  async function deleteVideo(id: string) {
    if (!window.confirm("Delete this recording? This cannot be undone.")) {
      return;
    }
    setDeletingId(id);
    try {
      await apiFetch(`/api/videos/${id}`, { method: "DELETE" });
      setVideos((prev) => prev.filter((v) => v.id !== id));
    } finally {
      setDeletingId(null);
    }
  }

  async function copyLink(shareUrl: string, videoId: string) {
    try {
      await navigator.clipboard.writeText(shareUrl);
      setCopiedId(videoId);
      setTimeout(() => setCopiedId(null), 2000);
    } catch {
      // Fallback for browsers/contexts where clipboard API is unavailable
      const textArea = document.createElement("textarea");
      textArea.value = shareUrl;
      textArea.style.position = "fixed";
      textArea.style.opacity = "0";
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand("copy");
      document.body.removeChild(textArea);
      setCopiedId(videoId);
      setTimeout(() => setCopiedId(null), 2000);
    }
  }

  async function downloadVideo(id: string) {
    setDownloadingId(id);
    try {
      const resp = await apiFetch<{ downloadUrl: string }>(`/api/videos/${id}/download`);
      if (resp?.downloadUrl) {
        window.location.href = resp.downloadUrl;
      }
    } finally {
      setDownloadingId(null);
    }
  }

  async function toggleDownload(video: Video) {
    const newValue = !video.downloadEnabled;
    await apiFetch(`/api/videos/${video.id}/download-enabled`, {
      method: "PUT",
      body: JSON.stringify({ downloadEnabled: newValue }),
    });
    setVideos((prev) => prev.map((v) => (v.id === video.id ? { ...v, downloadEnabled: newValue } : v)));
  }

  async function extendVideo(id: string) {
    setExtendingId(id);
    try {
      await apiFetch(`/api/videos/${id}/extend`, { method: "POST" });
      const result = await apiFetch<Video[]>("/api/videos");
      setVideos(result ?? []);
    } finally {
      setExtendingId(null);
    }
  }

  async function toggleLinkExpiry(video: Video) {
    const neverExpires = video.shareExpiresAt !== null;
    await apiFetch(`/api/videos/${video.id}/link-expiry`, {
      method: "PUT",
      body: JSON.stringify({ neverExpires }),
    });
    const result = await apiFetch<Video[]>("/api/videos");
    setVideos(result ?? []);
  }

  async function addPassword(id: string) {
    const password = window.prompt("Enter a password for this video:");
    if (!password) return;
    await apiFetch(`/api/videos/${id}/password`, {
      method: "PUT",
      body: JSON.stringify({ password }),
    });
    const [videosResult, limitsResult] = await Promise.all([
      apiFetch<Video[]>("/api/videos"),
      apiFetch<LimitsResponse>("/api/videos/limits"),
    ]);
    setVideos(videosResult ?? []);
    setLimits(limitsResult ?? null);
  }

  async function removePassword(id: string) {
    if (!window.confirm("Remove the password from this video?")) return;
    await apiFetch(`/api/videos/${id}/password`, {
      method: "PUT",
      body: JSON.stringify({ password: "" }),
    });
    const [videosResult, limitsResult] = await Promise.all([
      apiFetch<Video[]>("/api/videos"),
      apiFetch<LimitsResponse>("/api/videos/limits"),
    ]);
    setVideos(videosResult ?? []);
    setLimits(limitsResult ?? null);
  }

  async function saveTitle(id: string) {
    const original = videos.find((v) => v.id === id)?.title;
    if (!editTitle.trim() || editTitle === original) {
      setEditingId(null);
      return;
    }
    await apiFetch(`/api/videos/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ title: editTitle }),
    });
    setVideos((prev) => prev.map((v) => (v.id === id ? { ...v, title: editTitle } : v)));
    setEditingId(null);
  }

  async function retranscribeVideo(id: string) {
    await apiFetch(`/api/videos/${id}/retranscribe`, { method: "POST" });
    setVideos((prev) => prev.map((v) => (v.id === id ? { ...v, transcriptStatus: "pending" } : v)));
  }

  async function cycleCommentMode(video: Video) {
    const currentIndex = commentModeOrder.indexOf(video.commentMode);
    const nextMode = commentModeOrder[(currentIndex + 1) % commentModeOrder.length];
    await apiFetch(`/api/videos/${video.id}/comment-mode`, {
      method: "PUT",
      body: JSON.stringify({ commentMode: nextMode }),
    });
    setVideos((prev) => prev.map((v) => (v.id === video.id ? { ...v, commentMode: nextMode } : v)));
  }

  async function changeNotification(video: Video, value: string) {
    const viewNotification = value === "" ? null : value;
    try {
      await apiFetch(`/api/videos/${video.id}/notifications`, {
        method: "PUT",
        body: JSON.stringify({ viewNotification }),
      });
      setVideos((prev) => prev.map((v) => (v.id === video.id ? { ...v, viewNotification } : v)));
    } catch {
      // no-op: select stays at previous value since state is only updated on success
    }
  }

  async function openBranding(videoId: string) {
    setBrandingVideoId(videoId);
    setBrandingMessage("");
    try {
      const data = await apiFetch<VideoBranding>(`/api/videos/${videoId}/branding`);
      if (data) setVideoBranding(data);
      else setVideoBranding({ companyName: null, colorBackground: null, colorSurface: null, colorText: null, colorAccent: null, footerText: null });
    } catch {
      setVideoBranding({ companyName: null, colorBackground: null, colorSurface: null, colorText: null, colorAccent: null, footerText: null });
    }
  }

  async function saveBranding() {
    if (!brandingVideoId) return;
    setSavingBranding(true);
    setBrandingMessage("");
    try {
      await apiFetch(`/api/videos/${brandingVideoId}/branding`, {
        method: "PUT",
        body: JSON.stringify({
          companyName: videoBranding.companyName || null,
          colorBackground: videoBranding.colorBackground || null,
          colorSurface: videoBranding.colorSurface || null,
          colorText: videoBranding.colorText || null,
          colorAccent: videoBranding.colorAccent || null,
          footerText: videoBranding.footerText || null,
        }),
      });
      setBrandingMessage("Saved");
      setTimeout(() => setBrandingVideoId(null), 1000);
    } catch (err) {
      setBrandingMessage(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSavingBranding(false);
    }
  }

  async function uploadThumbnail(videoId: string, file: File) {
    if (file.size > 2 * 1024 * 1024) return;
    const validTypes = ["image/jpeg", "image/png", "image/webp"];
    if (!validTypes.includes(file.type)) return;
    setUploadingThumbnailId(videoId);
    try {
      const result = await apiFetch<{ uploadUrl: string }>(`/api/videos/${videoId}/thumbnail`, {
        method: "POST",
        body: JSON.stringify({ contentType: file.type, contentLength: file.size }),
      });
      if (!result) return;
      const uploadResp = await fetch(result.uploadUrl, {
        method: "PUT",
        headers: { "Content-Type": file.type },
        body: file,
      });
      if (!uploadResp.ok) return;
      await fetchVideosAndLimits(searchQuery);
    } finally {
      setUploadingThumbnailId(null);
    }
  }

  async function resetThumbnail(videoId: string) {
    setUploadingThumbnailId(videoId);
    try {
      await apiFetch(`/api/videos/${videoId}/thumbnail`, { method: "DELETE" });
      await fetchVideosAndLimits(searchQuery);
    } finally {
      setUploadingThumbnailId(null);
    }
  }

  if (loading) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16 }}>Loading...</p>
      </div>
    );
  }

  if (videos.length === 0) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16, marginBottom: 16 }}>No recordings yet.</p>
        <Link
          to="/"
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
          Create your first recording
        </Link>
        {limits && limits.maxVideosPerMonth > 0 && (
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13, marginTop: 16 }}>
            {limits.videosUsedThisMonth} / {limits.maxVideosPerMonth} videos this month
          </p>
        )}
      </div>
    );
  }

  return (
    <div className="page-container">
      <div className="library-header">
        <div>
          <h1 style={{ color: "var(--color-text)", fontSize: 24, margin: 0 }}>
            Library
          </h1>
          {limits && limits.maxVideosPerMonth > 0 && (
            <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "4px 0 0" }}>
              {limits.videosUsedThisMonth} / {limits.maxVideosPerMonth} videos this month
            </p>
          )}
        </div>
        <input
          type="text"
          placeholder="Search videos..."
          value={searchQuery}
          onChange={(e) => handleSearchChange(e.target.value)}
          className="library-search"
        />
        <Link
          to="/"
          style={{
            background: "var(--color-accent)",
            color: "var(--color-text)",
            borderRadius: 8,
            padding: "8px 20px",
            fontSize: 14,
            fontWeight: 600,
            textDecoration: "none",
            whiteSpace: "nowrap",
          }}
        >
          New Recording
        </Link>
      </div>

      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {videos.map((video) => (
          <div
            key={video.id}
            className="video-card"
            style={{
              background: "var(--color-surface)",
              border: "1px solid var(--color-border)",
              borderRadius: 8,
              padding: 16,
            }}
          >
            <div className="video-card-top">
              {video.thumbnailUrl && (
                <a href={`/watch/${video.shareToken}`} style={{ flexShrink: 0 }}>
                  <img
                    src={video.thumbnailUrl}
                    alt=""
                    style={{
                      width: 120,
                      height: 68,
                      objectFit: "cover",
                      borderRadius: 4,
                      background: "var(--color-border)",
                      display: "block",
                    }}
                  />
                </a>
              )}
              <div style={{ minWidth: 0, flex: 1 }}>
                {editingId === video.id ? (
                  <input
                    value={editTitle}
                    onChange={(e) => setEditTitle(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") saveTitle(video.id);
                      if (e.key === "Escape") setEditingId(null);
                    }}
                    onBlur={() => saveTitle(video.id)}
                    autoFocus
                    style={{
                      fontWeight: 600,
                      fontSize: 15,
                      color: "var(--color-text)",
                      background: "var(--color-surface)",
                      border: "1px solid var(--color-accent)",
                      borderRadius: 4,
                      padding: "2px 6px",
                      margin: 0,
                      width: "100%",
                      outline: "none",
                    }}
                  />
                ) : (
                  <a
                    href={`/watch/${video.shareToken}`}
                    onClick={(e) => {
                      e.preventDefault();
                      setEditingId(video.id);
                      setEditTitle(video.title);
                    }}
                    style={{
                      fontWeight: 600,
                      fontSize: 15,
                      color: "var(--color-text)",
                      margin: 0,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                      display: "block",
                      textDecoration: "none",
                      cursor: "text",
                    }}
                  >
                    {video.title}
                  </a>
                )}
                <p
                  style={{
                    color: "var(--color-text-secondary)",
                    fontSize: 13,
                    margin: "4px 0 0",
                  }}
                >
                  {formatDuration(video.duration)} &middot; {formatDate(video.createdAt)}
                  {video.status === "uploading" && (
                    <span style={{ color: "var(--color-accent)", marginLeft: 8 }}>
                      uploading...
                    </span>
                  )}
                  {video.status === "processing" && (
                    <span style={{ color: "var(--color-accent)", marginLeft: 8 }}>
                      processing...
                    </span>
                  )}
                  {video.status === "ready" && video.viewCount > 0 && (
                    <span style={{ marginLeft: 8 }}>
                      &middot; {video.viewCount === video.uniqueViewCount
                        ? `${video.viewCount} view${video.viewCount !== 1 ? "s" : ""}`
                        : `${video.viewCount} views (${video.uniqueViewCount} unique)`}
                    </span>
                  )}
                  {video.status === "ready" && video.viewCount === 0 && (
                    <span style={{ color: "var(--color-text-secondary)", marginLeft: 8, opacity: 0.6 }}>
                      &middot; No views yet
                    </span>
                  )}
                  {video.status === "ready" && (() => {
                    const expiry = expiryLabel(video.shareExpiresAt);
                    return (
                      <span style={{ color: video.shareExpiresAt === null ? "var(--color-accent)" : expiry.expired ? "var(--color-error)" : "var(--color-text-secondary)", marginLeft: 8 }}>
                        &middot; {expiry.text}
                      </span>
                    );
                  })()}
                  {video.status === "ready" && video.transcriptStatus === "pending" && (
                    <span style={{ color: "var(--color-text-secondary)", marginLeft: 8 }}>
                      &middot; Pending transcription...
                    </span>
                  )}
                  {video.status === "ready" && video.transcriptStatus === "processing" && (
                    <span style={{ color: "var(--color-accent)", marginLeft: 8 }}>
                      &middot; Transcribing...
                    </span>
                  )}
                  {video.status === "ready" && video.transcriptStatus !== "processing" && video.transcriptStatus !== "pending" && (
                    <span style={{ marginLeft: 8 }}>
                      &middot;{" "}
                      <button
                        onClick={() => retranscribeVideo(video.id)}
                        className="action-link"
                        style={{ color: video.transcriptStatus === "failed" ? "var(--color-error)" : undefined }}
                      >
                        {video.transcriptStatus === "failed" ? "Retry transcript" : video.transcriptStatus === "none" ? "Transcribe" : "Redo transcript"}
                      </button>
                    </span>
                  )}
                </p>
              </div>
            </div>

            {video.status === "ready" && (
              <div className="video-card-actions" style={{ borderTop: "1px solid var(--color-border)", marginTop: 12, paddingTop: 10 }}>
                <div style={{ display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
                  <a href={`/watch/${video.shareToken}`} className="action-link">View</a>
                  <span className="action-sep">&middot;</span>
                  <Link to={`/videos/${video.id}/analytics`} className="action-link">Analytics</Link>
                  <span className="action-sep">&middot;</span>
                  <button
                    onClick={() => copyLink(video.shareUrl, video.id)}
                    className="action-link"
                    style={{ color: copiedId === video.id ? "var(--color-accent)" : undefined }}
                  >
                    {copiedId === video.id ? "Copied!" : "Copy link"}
                  </button>
                  <span className="action-sep">&middot;</span>
                  <button
                    onClick={() => downloadVideo(video.id)}
                    disabled={downloadingId === video.id}
                    className="action-link"
                    style={{ opacity: downloadingId === video.id ? 0.5 : undefined }}
                  >
                    {downloadingId === video.id ? "Downloading..." : "Download"}
                  </button>
                  <span className="action-sep">&middot;</span>
                  <div style={{ position: "relative" }} ref={openMenuId === video.id ? menuRef : undefined}>
                    <button
                      onClick={() => setOpenMenuId(openMenuId === video.id ? null : video.id)}
                      className="action-link"
                      aria-label="More actions"
                      aria-expanded={openMenuId === video.id}
                    >
                      &middot;&middot;&middot;
                    </button>
                    {openMenuId === video.id && (
                      <div
                        style={{
                          position: "absolute",
                          top: "100%",
                          right: 0,
                          marginTop: 4,
                          background: "var(--color-surface)",
                          border: "1px solid var(--color-border)",
                          borderRadius: 8,
                          padding: "8px 0",
                          minWidth: 220,
                          zIndex: 50,
                          boxShadow: "0 4px 16px rgba(0,0,0,0.3)",
                        }}
                      >
                        <div style={{ padding: "4px 12px", fontSize: 11, color: "var(--color-text-secondary)", textTransform: "uppercase", letterSpacing: "0.05em" }}>
                          Sharing
                        </div>
                        <button
                          onClick={() => { toggleDownload(video); setOpenMenuId(null); }}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px", color: video.downloadEnabled ? "var(--color-accent)" : undefined }}
                        >
                          {video.downloadEnabled ? "Downloads on" : "Downloads off"}
                        </button>
                        <button
                          onClick={() => {
                            const snippet = `<iframe src="${window.location.origin}/embed/${video.shareToken}" width="640" height="360" frameborder="0" allowfullscreen></iframe>`;
                            navigator.clipboard.writeText(snippet);
                            setCopiedEmbedId(video.id);
                            setTimeout(() => setCopiedEmbedId(null), 2000);
                          }}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px", color: copiedEmbedId === video.id ? "var(--color-accent)" : undefined }}
                        >
                          {copiedEmbedId === video.id ? "Copied!" : "Embed"}
                        </button>
                        <button
                          onClick={() => { toggleLinkExpiry(video); setOpenMenuId(null); }}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px" }}
                        >
                          {video.shareExpiresAt === null ? "Set expiry" : "Remove expiry"}
                        </button>
                        {video.shareExpiresAt !== null && (
                          <button
                            onClick={() => { extendVideo(video.id); setOpenMenuId(null); }}
                            disabled={extendingId === video.id}
                            className="action-link"
                            style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px", opacity: extendingId === video.id ? 0.5 : undefined }}
                          >
                            {extendingId === video.id ? "Extending..." : "Extend"}
                          </button>
                        )}
                        <button
                          onClick={() => { video.hasPassword ? removePassword(video.id) : addPassword(video.id); setOpenMenuId(null); }}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px" }}
                        >
                          {video.hasPassword ? "Remove password" : "Add password"}
                        </button>
                        <button
                          onClick={() => { cycleCommentMode(video); setOpenMenuId(null); }}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px", color: video.commentMode !== "disabled" ? "var(--color-accent)" : undefined }}
                        >
                          {commentModeLabels[video.commentMode] ?? "Comments off"}
                          {video.commentCount > 0 && ` (${video.commentCount})`}
                        </button>

                        <div style={{ borderTop: "1px solid var(--color-border)", margin: "4px 0" }} />
                        <div style={{ padding: "4px 12px", fontSize: 11, color: "var(--color-text-secondary)", textTransform: "uppercase", letterSpacing: "0.05em" }}>
                          Customization
                        </div>
                        <label style={{ display: "block", padding: "6px 12px" }}>
                          <span
                            role="button"
                            tabIndex={0}
                            className="action-link"
                            style={{ cursor: uploadingThumbnailId === video.id ? "default" : "pointer" }}
                            onKeyDown={(e) => {
                              if (e.key === "Enter" || e.key === " ") {
                                const input = (e.currentTarget.parentElement as HTMLLabelElement).querySelector("input");
                                if (input) input.click();
                              }
                            }}
                          >
                            {uploadingThumbnailId === video.id ? "Uploading..." : "Thumbnail"}
                          </span>
                          <input
                            type="file"
                            accept="image/jpeg,image/png,image/webp"
                            style={{ display: "none" }}
                            disabled={uploadingThumbnailId === video.id}
                            onChange={(e) => {
                              const file = e.target.files?.[0];
                              if (file) uploadThumbnail(video.id, file);
                              e.target.value = "";
                              setOpenMenuId(null);
                            }}
                          />
                        </label>
                        {video.thumbnailUrl && (
                          <button
                            onClick={() => { resetThumbnail(video.id); setOpenMenuId(null); }}
                            disabled={uploadingThumbnailId === video.id}
                            className="action-link"
                            style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px" }}
                          >
                            Reset thumbnail
                          </button>
                        )}
                        {limits?.brandingEnabled && (
                          <button
                            onClick={() => { openBranding(video.id); setOpenMenuId(null); }}
                            className="action-link"
                            style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px" }}
                          >
                            Branding
                          </button>
                        )}
                        <div style={{ padding: "6px 12px" }}>
                          <label style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 13 }}>
                            <span className="action-link" style={{ cursor: "default" }}>Notifications</span>
                            <select
                              aria-label="View notifications"
                              value={video.viewNotification ?? ""}
                              onChange={(e) => { changeNotification(video, e.target.value); setOpenMenuId(null); }}
                              style={{
                                background: "var(--color-surface)",
                                border: "1px solid var(--color-border)",
                                borderRadius: 4,
                                color: "var(--color-text-secondary)",
                                fontSize: 12,
                                padding: "2px 4px",
                                cursor: "pointer",
                              }}
                            >
                              <option value="">Account default</option>
                              <option value="off">Off</option>
                              <option value="every">Every view</option>
                              <option value="first">First view only</option>
                              <option value="digest">Daily digest</option>
                            </select>
                          </label>
                        </div>

                        <div style={{ borderTop: "1px solid var(--color-border)", margin: "4px 0" }} />
                        <div style={{ padding: "4px 12px", fontSize: 11, color: "var(--color-text-secondary)", textTransform: "uppercase", letterSpacing: "0.05em" }}>
                          Editing
                        </div>
                        <button
                          onClick={() => { setTrimmingId(video.id); setOpenMenuId(null); }}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px" }}
                        >
                          Trim
                        </button>
                      </div>
                    )}
                  </div>
                  <span style={{ flex: 1 }} />
                  <button
                    onClick={() => deleteVideo(video.id)}
                    disabled={deletingId === video.id}
                    className="action-link"
                    style={{ color: "var(--color-error)", opacity: deletingId === video.id ? 0.5 : undefined }}
                  >
                    {deletingId === video.id ? "Deleting..." : "Delete"}
                  </button>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>

      {brandingVideoId && (
        <div
          style={{
            position: "fixed", inset: 0, background: "rgba(0,0,0,0.6)", display: "flex",
            alignItems: "center", justifyContent: "center", zIndex: 100,
          }}
          onClick={() => setBrandingVideoId(null)}
        >
          <div
            onClick={(e) => e.stopPropagation()}
            style={{
              background: "var(--color-surface)", borderRadius: 12, padding: 24,
              width: "calc(100vw - 32px)", maxWidth: 400, maxHeight: "80vh", overflow: "auto",
              border: "1px solid var(--color-border)",
            }}
          >
            <h3 style={{ color: "var(--color-text)", fontSize: 18, margin: "0 0 16px" }}>Video Branding</h3>
            <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 16px" }}>
              Override your account branding for this video. Leave empty to inherit.
            </p>

            <label style={{ display: "flex", flexDirection: "column", gap: 4, marginBottom: 12 }}>
              <span style={{ color: "var(--color-text-secondary)", fontSize: 13 }}>Company name</span>
              <input
                type="text"
                value={videoBranding.companyName ?? ""}
                onChange={(e) => setVideoBranding({ ...videoBranding, companyName: e.target.value || null })}
                placeholder="Inherit from account"
                maxLength={200}
                style={{
                  background: "var(--color-bg)", border: "1px solid var(--color-border)",
                  borderRadius: 4, color: "var(--color-text)", padding: "8px 12px", fontSize: 14, width: "100%",
                }}
              />
            </label>

            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8, marginBottom: 12 }}>
              {(["colorBackground", "colorSurface", "colorText", "colorAccent"] as const).map((key) => {
                const labels: Record<string, string> = {
                  colorBackground: "Background", colorSurface: "Surface", colorText: "Text", colorAccent: "Accent",
                };
                return (
                  <label key={key} style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                    <span style={{ color: "var(--color-text-secondary)", fontSize: 13 }}>{labels[key]}</span>
                    <input
                      type="text"
                      value={videoBranding[key] ?? ""}
                      onChange={(e) => setVideoBranding({ ...videoBranding, [key]: e.target.value || null })}
                      placeholder="Inherit"
                      style={{
                        background: "var(--color-bg)", border: "1px solid var(--color-border)",
                        borderRadius: 4, color: "var(--color-text)", padding: "6px 10px", fontSize: 13, width: "100%",
                      }}
                    />
                  </label>
                );
              })}
            </div>

            <label style={{ display: "flex", flexDirection: "column", gap: 4, marginBottom: 16 }}>
              <span style={{ color: "var(--color-text-secondary)", fontSize: 13 }}>Footer text</span>
              <input
                type="text"
                value={videoBranding.footerText ?? ""}
                onChange={(e) => setVideoBranding({ ...videoBranding, footerText: e.target.value || null })}
                placeholder="Inherit from account"
                maxLength={500}
                style={{
                  background: "var(--color-bg)", border: "1px solid var(--color-border)",
                  borderRadius: 4, color: "var(--color-text)", padding: "8px 12px", fontSize: 14, width: "100%",
                }}
              />
            </label>

            {brandingMessage && (
              <p style={{ color: brandingMessage === "Saved" ? "var(--color-accent)" : "var(--color-error)", fontSize: 13, margin: "0 0 12px" }}>
                {brandingMessage}
              </p>
            )}

            <div style={{ display: "flex", gap: 8 }}>
              <button
                onClick={saveBranding}
                disabled={savingBranding}
                style={{
                  background: "var(--color-accent)", color: "var(--color-text)",
                  borderRadius: 4, padding: "8px 16px", fontSize: 14, fontWeight: 600,
                  opacity: savingBranding ? 0.7 : 1, border: "none", cursor: "pointer",
                }}
              >
                {savingBranding ? "Saving..." : "Save"}
              </button>
              <button
                onClick={() => setBrandingVideoId(null)}
                style={{
                  background: "transparent", color: "var(--color-text-secondary)",
                  border: "1px solid var(--color-border)", borderRadius: 4,
                  padding: "8px 16px", fontSize: 14, cursor: "pointer",
                }}
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {trimmingId && (() => {
        const video = videos.find((v) => v.id === trimmingId);
        if (!video) return null;
        return (
          <TrimModal
            videoId={video.id}
            duration={video.duration}
            onClose={() => setTrimmingId(null)}
            onTrimStarted={() => {
              setVideos((prev) =>
                prev.map((v) => (v.id === trimmingId ? { ...v, status: "processing" } : v))
              );
              setTrimmingId(null);
            }}
          />
        );
      })()}
    </div>
  );
}
