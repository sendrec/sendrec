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
  shareExpiresAt: string;
  viewCount: number;
  uniqueViewCount: number;
  thumbnailUrl?: string;
  hasPassword: boolean;
  commentMode: string;
  commentCount: number;
  transcriptStatus: string;
  viewNotification: string | null;
}

interface LimitsResponse {
  maxVideosPerMonth: number;
  maxVideoDurationSeconds: number;
  videosUsedThisMonth: number;
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

function expiryLabel(shareExpiresAt: string): { text: string; expired: boolean } {
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
  const [downloadingId, setDownloadingId] = useState<string | null>(null);
  const [limits, setLimits] = useState<LimitsResponse | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editTitle, setEditTitle] = useState("");
  const [trimmingId, setTrimmingId] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

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
    await apiFetch(`/api/videos/${video.id}/notifications`, {
      method: "PUT",
      body: JSON.stringify({ viewNotification }),
    });
    setVideos((prev) => prev.map((v) => (v.id === video.id ? { ...v, viewNotification } : v)));
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
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 24 }}>
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
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: "8px 12px",
            fontSize: 14,
            color: "var(--color-text)",
            width: 220,
            outline: "none",
          }}
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
                      <span style={{ color: expiry.expired ? "var(--color-error)" : "var(--color-text-secondary)", marginLeft: 8 }}>
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
                  <a
                    href={`/watch/${video.shareToken}`}
                    className="action-link"
                  >
                    View
                  </a>
                  <span className="action-sep">&middot;</span>
                  <Link
                    to={`/videos/${video.id}/analytics`}
                    className="action-link"
                  >
                    Analytics
                  </Link>
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
                  <button
                    onClick={() => setTrimmingId(video.id)}
                    className="action-link"
                  >
                    Trim
                  </button>
                  <span className="action-sep">&middot;</span>
                  <button
                    onClick={() => extendVideo(video.id)}
                    disabled={extendingId === video.id}
                    className="action-link"
                    style={{ opacity: extendingId === video.id ? 0.5 : undefined }}
                  >
                    {extendingId === video.id ? "Extending..." : "Extend"}
                  </button>
                  <span className="action-sep">&middot;</span>
                  <button
                    onClick={() => cycleCommentMode(video)}
                    className="action-link"
                    style={{ color: video.commentMode !== "disabled" ? "var(--color-accent)" : undefined }}
                  >
                    {commentModeLabels[video.commentMode] ?? "Comments off"}
                    {video.commentCount > 0 && ` (${video.commentCount})`}
                  </button>
                  <span className="action-sep">&middot;</span>
                  <button
                    onClick={() => video.hasPassword ? removePassword(video.id) : addPassword(video.id)}
                    className="action-link"
                  >
                    {video.hasPassword ? "Remove password" : "Add password"}
                  </button>
                  <span className="action-sep">&middot;</span>
                  <label style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: 13 }}>
                    <span className="action-link" style={{ cursor: "default" }}>Notifications</span>
                    <select
                      aria-label="Notifications"
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
                      <option value="first">First view only</option>
                      <option value="digest">Daily digest</option>
                    </select>
                  </label>
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
