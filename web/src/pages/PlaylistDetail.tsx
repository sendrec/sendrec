import { useState, useEffect, useCallback, useRef } from "react";
import { useParams, useNavigate, Link } from "react-router-dom";
import { apiFetch } from "../api/client";

interface PlaylistVideo {
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

interface PlaylistData {
  id: string;
  title: string;
  description?: string;
  isShared: boolean;
  shareToken?: string;
  shareUrl?: string;
  requireEmail: boolean;
  position: number;
  videoCount: number;
  videos: PlaylistVideo[];
  createdAt: string;
  updatedAt: string;
}

interface LibraryVideo {
  id: string;
  title: string;
  duration: number;
  shareToken: string;
  status: string;
  createdAt: string;
}

function formatDuration(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = Math.floor(seconds % 60);
  return `${minutes}:${String(remainingSeconds).padStart(2, "0")}`;
}

export function PlaylistDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [playlist, setPlaylist] = useState<PlaylistData | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);

  const [editingTitle, setEditingTitle] = useState(false);
  const [editTitle, setEditTitle] = useState("");

  const [editingDescription, setEditingDescription] = useState(false);
  const [editDescription, setEditDescription] = useState("");

  const [showAddVideos, setShowAddVideos] = useState(false);
  const [libraryVideos, setLibraryVideos] = useState<LibraryVideo[]>([]);
  const [selectedVideoIds, setSelectedVideoIds] = useState<Set<string>>(
    new Set(),
  );
  const [addingVideos, setAddingVideos] = useState(false);

  const [videoSearch, setVideoSearch] = useState("");

  const [toast, setToast] = useState<string | null>(null);
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [passwordInput, setPasswordInput] = useState("");

  const fetchPlaylist = useCallback(async () => {
    try {
      const data = await apiFetch<PlaylistData>(`/api/playlists/${id}`);
      if (data) {
        setPlaylist(data);
      } else {
        setNotFound(true);
      }
    } catch {
      setNotFound(true);
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    fetchPlaylist();
  }, [fetchPlaylist]);

  function showToast(message: string) {
    if (toastTimer.current) clearTimeout(toastTimer.current);
    setToast(message);
    toastTimer.current = setTimeout(() => setToast(null), 2000);
  }

  async function copyToClipboard(text: string) {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      const textArea = document.createElement("textarea");
      textArea.value = text;
      textArea.style.position = "fixed";
      textArea.style.opacity = "0";
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand("copy");
      document.body.removeChild(textArea);
    }
  }

  async function saveTitle() {
    if (!playlist) return;
    if (!editTitle.trim() || editTitle === playlist.title) {
      setEditingTitle(false);
      return;
    }
    await apiFetch(`/api/playlists/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ title: editTitle.trim() }),
    });
    setPlaylist((prev) =>
      prev ? { ...prev, title: editTitle.trim() } : prev,
    );
    setEditingTitle(false);
  }

  async function saveDescription() {
    if (!playlist) return;
    await apiFetch(`/api/playlists/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ description: editDescription.trim() || null }),
    });
    setPlaylist((prev) =>
      prev
        ? { ...prev, description: editDescription.trim() || undefined }
        : prev,
    );
    setEditingDescription(false);
  }

  async function toggleSharing() {
    if (!playlist) return;
    const newValue = !playlist.isShared;
    await apiFetch(`/api/playlists/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ isShared: newValue }),
    });
    await fetchPlaylist();
    showToast(newValue ? "Sharing enabled" : "Sharing disabled");
  }

  async function toggleEmailGate() {
    if (!playlist) return;
    const newValue = !playlist.requireEmail;
    await apiFetch(`/api/playlists/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ requireEmail: newValue }),
    });
    setPlaylist((prev) =>
      prev ? { ...prev, requireEmail: newValue } : prev,
    );
  }

  async function setSharePassword() {
    if (!playlist || !passwordInput.trim()) return;
    await apiFetch(`/api/playlists/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ sharePassword: passwordInput }),
    });
    setPasswordInput("");
    showToast("Password set");
  }

  async function removeSharePassword() {
    if (!playlist) return;
    await apiFetch(`/api/playlists/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ sharePassword: "" }),
    });
    showToast("Password removed");
  }

  async function removeVideo(videoId: string) {
    if (!playlist) return;
    await apiFetch(`/api/playlists/${id}/videos/${videoId}`, {
      method: "DELETE",
    });
    setPlaylist((prev) => {
      if (!prev) return prev;
      const filtered = prev.videos.filter((v) => v.id !== videoId);
      return { ...prev, videos: filtered, videoCount: filtered.length };
    });
  }

  async function moveVideo(videoId: string, direction: "up" | "down") {
    if (!playlist) return;
    const videos = [...playlist.videos];
    const index = videos.findIndex((v) => v.id === videoId);
    if (index < 0) return;
    const targetIndex = direction === "up" ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= videos.length) return;

    [videos[index], videos[targetIndex]] = [
      videos[targetIndex],
      videos[index],
    ];
    const videoIds = videos.map((v) => v.id);

    setPlaylist((prev) =>
      prev
        ? {
            ...prev,
            videos: videos.map((v, i) => ({ ...v, position: i })),
          }
        : prev,
    );

    await apiFetch(`/api/playlists/${id}/videos/reorder`, {
      method: "PATCH",
      body: JSON.stringify({ videoIds }),
    });
  }

  async function openAddVideos() {
    setShowAddVideos(true);
    setSelectedVideoIds(new Set());
    setVideoSearch("");
    try {
      const videos = await apiFetch<LibraryVideo[]>("/api/videos");
      setLibraryVideos(videos ?? []);
    } catch {
      setLibraryVideos([]);
    }
  }

  async function addSelectedVideos() {
    if (!playlist || selectedVideoIds.size === 0) return;
    setAddingVideos(true);
    try {
      await apiFetch(`/api/playlists/${id}/videos`, {
        method: "POST",
        body: JSON.stringify({ videoIds: Array.from(selectedVideoIds) }),
      });
      setShowAddVideos(false);
      setSelectedVideoIds(new Set());
      await fetchPlaylist();
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

  async function deletePlaylist() {
    if (!playlist) return;
    if (
      !window.confirm(
        "Delete this playlist? Videos will not be deleted.",
      )
    )
      return;
    await apiFetch(`/api/playlists/${id}`, { method: "DELETE" });
    navigate("/playlists");
  }

  if (loading) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16 }}>
          Loading...
        </p>
      </div>
    );
  }

  if (notFound || !playlist) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16 }}>
          Playlist not found
        </p>
        <Link
          to="/playlists"
          style={{
            color: "var(--color-accent)",
            textDecoration: "none",
            fontSize: 14,
            marginTop: 8,
          }}
        >
          Back to Playlists
        </Link>
      </div>
    );
  }

  const existingVideoIds = new Set(playlist.videos.map((v) => v.id));
  const availableVideos = libraryVideos
    .filter((v) => !existingVideoIds.has(v.id) && v.status === "ready")
    .filter((v) => !videoSearch || v.title.toLowerCase().includes(videoSearch.toLowerCase()));

  return (
    <div className="page-container">
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          marginBottom: 24,
        }}
      >
        <Link
          to="/playlists"
          style={{
            color: "var(--color-text-secondary)",
            textDecoration: "none",
            fontSize: 14,
          }}
        >
          &larr; Playlists
        </Link>
        {playlist.isShared && playlist.shareUrl && (
          <a
            href={playlist.shareUrl}
            target="_blank"
            rel="noopener noreferrer"
            style={{
              color: "var(--color-accent)",
              textDecoration: "none",
              fontSize: 14,
            }}
          >
            View as viewer &rarr;
          </a>
        )}
      </div>

      {/* Title */}
      <div style={{ marginBottom: 8 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          {editingTitle ? (
            <input
              value={editTitle}
              onChange={(e) => setEditTitle(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") saveTitle();
                if (e.key === "Escape") setEditingTitle(false);
              }}
              onBlur={() => saveTitle()}
              autoFocus
              aria-label="Edit title"
              style={{
                color: "var(--color-text)",
                fontSize: 24,
                fontWeight: 700,
                background: "var(--color-surface)",
                border: "1px solid var(--color-accent)",
                borderRadius: 4,
                padding: "2px 6px",
                margin: 0,
                flex: 1,
                outline: "none",
              }}
            />
          ) : (
            <>
              <h1
                style={{
                  color: "var(--color-text)",
                  fontSize: 24,
                  margin: 0,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {playlist.title}
              </h1>
              <button
                onClick={() => {
                  setEditingTitle(true);
                  setEditTitle(playlist.title);
                }}
                aria-label="Edit title"
                style={{
                  background: "none",
                  border: "none",
                  cursor: "pointer",
                  padding: 4,
                  color: "var(--color-text-secondary)",
                  fontSize: 16,
                  flexShrink: 0,
                }}
              >
                &#9998;
              </button>
            </>
          )}
        </div>

        {/* Description */}
        {editingDescription ? (
          <div style={{ marginTop: 8, display: "flex", gap: 8 }}>
            <input
              value={editDescription}
              onChange={(e) => setEditDescription(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") saveDescription();
                if (e.key === "Escape") setEditingDescription(false);
              }}
              autoFocus
              placeholder="Add a description"
              aria-label="Edit description"
              style={{
                flex: 1,
                padding: "6px 10px",
                fontSize: 14,
                background: "var(--color-surface)",
                border: "1px solid var(--color-accent)",
                borderRadius: 4,
                color: "var(--color-text)",
                outline: "none",
              }}
            />
            <button onClick={saveDescription} className="detail-btn">
              Save
            </button>
          </div>
        ) : (
          <p
            style={{
              color: "var(--color-text-secondary)",
              fontSize: 14,
              marginTop: 4,
              cursor: "pointer",
            }}
            onClick={() => {
              setEditingDescription(true);
              setEditDescription(playlist.description ?? "");
            }}
          >
            {playlist.description || "Add a description..."}
          </p>
        )}

        <p
          style={{
            color: "var(--color-text-secondary)",
            fontSize: 13,
            marginTop: 4,
          }}
        >
          {playlist.videoCount}{" "}
          {playlist.videoCount === 1 ? "video" : "videos"}
          {playlist.isShared && (
            <span
              style={{
                marginLeft: 8,
                fontSize: 11,
                fontWeight: 600,
                padding: "1px 6px",
                borderRadius: 8,
                background: "var(--color-accent)",
                color: "var(--color-on-accent)",
              }}
            >
              Shared
            </span>
          )}
        </p>
      </div>

      {/* Videos Section */}
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

        {playlist.videos.length === 0 ? (
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
            {playlist.videos.map((video, index) => (
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
                    disabled={index === playlist.videos.length - 1}
                    aria-label={`Move ${video.title} down`}
                    style={{
                      background: "none",
                      border: "none",
                      color:
                        index === playlist.videos.length - 1
                          ? "var(--color-border)"
                          : "var(--color-text-secondary)",
                      cursor:
                        index === playlist.videos.length - 1
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

      {/* Sharing Section */}
      <div className="video-detail-section">
        <h2 className="video-detail-section-title">Sharing</h2>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Public link</span>
          <button
            onClick={toggleSharing}
            className={`detail-toggle${playlist.isShared ? " detail-toggle--active" : ""}`}
          >
            {playlist.isShared ? "Enabled" : "Disabled"}
          </button>
        </div>

        {playlist.isShared && playlist.shareUrl && (
          <>
            <div className="detail-setting-row">
              <span className="detail-setting-label">Share link</span>
              <div
                style={{ display: "flex", gap: 8, flex: 1, minWidth: 0 }}
              >
                <input
                  type="text"
                  readOnly
                  value={playlist.shareUrl}
                  aria-label="Share link"
                  style={{
                    flex: 1,
                    minWidth: 0,
                    padding: "6px 10px",
                    fontSize: 13,
                    background: "var(--color-bg)",
                    border: "1px solid var(--color-border)",
                    borderRadius: 4,
                    color: "var(--color-text)",
                  }}
                />
                <button
                  onClick={() => {
                    copyToClipboard(playlist.shareUrl!);
                    showToast("Link copied");
                  }}
                  className="detail-btn"
                >
                  Copy link
                </button>
              </div>
            </div>

            <div className="detail-setting-row">
              <span className="detail-setting-label">Password</span>
              <div
                style={{ display: "flex", gap: 8, alignItems: "center" }}
              >
                <input
                  type="password"
                  value={passwordInput}
                  onChange={(e) => setPasswordInput(e.target.value)}
                  placeholder="Set a password"
                  aria-label="Share password"
                  style={{
                    padding: "6px 10px",
                    fontSize: 13,
                    background: "var(--color-bg)",
                    border: "1px solid var(--color-border)",
                    borderRadius: 4,
                    color: "var(--color-text)",
                    width: 160,
                  }}
                />
                <button
                  onClick={setSharePassword}
                  disabled={!passwordInput.trim()}
                  className="detail-btn"
                >
                  Set
                </button>
                <button
                  onClick={removeSharePassword}
                  className="detail-btn"
                >
                  Remove
                </button>
              </div>
            </div>

            <div className="detail-setting-row">
              <span className="detail-setting-label">Email gate</span>
              <button
                onClick={toggleEmailGate}
                className={`detail-toggle${playlist.requireEmail ? " detail-toggle--active" : ""}`}
              >
                {playlist.requireEmail ? "Enabled" : "Disabled"}
              </button>
            </div>
          </>
        )}
      </div>

      {/* Delete */}
      <div
        style={{
          marginTop: 32,
          paddingTop: 16,
          borderTop: "1px solid var(--color-border)",
        }}
      >
        <button
          onClick={deletePlaylist}
          className="detail-btn detail-btn--danger"
          style={{ padding: "8px 20px" }}
        >
          Delete playlist
        </button>
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
              padding: 24,
              width: "calc(100vw - 32px)",
              maxWidth: 500,
              maxHeight: "80vh",
              overflow: "auto",
              border: "1px solid var(--color-border)",
            }}
          >
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
                  marginBottom: 16,
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

            <div style={{ display: "flex", gap: 8 }}>
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

      {/* Toast */}
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
