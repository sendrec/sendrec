import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { apiFetch } from "../../api/client";
import { ConfirmDialog, ConfirmDialogState } from "../../components/ConfirmDialog";
import { Toast } from "../../components/Toast";
import { TransferDialog } from "../../components/TransferDialog";
import { useOrganization } from "../../hooks/useOrganization";
import { useToast } from "../../hooks/useToast";
import { LimitsResponse } from "../../types/limits";
import type { Video, Folder, Tag } from "../../types/video";
import { copyToClipboard } from "../../utils/clipboard";
import { LibrarySidebar } from "./LibrarySidebar";
import { VideoGrid } from "./VideoGrid";

interface Playlist {
  id: string;
  title: string;
  videoCount: number;
}

export function Library() {
  const { selectedOrg } = useOrganization();
  const isViewer = selectedOrg?.role === "viewer";
  const [videos, setVideos] = useState<Video[]>([]);
  const [loading, setLoading] = useState(true);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const toast = useToast();
  const [downloadingId, setDownloadingId] = useState<string | null>(null);
  const [limits, setLimits] = useState<LimitsResponse | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const [folders, setFolders] = useState<Folder[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);
  const [playlists, setPlaylists] = useState<Playlist[]>([]);
  const [activeFilter, setActiveFilter] = useState<string>("all");
  const [creatingFolder, setCreatingFolder] = useState(false);
  const [creatingTag, setCreatingTag] = useState(false);
  const [newFolderName, setNewFolderName] = useState("");
  const [newTagName, setNewTagName] = useState("");
  const [newTagColor, setNewTagColor] = useState("#3b82f6");
  const [editingSidebarId, setEditingSidebarId] = useState<string | null>(null);
  const [editingSidebarName, setEditingSidebarName] = useState("");
  const [sidebarMenuId, setSidebarMenuId] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [batchLoading, setBatchLoading] = useState(false);
  const [transferVideoId, setTransferVideoId] = useState<string | null>(null);
  const [confirmDialog, setConfirmDialog] = useState<ConfirmDialogState | null>(null);
  const [viewMode, setViewModeState] = useState<"grid" | "list">(() => {
    const stored = localStorage.getItem("library-view");
    return stored === "list" ? "list" : "grid";
  });
  function setViewMode(mode: "grid" | "list") {
    setViewModeState(mode);
    localStorage.setItem("library-view", mode);
  }
  const [sortBy, setSortBy] = useState<"newest" | "oldest" | "most-viewed" | "title">("newest");

  const sortedVideos = useMemo(() => {
    const sorted = [...videos];
    switch (sortBy) {
      case "newest": sorted.sort((a, b) => b.createdAt.localeCompare(a.createdAt)); break;
      case "oldest": sorted.sort((a, b) => a.createdAt.localeCompare(b.createdAt)); break;
      case "most-viewed": sorted.sort((a, b) => b.viewCount - a.viewCount); break;
      case "title": sorted.sort((a, b) => a.title.localeCompare(b.title)); break;
    }
    return sorted;
  }, [videos, sortBy]);

  const fetchVideosAndLimits = useCallback(async (query = "", filter = "all") => {
    const params = new URLSearchParams();
    if (query) params.set("q", query);
    if (filter.startsWith("folder:")) params.set("folder_id", filter.slice(7));
    else if (filter === "unfiled") params.set("folder_id", "unfiled");
    else if (filter.startsWith("tag:")) params.set("tag_id", filter.slice(4));
    const qs = params.toString();
    const [videosResult, limitsResult] = await Promise.all([
      apiFetch<Video[]>(`/api/videos${qs ? `?${qs}` : ""}`),
      apiFetch<LimitsResponse>("/api/videos/limits"),
    ]);
    setVideos(videosResult ?? []);
    setLimits(limitsResult ?? null);
  }, []);

  const fetchFoldersAndTags = useCallback(async () => {
    const [foldersResult, tagsResult, playlistsResult] = await Promise.all([
      apiFetch<Folder[]>("/api/folders"),
      apiFetch<Tag[]>("/api/tags"),
      apiFetch<Playlist[]>("/api/playlists"),
    ]);
    setFolders(foldersResult ?? []);
    setTags(tagsResult ?? []);
    setPlaylists(playlistsResult ?? []);
  }, []);

  useEffect(() => {
    async function fetchData() {
      try {
        await Promise.all([fetchVideosAndLimits(), fetchFoldersAndTags()]);
      } catch {
        setVideos([]);
      } finally {
        setLoading(false);
      }
    }

    fetchData();
  }, [fetchVideosAndLimits, fetchFoldersAndTags]);

  useEffect(() => {
    const hasProcessing = videos.some(
      (v) => v.status === "processing" || v.transcriptStatus === "processing" || v.transcriptStatus === "pending"
    );
    if (!hasProcessing) return;

    const interval = setInterval(async () => {
      try {
        await fetchVideosAndLimits(searchQuery, activeFilter);
      } catch {
        // ignore poll errors
      }
    }, 5000);

    return () => clearInterval(interval);
  }, [videos, searchQuery, activeFilter, fetchVideosAndLimits]);

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

  useEffect(() => {
    if (!sidebarMenuId) return;
    function handleClick() { setSidebarMenuId(null); }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [sidebarMenuId]);

  function handleSearchChange(value: string) {
    setSearchQuery(value);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      fetchVideosAndLimits(value, activeFilter);
    }, 300);
  }

  function handleFilterChange(filter: string) {
    setActiveFilter(filter);
    setSelectedIds(new Set());
    fetchVideosAndLimits(searchQuery, filter);
  }

  function deleteVideo(id: string) {
    setConfirmDialog({
      message: "Delete this recording? This cannot be undone.",
      confirmLabel: "Delete",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        setDeletingId(id);
        try {
          await apiFetch(`/api/videos/${id}`, { method: "DELETE" });
          setVideos((prev) => prev.filter((v) => v.id !== id));
          setLimits((prev) => prev ? { ...prev, videosUsedThisMonth: Math.max(0, prev.videosUsedThisMonth - 1) } : prev);
        } finally {
          setDeletingId(null);
        }
      },
    });
  }

  async function copyLink(shareUrl: string) {
    await copyToClipboard(shareUrl);
    toast.show("Link copied");
  }

  async function togglePin(id: string) {
    try {
      const resp = await apiFetch<{ pinned: boolean }>(`/api/videos/${id}/pin`, { method: "PUT" });
      if (resp) {
        setVideos((prev) => prev.map((v) => v.id === id ? { ...v, pinned: resp.pinned } : v));
        toast.show(resp.pinned ? "Video pinned" : "Video unpinned");
      }
    } catch {
      toast.show("Failed to update pin");
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

  async function createFolder() {
    if (!newFolderName.trim()) return;
    await apiFetch("/api/folders", { method: "POST", body: JSON.stringify({ name: newFolderName.trim() }) });
    setNewFolderName("");
    setCreatingFolder(false);
    await fetchFoldersAndTags();
  }

  async function createTag() {
    if (!newTagName.trim()) return;
    await apiFetch("/api/tags", { method: "POST", body: JSON.stringify({ name: newTagName.trim(), color: newTagColor }) });
    setNewTagName("");
    setCreatingTag(false);
    await fetchFoldersAndTags();
  }

  async function renameSidebarItem(type: "folder" | "tag", id: string) {
    if (!editingSidebarName.trim()) return;
    const url = type === "folder" ? `/api/folders/${id}` : `/api/tags/${id}`;
    await apiFetch(url, { method: "PUT", body: JSON.stringify({ name: editingSidebarName.trim() }) });
    setEditingSidebarId(null);
    await fetchFoldersAndTags();
  }

  function deleteSidebarItem(type: "folder" | "tag", id: string) {
    const msg = type === "folder" ? "Delete this folder? Videos will become unfiled." : "Delete this tag? It will be removed from all videos.";
    setConfirmDialog({
      message: msg,
      confirmLabel: "Delete",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        const url = type === "folder" ? `/api/folders/${id}` : `/api/tags/${id}`;
        await apiFetch(url, { method: "DELETE" });
        if (activeFilter === `${type}:${id}`) setActiveFilter("all");
        await fetchFoldersAndTags();
        await fetchVideosAndLimits(searchQuery, activeFilter === `${type}:${id}` ? "all" : activeFilter);
      },
    });
  }

  function toggleSelect(id: string) {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function selectAll() {
    setSelectedIds(new Set(videos.map(v => v.id)));
  }

  function deselectAll() {
    setSelectedIds(new Set());
  }

  function batchDelete() {
    const count = selectedIds.size;
    setConfirmDialog({
      message: `Delete ${count} video(s)? This cannot be undone.`,
      confirmLabel: "Delete",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        setBatchLoading(true);
        try {
          await apiFetch("/api/videos/batch/delete", {
            method: "POST",
            body: JSON.stringify({ videoIds: Array.from(selectedIds) }),
          });
          toast.show(`Deleted ${count} video(s)`);
          setSelectedIds(new Set());
          fetchVideosAndLimits(searchQuery, activeFilter);
        } catch {
          toast.show("Failed to delete videos");
        } finally {
          setBatchLoading(false);
        }
      },
    });
  }

  async function batchSetFolder(folderId: string | null) {
    setBatchLoading(true);
    try {
      await apiFetch("/api/videos/batch/folder", {
        method: "POST",
        body: JSON.stringify({ videoIds: Array.from(selectedIds), folderId }),
      });
      toast.show("Moved videos");
      setSelectedIds(new Set());
      fetchVideosAndLimits(searchQuery, activeFilter);
      fetchFoldersAndTags();
    } catch {
      toast.show("Failed to move videos");
    } finally {
      setBatchLoading(false);
    }
  }

  if (loading) {
    return (
      <div className="page-container page-container--wide">
        <div className="library-header">
          <div>
            <h1 style={{ color: "var(--color-text)", fontSize: 24, margin: 0 }}>Library</h1>
          </div>
        </div>
        <div className="video-grid">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="skeleton-card">
              <div className="skeleton-thumb skeleton" />
              <div className="skeleton-body">
                <div className="skeleton-title skeleton" />
                <div className="skeleton-meta skeleton" />
                <div className="skeleton-meta-short skeleton" />
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (videos.length === 0) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16, marginBottom: 20 }}>No recordings yet.</p>
        <div style={{
          maxWidth: 400,
          margin: "0 auto 24px",
          padding: "20px 24px",
          background: "var(--color-surface)",
          borderRadius: 12,
          textAlign: "left",
        }}>
          <p style={{ color: "var(--color-text)", fontSize: 15, fontWeight: 600, marginBottom: 12 }}>
            Get started in 3 steps
          </p>
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span style={{ color: "var(--color-accent)", fontWeight: 700, fontSize: 16 }}>1.</span>
              <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Record your screen or upload a video</span>
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span style={{ color: "var(--color-accent)", fontWeight: 700, fontSize: 16 }}>2.</span>
              <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Share the link with anyone</span>
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span style={{ color: "var(--color-accent)", fontWeight: 700, fontSize: 16 }}>3.</span>
              <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Track views and get feedback</span>
            </div>
          </div>
        </div>
        <div style={{ display: "flex", gap: 12, flexWrap: "wrap", justifyContent: "center" }}>
          {!isViewer && (
            <>
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
                Record
              </Link>
              <Link
                to="/?tab=upload"
                style={{
                  background: "transparent",
                  color: "var(--color-accent)",
                  border: "1px solid var(--color-accent)",
                  borderRadius: 8,
                  padding: "10px 24px",
                  fontSize: 14,
                  fontWeight: 600,
                  textDecoration: "none",
                }}
              >
                Upload
              </Link>
            </>
          )}
        </div>
        {limits && limits.maxVideosPerMonth > 0 && (
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13, marginTop: 16 }}>
            {limits.videosUsedThisMonth} / {limits.maxVideosPerMonth} videos this month
          </p>
        )}
      </div>
    );
  }

  return (
    <div className="page-container page-container--wide">
      <div className="library-layout">
        <LibrarySidebar
          folders={folders}
          tags={tags}
          playlists={playlists}
          limits={limits}
          activeFilter={activeFilter}
          onFilterChange={handleFilterChange}
          creatingFolder={creatingFolder}
          onSetCreatingFolder={setCreatingFolder}
          newFolderName={newFolderName}
          onSetNewFolderName={setNewFolderName}
          onCreateFolder={createFolder}
          creatingTag={creatingTag}
          onSetCreatingTag={setCreatingTag}
          newTagName={newTagName}
          onSetNewTagName={setNewTagName}
          newTagColor={newTagColor}
          onSetNewTagColor={setNewTagColor}
          onCreateTag={createTag}
          editingSidebarId={editingSidebarId}
          editingSidebarName={editingSidebarName}
          onSetEditingSidebarId={setEditingSidebarId}
          onSetEditingSidebarName={setEditingSidebarName}
          onRenameSidebarItem={renameSidebarItem}
          onDeleteSidebarItem={deleteSidebarItem}
          sidebarMenuId={sidebarMenuId}
          onSetSidebarMenuId={setSidebarMenuId}
        />

        <VideoGrid
          sortedVideos={sortedVideos}
          videos={videos}
          limits={limits}
          folders={folders}
          isViewer={isViewer}
          searchQuery={searchQuery}
          onSearchChange={handleSearchChange}
          sortBy={sortBy}
          onSortByChange={setSortBy}
          viewMode={viewMode}
          onSetViewMode={setViewMode}
          selectedIds={selectedIds}
          onToggleSelect={toggleSelect}
          onSelectAll={selectAll}
          onDeselectAll={deselectAll}
          batchLoading={batchLoading}
          onBatchDelete={batchDelete}
          onBatchSetFolder={batchSetFolder}
          openMenuId={openMenuId}
          onSetOpenMenuId={setOpenMenuId}
          menuRef={menuRef}
          deletingId={deletingId}
          downloadingId={downloadingId}
          onDeleteVideo={deleteVideo}
          onCopyLink={copyLink}
          onTogglePin={togglePin}
          onDownloadVideo={downloadVideo}
          onSetTransferVideoId={setTransferVideoId}
        />
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

      {transferVideoId && (
        <TransferDialog
          videoId={transferVideoId}
          videoTitle={videos.find((v) => v.id === transferVideoId)?.title ?? "Untitled"}
          onTransferred={() => {
            setTransferVideoId(null);
            toast.show("Video moved");
            fetchVideosAndLimits(searchQuery, activeFilter);
          }}
          onCancel={() => setTransferVideoId(null)}
        />
      )}
    </div>
  );
}
