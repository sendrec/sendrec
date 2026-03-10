import { useState, useEffect, useCallback } from "react";
import { useParams, useNavigate, Link } from "react-router-dom";
import { apiFetch } from "../../api/client";
import { ConfirmDialog, ConfirmDialogState } from "../../components/ConfirmDialog";
import { PromptDialog } from "../../components/PromptDialog";
import { Toast } from "../../components/Toast";
import { useToast } from "../../hooks/useToast";
import { PlaylistVideos } from "./PlaylistVideos";
import type { PlaylistVideo } from "./PlaylistVideos";
import { PlaylistSharing } from "./PlaylistSharing";

interface PlaylistData {
  id: string;
  title: string;
  description?: string;
  isShared: boolean;
  shareToken?: string;
  shareUrl?: string;
  hasPassword: boolean;
  requireEmail: boolean;
  position: number;
  videoCount: number;
  videos: PlaylistVideo[];
  createdAt: string;
  updatedAt: string;
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

  const toast = useToast();
  const [confirmDialog, setConfirmDialog] = useState<ConfirmDialogState | null>(null);
  const [promptDialog, setPromptDialog] = useState<{
    title: string;
    onSubmit: (value: string) => void;
    placeholder?: string;
    submitLabel?: string;
  } | null>(null);

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

  function handleVideosChanged(videos: PlaylistVideo[]) {
    setPlaylist((prev) =>
      prev ? { ...prev, videos, videoCount: videos.length } : prev,
    );
  }

  function deletePlaylist() {
    if (!playlist) return;
    setConfirmDialog({
      message: "Delete this playlist? Videos will not be deleted.",
      confirmLabel: "Delete",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        await apiFetch(`/api/playlists/${id}`, { method: "DELETE" });
        navigate("/playlists");
      },
    });
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
      <PlaylistVideos
        playlistId={id!}
        videos={playlist.videos}
        onVideosChanged={handleVideosChanged}
        onPlaylistRefresh={fetchPlaylist}
        showToast={toast.show}
      />

      {/* Sharing Section */}
      <PlaylistSharing
        playlistId={id!}
        playlist={playlist}
        onPlaylistUpdate={(updates) =>
          setPlaylist((prev) => (prev ? { ...prev, ...updates } : prev))
        }
        onPlaylistRefresh={fetchPlaylist}
        showToast={toast.show}
        setConfirmDialog={setConfirmDialog}
        setPromptDialog={setPromptDialog}
      />

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

      <Toast message={toast.message} />

      {confirmDialog && (
        <ConfirmDialog
          message={confirmDialog.message}
          confirmLabel={confirmDialog.confirmLabel}
          danger={confirmDialog.danger}
          onConfirm={confirmDialog.onConfirm}
          onCancel={() => setConfirmDialog(null)}
        />
      )}

      {promptDialog && (
        <PromptDialog
          title={promptDialog.title}
          placeholder={promptDialog.placeholder}
          submitLabel={promptDialog.submitLabel}
          onSubmit={promptDialog.onSubmit}
          onCancel={() => setPromptDialog(null)}
        />
      )}
    </div>
  );
}
