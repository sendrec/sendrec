import { useEffect, useState } from "react";
import { apiFetch } from "../api/client";

interface Video {
  id: string;
  title: string;
  status: string;
  duration: number;
  shareToken: string;
  shareUrl: string;
  createdAt: string;
}

function formatDuration(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = Math.floor(seconds % 60);
  return `${minutes}:${String(remainingSeconds).padStart(2, "0")}`;
}

function formatDate(isoDate: string): string {
  return new Date(isoDate).toLocaleDateString();
}

export function Library() {
  const [videos, setVideos] = useState<Video[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function fetchVideos() {
      try {
        const result = await apiFetch<Video[]>("/api/videos");
        setVideos(result ?? []);
      } catch {
        setVideos([]);
      } finally {
        setLoading(false);
      }
    }

    fetchVideos();
  }, []);

  async function deleteVideo(id: string) {
    await apiFetch(`/api/videos/${id}`, { method: "DELETE" });
    setVideos((prev) => prev.filter((v) => v.id !== id));
  }

  function copyLink(shareUrl: string) {
    navigator.clipboard.writeText(shareUrl);
  }

  if (loading) {
    return (
      <div style={{ maxWidth: 800, margin: "80px auto", padding: 24, textAlign: "center" }}>
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16 }}>Loading...</p>
      </div>
    );
  }

  if (videos.length === 0) {
    return (
      <div style={{ maxWidth: 800, margin: "80px auto", padding: 24, textAlign: "center" }}>
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16 }}>No recordings yet.</p>
      </div>
    );
  }

  return (
    <div style={{ maxWidth: 800, margin: "40px auto", padding: 24 }}>
      <h1
        style={{
          color: "var(--color-text)",
          fontSize: 24,
          marginBottom: 24,
        }}
      >
        Library
      </h1>

      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {videos.map((video) => (
          <div
            key={video.id}
            style={{
              background: "var(--color-surface)",
              border: "1px solid var(--color-border)",
              borderRadius: 8,
              padding: 16,
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 16,
            }}
          >
            <div style={{ minWidth: 0, flex: 1 }}>
              <p
                style={{
                  fontWeight: 600,
                  fontSize: 15,
                  color: "var(--color-text)",
                  margin: 0,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {video.title}
              </p>
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
              </p>
            </div>

            <div style={{ display: "flex", gap: 8, flexShrink: 0 }}>
              {video.status === "ready" && (
                <button
                  onClick={() => copyLink(video.shareUrl)}
                  style={{
                    background: "var(--color-accent)",
                    color: "var(--color-text)",
                    borderRadius: 4,
                    padding: "6px 14px",
                    fontSize: 13,
                    fontWeight: 600,
                  }}
                >
                  Copy link
                </button>
              )}

              <button
                onClick={() => deleteVideo(video.id)}
                style={{
                  background: "transparent",
                  color: "var(--color-error)",
                  border: "1px solid var(--color-error)",
                  borderRadius: 4,
                  padding: "6px 14px",
                  fontSize: 13,
                  fontWeight: 600,
                }}
              >
                Delete
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
