import { useState, useEffect, useCallback } from "react";
import { Link } from "react-router-dom";
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

export function Playlists() {
  const [playlists, setPlaylists] = useState<Playlist[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newTitle, setNewTitle] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");

  const fetchPlaylists = useCallback(async () => {
    try {
      const data = await apiFetch<Playlist[]>("/api/playlists");
      setPlaylists(data ?? []);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchPlaylists();
  }, [fetchPlaylists]);

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
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16 }}>
          Loading...
        </p>
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

      {playlists.length === 0 ? (
        <div className="page-container--centered">
          <p
            style={{
              color: "var(--color-text-secondary)",
              fontSize: 16,
              marginBottom: 8,
            }}
          >
            No playlists yet
          </p>
          <p style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>
            Create a playlist to organize and share collections of videos.
          </p>
        </div>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          {playlists.map((playlist) => (
            <div
              key={playlist.id}
              style={{
                background: "var(--color-surface)",
                border: "1px solid var(--color-border)",
                borderRadius: 8,
                padding: 16,
                display: "flex",
                alignItems: "center",
                gap: 12,
              }}
            >
              <Link
                to={`/playlists/${playlist.id}`}
                style={{
                  flex: 1,
                  minWidth: 0,
                  textDecoration: "none",
                }}
              >
                <h3
                  style={{
                    color: "var(--color-text)",
                    fontSize: 15,
                    fontWeight: 600,
                    margin: 0,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {playlist.title}
                </h3>
                <div
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 8,
                    marginTop: 4,
                  }}
                >
                  <span
                    style={{
                      color: "var(--color-text-secondary)",
                      fontSize: 13,
                    }}
                  >
                    {playlist.videoCount}{" "}
                    {playlist.videoCount === 1 ? "video" : "videos"}
                  </span>
                  {playlist.isShared && (
                    <span
                      style={{
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
                </div>
              </Link>
              <button
                className="detail-btn detail-btn--danger"
                onClick={() => handleDelete(playlist.id)}
                aria-label="Delete playlist"
              >
                Delete
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
