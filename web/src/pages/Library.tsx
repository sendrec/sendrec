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
  emailGateEnabled: boolean;
  ctaText: string | null;
  ctaUrl: string | null;
  summaryStatus: string;
}

interface LimitsResponse {
  maxVideosPerMonth: number;
  maxVideoDurationSeconds: number;
  videosUsedThisMonth: number;
  brandingEnabled: boolean;
  aiEnabled: boolean;
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
  const [toast, setToast] = useState<string | null>(null);
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
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
  const [ctaVideoId, setCtaVideoId] = useState<string | null>(null);
  const [ctaText, setCtaText] = useState("");
  const [ctaUrl, setCtaUrl] = useState("");

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

  function showToast(message: string) {
    if (toastTimer.current) clearTimeout(toastTimer.current);
    setToast(message);
    toastTimer.current = setTimeout(() => setToast(null), 2000);
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

  async function copyLink(shareUrl: string) {
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
    showToast("Link copied");
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

  async function toggleEmailGate(video: Video) {
    const newValue = !video.emailGateEnabled;
    await apiFetch(`/api/videos/${video.id}/email-gate`, {
      method: "PUT",
      body: JSON.stringify({ enabled: newValue }),
    });
    setVideos((prev) => prev.map((v) => (v.id === video.id ? { ...v, emailGateEnabled: newValue } : v)));
  }

  async function summarizeVideo(video: Video) {
    await apiFetch(`/api/videos/${video.id}/summarize`, { method: "POST" });
    setVideos((prev) => prev.map((v) => (v.id === video.id ? { ...v, summaryStatus: "pending" } : v)));
    showToast("Summary queued");
    setOpenMenuId(null);
  }

  async function extendVideo(id: string) {
    setExtendingId(id);
    try {
      await apiFetch(`/api/videos/${id}/extend`, { method: "POST" });
      const result = await apiFetch<Video[]>("/api/videos");
      setVideos(result ?? []);
      showToast("Link extended");
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
    showToast("Transcription queued");
  }

  async function changeCommentMode(video: Video, mode: string) {
    try {
      await apiFetch(`/api/videos/${video.id}/comment-mode`, {
        method: "PUT",
        body: JSON.stringify({ commentMode: mode }),
      });
      setVideos((prev) => prev.map((v) => (v.id === video.id ? { ...v, commentMode: mode } : v)));
    } catch {
      // no-op: select stays at previous value since state is only updated on success
    }
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
      showToast("Thumbnail updated");
    } finally {
      setUploadingThumbnailId(null);
    }
  }

  async function resetThumbnail(videoId: string) {
    setUploadingThumbnailId(videoId);
    try {
      await apiFetch(`/api/videos/${videoId}/thumbnail`, { method: "DELETE" });
      await fetchVideosAndLimits(searchQuery);
      showToast("Thumbnail reset");
    } finally {
      setUploadingThumbnailId(null);
    }
  }

  async function saveCTA(video: Video) {
    await apiFetch(`/api/videos/${video.id}/cta`, {
      method: "PUT",
      body: JSON.stringify({ text: ctaText, url: ctaUrl }),
    });
    setVideos((prev) => prev.map((v) => (v.id === video.id ? { ...v, ctaText: ctaText, ctaUrl: ctaUrl } : v)));
    setCtaVideoId(null);
    showToast("CTA saved");
  }

  async function clearCTA(video: Video) {
    await apiFetch(`/api/videos/${video.id}/cta`, {
      method: "PUT",
      body: JSON.stringify({ text: null, url: null }),
    });
    setVideos((prev) => prev.map((v) => (v.id === video.id ? { ...v, ctaText: null, ctaUrl: null } : v)));
    setCtaVideoId(null);
    showToast("CTA removed");
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
                </p>
                <div style={{ display: "flex", alignItems: "center", gap: 6, marginTop: 4, fontSize: 13 }}>
                  {video.transcriptStatus === "pending" && (
                    <span style={{ color: "var(--color-text-secondary)" }}>Pending transcription...</span>
                  )}
                  {video.transcriptStatus === "processing" && (
                    <span style={{ color: "var(--color-accent)" }}>Transcribing...</span>
                  )}
                  {video.transcriptStatus !== "processing" && video.transcriptStatus !== "pending" && (
                    <button
                      onClick={() => retranscribeVideo(video.id)}
                      className="action-link"
                      style={{ color: video.transcriptStatus === "failed" ? "var(--color-error)" : undefined }}
                    >
                      {video.transcriptStatus === "failed" ? "Retry transcript" : video.transcriptStatus === "none" ? "Transcribe" : "Redo transcript"}
                    </button>
                  )}
                  {limits?.aiEnabled && video.transcriptStatus === "ready" && (
                    <>
                      <span className="action-sep">&middot;</span>
                      <button
                        onClick={() => summarizeVideo(video)}
                        disabled={video.summaryStatus === "pending" || video.summaryStatus === "processing"}
                        className="action-link"
                        style={{ opacity: video.summaryStatus === "pending" || video.summaryStatus === "processing" ? 0.5 : undefined }}
                      >
                        {video.summaryStatus === "pending" || video.summaryStatus === "processing"
                          ? "Summarizing..."
                          : video.summaryStatus === "ready" ? "Re-summarize" : "Summarize"}
                      </button>
                    </>
                  )}
                </div>
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
                    onClick={() => copyLink(video.shareUrl)}
                    className="action-link"
                  >
                    Copy link
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
                          boxShadow: "0 4px 16px var(--color-shadow)",
                        }}
                      >
                        <div style={{ padding: "4px 12px", fontSize: 11, color: "var(--color-text-secondary)", textTransform: "uppercase", letterSpacing: "0.05em" }}>
                          Sharing
                        </div>
                        <button
                          onClick={() => toggleDownload(video)}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px", color: video.downloadEnabled ? "var(--color-accent)" : undefined }}
                        >
                          {video.downloadEnabled ? "Downloads on" : "Downloads off"}
                        </button>
                        <button
                          onClick={() => toggleEmailGate(video)}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px", color: video.emailGateEnabled ? "var(--color-accent)" : undefined }}
                        >
                          {video.emailGateEnabled ? "Email required" : "Require email"}
                        </button>
                        <button
                          onClick={() => {
                            const snippet = `<iframe src="${window.location.origin}/embed/${video.shareToken}" width="640" height="360" frameborder="0" allowfullscreen></iframe>`;
                            navigator.clipboard.writeText(snippet);
                            showToast("Embed code copied");
                            setOpenMenuId(null);
                          }}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px" }}
                        >
                          Embed
                        </button>
                        <button
                          onClick={() => {
                            setCtaVideoId(ctaVideoId === video.id ? null : video.id);
                            setCtaText(video.ctaText ?? "");
                            setCtaUrl(video.ctaUrl ?? "");
                            setOpenMenuId(null);
                          }}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px", color: video.ctaText ? "var(--color-accent)" : undefined }}
                        >
                          {video.ctaText ? "Edit CTA" : "Call to action"}
                        </button>
                        <button
                          onClick={() => toggleLinkExpiry(video)}
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
                          onClick={() => { video.hasPassword ? removePassword(video.id) : addPassword(video.id); }}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px" }}
                        >
                          {video.hasPassword ? "Remove password" : "Add password"}
                        </button>
                        <div style={{ padding: "6px 12px" }}>
                          <label style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 13 }}>
                            <span className="action-link" style={{ cursor: "default" }}>
                              Comments{video.commentCount > 0 && ` (${video.commentCount})`}
                            </span>
                            <select
                              aria-label="Comment mode"
                              value={video.commentMode}
                              onChange={(e) => changeCommentMode(video, e.target.value)}
                              style={{
                                background: "var(--color-surface)",
                                border: "1px solid var(--color-border)",
                                borderRadius: 4,
                                color: video.commentMode !== "disabled" ? "var(--color-accent)" : "var(--color-text-secondary)",
                                fontSize: 12,
                                padding: "2px 4px",
                                cursor: "pointer",
                              }}
                            >
                              <option value="disabled">Off</option>
                              <option value="anonymous">Anonymous</option>
                              <option value="name_required">Name required</option>
                              <option value="name_email_required">Name + email</option>
                            </select>
                          </label>
                        </div>

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
                              onChange={(e) => changeNotification(video, e.target.value)}
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

                {ctaVideoId === video.id && (
                  <div style={{ marginTop: 12, padding: 12, background: "var(--color-surface)", borderRadius: 8, border: "1px solid var(--color-border)" }}>
                    <p style={{ fontSize: 13, color: "var(--color-text-secondary)", margin: "0 0 8px" }}>Call to action</p>
                    <input
                      type="text"
                      placeholder="Button text (e.g. Book a demo)"
                      value={ctaText}
                      onChange={(e) => setCtaText(e.target.value)}
                      maxLength={100}
                      style={{
                        width: "100%", padding: "8px 10px", marginBottom: 8,
                        background: "var(--color-background)", border: "1px solid var(--color-border)",
                        borderRadius: 6, color: "var(--color-text)", fontSize: 13,
                      }}
                    />
                    <input
                      type="url"
                      placeholder="URL (e.g. https://example.com/demo)"
                      value={ctaUrl}
                      onChange={(e) => setCtaUrl(e.target.value)}
                      maxLength={2000}
                      style={{
                        width: "100%", padding: "8px 10px", marginBottom: 8,
                        background: "var(--color-background)", border: "1px solid var(--color-border)",
                        borderRadius: 6, color: "var(--color-text)", fontSize: 13,
                      }}
                    />
                    <div style={{ display: "flex", gap: 8 }}>
                      <button
                        onClick={() => saveCTA(video)}
                        disabled={!ctaText.trim() || !ctaUrl.trim()}
                        style={{
                          padding: "6px 16px", background: "var(--color-accent)", color: "var(--color-on-accent)",
                          border: "none", borderRadius: 6, fontSize: 13, fontWeight: 600,
                          cursor: "pointer", opacity: (!ctaText.trim() || !ctaUrl.trim()) ? 0.5 : 1,
                        }}
                      >
                        Save
                      </button>
                      {video.ctaText && (
                        <button
                          onClick={() => clearCTA(video)}
                          style={{
                            padding: "6px 16px", background: "transparent", color: "var(--color-error)",
                            border: "1px solid var(--color-error)", borderRadius: 6, fontSize: 13,
                            cursor: "pointer",
                          }}
                        >
                          Remove
                        </button>
                      )}
                      <button
                        onClick={() => setCtaVideoId(null)}
                        style={{
                          padding: "6px 16px", background: "transparent", color: "var(--color-text-secondary)",
                          border: "1px solid var(--color-border)", borderRadius: 6, fontSize: 13,
                          cursor: "pointer",
                        }}
                      >
                        Cancel
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
            position: "fixed", inset: 0, background: "var(--color-overlay)", display: "flex",
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

      {toast && (
        <div
          role="status"
          aria-live="polite"
          style={{
            position: "fixed",
            bottom: 24,
            left: "50%",
            transform: "translateX(-50%)",
            background: "var(--color-surface)",
            color: "var(--color-text)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: "10px 20px",
            fontSize: 14,
            fontWeight: 500,
            zIndex: 200,
            boxShadow: "0 4px 16px var(--color-shadow)",
            pointerEvents: "none",
          }}
        >
          {toast}
        </div>
      )}
    </div>
  );
}
