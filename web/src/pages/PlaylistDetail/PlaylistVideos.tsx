import { useState } from "react";
import { Link } from "react-router-dom";
import { apiFetch } from "../../api/client";
import { formatDuration } from "../../utils/format";

export interface PlaylistVideo {
  id: string;
  title: string;
  duration: number;
  shareToken: string;
  shareUrl: string;
  status: string;
  position: number;
  thumbnailUrl?: string;
  createdAt: string;
}

interface LibraryVideo {
  id: string;
  title: string;
  duration: number;
  shareToken: string;
  status: string;
  createdAt: string;
}

interface PlaylistVideosProps {
  playlistId: string;
  videos: PlaylistVideo[];
  onVideosChanged: (videos: PlaylistVideo[]) => void;
  onPlaylistRefresh: () => Promise<void>;
  showToast: (message: string) => void;
}

export function PlaylistVideos({
  playlistId,
  videos,
  onVideosChanged,
  onPlaylistRefresh,
  showToast,
}: PlaylistVideosProps) {
  const [showAddVideos, setShowAddVideos] = useState(false);
  const [libraryVideos, setLibraryVideos] = useState<LibraryVideo[]>([]);
  const [selectedVideoIds, setSelectedVideoIds] = useState<Set<string>>(
    new Set(),
  );
  const [addingVideos, setAddingVideos] = useState(false);
  const [videoSearch, setVideoSearch] = useState("");

  async function removeVideo(videoId: string) {
    await apiFetch(`/api/playlists/${playlistId}/videos/${videoId}`, {
      method: "DELETE",
    });
    const filtered = videos.filter((v) => v.id !== videoId);
    onVideosChanged(filtered);
  }

  async function moveVideo(videoId: string, direction: "up" | "down") {
    const reordered = [...videos];
    const index = reordered.findIndex((v) => v.id === videoId);
    if (index < 0) return;
    const targetIndex = direction === "up" ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= reordered.length) return;

    [reordered[index], reordered[targetIndex]] = [
      reordered[targetIndex],
      reordered[index],
    ];

    const repositioned = reordered.map((v, i) => ({ ...v, position: i }));
    onVideosChanged(repositioned);

    await apiFetch(`/api/playlists/${playlistId}/videos/reorder`, {
      method: "PATCH",
      body: JSON.stringify({
        items: reordered.map((v, i) => ({ videoId: v.id, position: i })),
      }),
    });
  }

  async function openAddVideos() {
    setShowAddVideos(true);
    setSelectedVideoIds(new Set());
    setVideoSearch("");
    try {
      const fetchedVideos = await apiFetch<LibraryVideo[]>("/api/videos");
      setLibraryVideos(fetchedVideos ?? []);
    } catch {
      setLibraryVideos([]);
    }
  }

  async function addSelectedVideos() {
    if (selectedVideoIds.size === 0) return;
    setAddingVideos(true);
    try {
      await apiFetch(`/api/playlists/${playlistId}/videos`, {
        method: "POST",
        body: JSON.stringify({ videoIds: Array.from(selectedVideoIds) }),
      });
      setShowAddVideos(false);
      setSelectedVideoIds(new Set());
      await onPlaylistRefresh();
      showToast(`Added ${selectedVideoIds.size} video(s)`);
    } catch {
      showToast("Failed to add videos");
    } finally {
      setAddingVideos(false);
    }
  }

  function toggleVideoSelection(videoId: string) {
    setSelectedVideoIds((prev) => {
      const next = new Set(prev);
      if (next.has(videoId)) next.delete(videoId);
      else next.add(videoId);
      return next;
    });
  }

  const existingVideoIds = new Set(videos.map((v) => v.id));
  const availableVideos = libraryVideos
    .filter((v) => !existingVideoIds.has(v.id) && v.status === "ready")
    .filter(
      (v) =>
        !videoSearch ||
        v.title.toLowerCase().includes(videoSearch.toLowerCase()),
    );

  return (
    <>
      <div className="video-detail-section">
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            marginBottom: 16,
          }}
        >
          <h2 className="video-detail-section-title" style={{ margin: 0 }}>
            Videos
          </h2>
          <button className="detail-btn" onClick={openAddVideos}>
            Add Videos
          </button>
        </div>

        {videos.length === 0 ? (
          <p
            style={{
              color: "var(--color-text-secondary)",
              fontSize: 14,
              textAlign: "center",
              padding: "24px 0",
            }}
          >
            No videos in this playlist yet
          </p>
        ) : (
          <div
            style={{ display: "flex", flexDirection: "column", gap: 8 }}
          >
            {videos.map((video, index) => (
              <div
                key={video.id}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 12,
                  padding: "10px 12px",
                  background: "var(--color-surface)",
                  border: "1px solid var(--color-border)",
                  borderRadius: 8,
                }}
              >
                <span
                  style={{
                    color: "var(--color-text-secondary)",
                    fontSize: 13,
                    fontWeight: 600,
                    minWidth: 20,
                    textAlign: "center",
                  }}
                >
                  {index + 1}
                </span>

                {video.thumbnailUrl && (
                  <img
                    src={video.thumbnailUrl}
                    alt=""
                    style={{
                      width: 80,
                      height: 45,
                      objectFit: "cover",
                      borderRadius: 4,
                      background: "var(--color-border)",
                      flexShrink: 0,
                    }}
                  />
                )}

                <div style={{ flex: 1, minWidth: 0 }}>
                  <Link
                    to={`/videos/${video.id}`}
                    style={{
                      color: "var(--color-text)",
                      fontSize: 14,
                      fontWeight: 600,
                      textDecoration: "none",
                      display: "block",
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {video.title}
                  </Link>
                  <span
                    style={{
                      color: "var(--color-text-secondary)",
                      fontSize: 12,
                    }}
                  >
                    {formatDuration(video.duration)}
                  </span>
                </div>

                <div
                  style={{ display: "flex", alignItems: "center", gap: 4 }}
                >
                  <button
                    onClick={() => moveVideo(video.id, "up")}
                    disabled={index === 0}
                    aria-label={`Move ${video.title} up`}
                    style={{
                      background: "none",
                      border: "none",
                      color:
                        index === 0
                          ? "var(--color-border)"
                          : "var(--color-text-secondary)",
                      cursor: index === 0 ? "default" : "pointer",
                      fontSize: 16,
                      padding: "2px 6px",
                    }}
                  >
                    &uarr;
                  </button>
                  <button
                    onClick={() => moveVideo(video.id, "down")}
                    disabled={index === videos.length - 1}
                    aria-label={`Move ${video.title} down`}
                    style={{
                      background: "none",
                      border: "none",
                      color:
                        index === videos.length - 1
                          ? "var(--color-border)"
                          : "var(--color-text-secondary)",
                      cursor:
                        index === videos.length - 1
                          ? "default"
                          : "pointer",
                      fontSize: 16,
                      padding: "2px 6px",
                    }}
                  >
                    &darr;
                  </button>
                  <button
                    onClick={() => removeVideo(video.id)}
                    aria-label={`Remove ${video.title}`}
                    className="detail-btn detail-btn--danger"
                    style={{ padding: "4px 10px", fontSize: 12 }}
                  >
                    Remove
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Add Videos Modal */}
      {showAddVideos && (
        <div
          style={{
            position: "fixed",
            inset: 0,
            background: "var(--color-overlay)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            zIndex: 100,
          }}
          onClick={() => setShowAddVideos(false)}
        >
          <div
            onClick={(e) => e.stopPropagation()}
            style={{
              background: "var(--color-surface)",
              borderRadius: 12,
              width: "calc(100vw - 32px)",
              maxWidth: 500,
              maxHeight: "80vh",
              border: "1px solid var(--color-border)",
              display: "flex",
              flexDirection: "column",
            }}
          >
            <div style={{ padding: "24px 24px 0" }}>
              <h3
                style={{
                  color: "var(--color-text)",
                  fontSize: 18,
                  margin: "0 0 16px",
                }}
              >
                Add Videos
              </h3>

              <input
                type="text"
                value={videoSearch}
                onChange={(e) => setVideoSearch(e.target.value)}
                placeholder="Search videos..."
                autoFocus
                style={{
                  width: "100%",
                  padding: "8px 12px",
                  fontSize: 14,
                  background: "var(--color-bg)",
                  border: "1px solid var(--color-border)",
                  borderRadius: 6,
                  color: "var(--color-text)",
                  marginBottom: 12,
                  boxSizing: "border-box",
                }}
              />
            </div>

            <div
              style={{
                flex: 1,
                overflowY: "auto",
                padding: "0 24px",
                minHeight: 0,
              }}
            >
              {availableVideos.length === 0 ? (
                <p
                  style={{
                    color: "var(--color-text-secondary)",
                    fontSize: 14,
                  }}
                >
                  No available videos to add
                </p>
              ) : (
                <div
                  style={{
                    display: "flex",
                    flexDirection: "column",
                    gap: 8,
                  }}
                >
                  {availableVideos.map((video) => (
                    <label
                      key={video.id}
                      style={{
                        display: "flex",
                        alignItems: "center",
                        gap: 10,
                        padding: "8px 10px",
                        borderRadius: 6,
                        cursor: "pointer",
                        background: selectedVideoIds.has(video.id)
                          ? "var(--color-bg)"
                          : "transparent",
                        border: "1px solid var(--color-border)",
                      }}
                    >
                      <input
                        type="checkbox"
                        checked={selectedVideoIds.has(video.id)}
                        onChange={() => toggleVideoSelection(video.id)}
                        style={{ accentColor: "var(--color-accent)" }}
                      />
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <span
                          style={{
                            color: "var(--color-text)",
                            fontSize: 14,
                            display: "block",
                            overflow: "hidden",
                            textOverflow: "ellipsis",
                            whiteSpace: "nowrap",
                          }}
                        >
                          {video.title}
                        </span>
                        <span
                          style={{
                            color: "var(--color-text-secondary)",
                            fontSize: 12,
                          }}
                        >
                          {formatDuration(video.duration)}
                        </span>
                      </div>
                    </label>
                  ))}
                </div>
              )}
            </div>

            <div
              style={{
                display: "flex",
                gap: 8,
                padding: "16px 24px",
                borderTop: "1px solid var(--color-border)",
              }}
            >
              <button
                onClick={addSelectedVideos}
                disabled={selectedVideoIds.size === 0 || addingVideos}
                className="detail-btn detail-btn--accent"
              >
                {addingVideos
                  ? "Adding..."
                  : `Add ${selectedVideoIds.size} video(s)`}
              </button>
              <button
                onClick={() => setShowAddVideos(false)}
                className="detail-btn"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
