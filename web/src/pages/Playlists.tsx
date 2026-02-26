import { useState, useEffect, useCallback, useRef } from "react";
import { Link, useNavigate } from "react-router-dom";
import { apiFetch } from "../api/client";

interface Playlist {
  id: string;
  title: string;
  description?: string;
  isShared: boolean;
  shareToken?: string;
  shareUrl?: string;
  videoCount: number;
  position: number;
  createdAt: string;
  updatedAt: string;
}

interface LimitsResponse {
  maxPlaylists: number;
  playlistsUsed: number;
}

function formatDate(isoDate: string): string {
  return new Date(isoDate).toLocaleDateString("en-GB");
}

export function Playlists() {
  const navigate = useNavigate();
  const [playlists, setPlaylists] = useState<Playlist[]>([]);
  const [limits, setLimits] = useState<LimitsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newTitle, setNewTitle] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");
  const [menuId, setMenuId] = useState<string | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  const fetchPlaylists = useCallback(async () => {
    try {
      const [data, limitsData] = await Promise.all([
        apiFetch<Playlist[]>("/api/playlists"),
        apiFetch<LimitsResponse>("/api/videos/limits"),
      ]);
      setPlaylists(data ?? []);
      setLimits(limitsData ?? null);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchPlaylists();
  }, [fetchPlaylists]);

  useEffect(() => {
    if (!menuId) return;
    function handleClickOutside(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuId(null);
      }
    }
    function handleEscape(e: KeyboardEvent) {
      if (e.key === "Escape") setMenuId(null);
    }
    document.addEventListener("mousedown", handleClickOutside);
    document.addEventListener("keydown", handleEscape);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleEscape);
    };
  }, [menuId]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newTitle.trim() || creating) return;
    setCreating(true);
    setError("");
    try {
      await apiFetch("/api/playlists", {
        method: "POST",
        body: JSON.stringify({ title: newTitle.trim() }),
      });
      setNewTitle("");
      setShowCreate(false);
      fetchPlaylists();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to create playlist",
      );
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm("Delete this playlist? Videos will not be deleted.")) return;
    try {
      await apiFetch(`/api/playlists/${id}`, { method: "DELETE" });
      fetchPlaylists();
    } catch {
      // ignore
    }
  };

  if (loading) {
    return (
      <div className="page-container">
        <div className="library-header">
          <h1
            style={{ color: "var(--color-text)", fontSize: 24, margin: 0 }}
          >
            Playlists
          </h1>
        </div>
        <div className="video-grid">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="skeleton-card">
              <div className="skeleton-thumb skeleton" />
              <div className="skeleton-body">
                <div className="skeleton-title skeleton" />
                <div className="skeleton-meta skeleton" />
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="page-container">
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          marginBottom: 24,
        }}
      >
        <h1 style={{ color: "var(--color-text)", fontSize: 24, margin: 0 }}>
          Playlists
        </h1>
        <button
          className="detail-btn detail-btn--accent"
          onClick={() => setShowCreate(!showCreate)}
        >
          {showCreate ? "Cancel" : "New Playlist"}
        </button>
      </div>

      {showCreate && (
        <form
          onSubmit={handleCreate}
          style={{
            display: "flex",
            gap: 8,
            marginBottom: 24,
            alignItems: "center",
          }}
        >
          <input
            type="text"
            value={newTitle}
            onChange={(e) => setNewTitle(e.target.value)}
            placeholder="Playlist title"
            maxLength={200}
            autoFocus
            style={{
              flex: 1,
              padding: "8px 12px",
              fontSize: 14,
              background: "var(--color-surface)",
              border: "1px solid var(--color-border)",
              borderRadius: 8,
              color: "var(--color-text)",
            }}
          />
          <button
            type="submit"
            className="detail-btn detail-btn--accent"
            disabled={creating || !newTitle.trim()}
          >
            {creating ? "Creating..." : "Create"}
          </button>
          {error && (
            <p
              style={{
                color: "var(--color-error)",
                fontSize: 13,
                margin: 0,
              }}
            >
              {error}
            </p>
          )}
        </form>
      )}

      {limits && limits.maxPlaylists > 0 && (
        <div className="playlist-usage">
          <span>
            <strong>{limits.playlistsUsed}</strong> of {limits.maxPlaylists}{" "}
            playlists used
          </span>
          <div className="playlist-usage-bar">
            <div className="usage-bar">
              <div
                className={`usage-bar-fill${limits.playlistsUsed >= limits.maxPlaylists ? " usage-bar-fill--warning" : ""}`}
                style={{
                  width: `${Math.min(100, (limits.playlistsUsed / limits.maxPlaylists) * 100)}%`,
                }}
              />
            </div>
          </div>
        </div>
      )}

      {playlists.length === 0 ? (
        <div className="empty-state">
          <div className="empty-state-icon">
            <svg
              width="28"
              height="28"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <rect x="2" y="3" width="20" height="14" rx="2" />
              <path d="M2 20h20" />
              <path d="M6 20v-3" />
              <path d="M18 20v-3" />
              <polygon points="10,7 10,13 15,10" />
            </svg>
          </div>
          <h2 className="empty-state-title">No playlists yet</h2>
          <p className="empty-state-desc">
            Create a playlist to organize and share collections of videos.
          </p>
          <button
            className="detail-btn detail-btn--accent"
            onClick={() => setShowCreate(true)}
          >
            Create your first playlist
          </button>
        </div>
      ) : (
        <div className="video-grid">
          {playlists.map((playlist) => (
            <div
              key={playlist.id}
              className="playlist-card"
              onClick={() => navigate(`/playlists/${playlist.id}`)}
            >
              <button
                className="playlist-card-menu"
                onClick={(e) => {
                  e.stopPropagation();
                  setMenuId(menuId === playlist.id ? null : playlist.id);
                }}
                aria-label="Playlist options"
              >
                &middot;&middot;&middot;
              </button>
              {menuId === playlist.id && (
                <div
                  className="playlist-context-menu"
                  ref={menuRef}
                  onClick={(e) => e.stopPropagation()}
                >
                  <button
                    onClick={() => {
                      navigate(`/playlists/${playlist.id}`);
                      setMenuId(null);
                    }}
                  >
                    Edit
                  </button>
                  <button
                    className="playlist-context-menu-danger"
                    onClick={() => {
                      handleDelete(playlist.id);
                      setMenuId(null);
                    }}
                  >
                    Delete
                  </button>
                </div>
              )}
              <div className="playlist-thumb-stack">
                <div className="playlist-thumb playlist-thumb--back2" />
                <div className="playlist-thumb playlist-thumb--back1" />
                <div className="playlist-thumb playlist-thumb--front">
                  <span className="playlist-thumb-play" />
                  <span className="playlist-thumb-count">
                    {playlist.videoCount}{" "}
                    {playlist.videoCount === 1 ? "video" : "videos"}
                  </span>
                </div>
              </div>
              <div className="playlist-card-body">
                <Link
                  to={`/playlists/${playlist.id}`}
                  className="playlist-card-title"
                  onClick={(e) => e.stopPropagation()}
                >
                  {playlist.title}
                </Link>
                <div className="playlist-card-meta">
                  <span className="playlist-card-meta-text">
                    {playlist.videoCount}{" "}
                    {playlist.videoCount === 1 ? "video" : "videos"}
                  </span>
                  <span
                    className={`playlist-card-badge ${playlist.isShared ? "playlist-card-badge--shared" : "playlist-card-badge--private"}`}
                  >
                    {playlist.isShared ? "Shared" : "Private"}
                  </span>
                </div>
                <div className="playlist-card-date">
                  Created {formatDate(playlist.createdAt)}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
