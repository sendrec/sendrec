import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { apiFetch } from "../api/client";

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
  const [limits, setLimits] = useState<LimitsResponse | null>(null);

  useEffect(() => {
    async function fetchData() {
      try {
        const [videosResult, limitsResult] = await Promise.all([
          apiFetch<Video[]>("/api/videos"),
          apiFetch<LimitsResponse>("/api/videos/limits"),
        ]);
        setVideos(videosResult ?? []);
        setLimits(limitsResult ?? null);
      } catch {
        setVideos([]);
      } finally {
        setLoading(false);
      }
    }

    fetchData();
  }, []);

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
              justifyContent: "space-between",
            }}
          >
            {video.thumbnailUrl && (
              <img
                src={video.thumbnailUrl}
                alt=""
                style={{
                  width: 120,
                  height: 68,
                  objectFit: "cover",
                  borderRadius: 4,
                  flexShrink: 0,
                  background: "var(--color-border)",
                }}
              />
            )}
            <div style={{ minWidth: 0, flex: 1 }}>
              <a
                href={`/watch/${video.shareToken}`}
                target="_blank"
                rel="noopener noreferrer"
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
                }}
              >
                {video.title}
              </a>
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
              </p>
            </div>

            <div className="video-card-actions">
              {video.status === "ready" && (
                <>
                  <button
                    onClick={() => copyLink(video.shareUrl, video.id)}
                    style={{
                      background: "var(--color-accent)",
                      color: "var(--color-text)",
                      borderRadius: 4,
                      padding: "6px 14px",
                      fontSize: 13,
                      fontWeight: 600,
                    }}
                  >
                    {copiedId === video.id ? "Copied!" : "Copy link"}
                  </button>
                  <button
                    onClick={() => extendVideo(video.id)}
                    disabled={extendingId === video.id}
                    style={{
                      background: "transparent",
                      color: "var(--color-accent)",
                      border: "1px solid var(--color-accent)",
                      borderRadius: 4,
                      padding: "6px 14px",
                      fontSize: 13,
                      fontWeight: 600,
                      opacity: extendingId === video.id ? 0.7 : 1,
                    }}
                  >
                    {extendingId === video.id ? "Extending..." : "Extend"}
                  </button>
                </>
              )}

              <button
                onClick={() => deleteVideo(video.id)}
                disabled={deletingId === video.id}
                style={{
                  background: "transparent",
                  color: "var(--color-error)",
                  border: "1px solid var(--color-error)",
                  borderRadius: 4,
                  padding: "6px 14px",
                  fontSize: 13,
                  fontWeight: 600,
                  opacity: deletingId === video.id ? 0.7 : 1,
                }}
              >
                {deletingId === video.id ? "Deleting..." : "Delete"}
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
