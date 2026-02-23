import { useCallback, useEffect, useRef, useState } from "react";
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
  suggestedTitle: string | null;
  summaryStatus: string;
  folderId: string | null;
  tags: VideoTag[];
}

interface LimitsResponse {
  maxVideosPerMonth: number;
  maxVideoDurationSeconds: number;
  videosUsedThisMonth: number;
  brandingEnabled: boolean;
  aiEnabled: boolean;
}

interface Folder {
  id: string;
  name: string;
  position: number;
  videoCount: number;
  createdAt: string;
}

interface Tag {
  id: string;
  name: string;
  color: string | null;
  videoCount: number;
  createdAt: string;
}

interface VideoTag {
  id: string;
  name: string;
  color: string | null;
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
  const [toast, setToast] = useState<string | null>(null);
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [downloadingId, setDownloadingId] = useState<string | null>(null);
  const [limits, setLimits] = useState<LimitsResponse | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const [folders, setFolders] = useState<Folder[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);
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
    const [foldersResult, tagsResult] = await Promise.all([
      apiFetch<Folder[]>("/api/folders"),
      apiFetch<Tag[]>("/api/tags"),
    ]);
    setFolders(foldersResult ?? []);
    setTags(tagsResult ?? []);
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

  async function deleteSidebarItem(type: "folder" | "tag", id: string) {
    const msg = type === "folder" ? "Delete this folder? Videos will become unfiled." : "Delete this tag? It will be removed from all videos.";
    if (!window.confirm(msg)) return;
    const url = type === "folder" ? `/api/folders/${id}` : `/api/tags/${id}`;
    await apiFetch(url, { method: "DELETE" });
    if (activeFilter === `${type}:${id}`) setActiveFilter("all");
    await fetchFoldersAndTags();
    await fetchVideosAndLimits(searchQuery, activeFilter === `${type}:${id}` ? "all" : activeFilter);
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

  async function batchDelete() {
    if (!window.confirm(`Delete ${selectedIds.size} video(s)? This cannot be undone.`)) return;
    setBatchLoading(true);
    try {
      await apiFetch("/api/videos/batch/delete", {
        method: "POST",
        body: JSON.stringify({ videoIds: Array.from(selectedIds) }),
      });
      showToast(`Deleted ${selectedIds.size} video(s)`);
      setSelectedIds(new Set());
      fetchVideosAndLimits(searchQuery, activeFilter);
    } catch {
      showToast("Failed to delete videos");
    } finally {
      setBatchLoading(false);
    }
  }

  async function batchSetFolder(folderId: string | null) {
    setBatchLoading(true);
    try {
      await apiFetch("/api/videos/batch/folder", {
        method: "POST",
        body: JSON.stringify({ videoIds: Array.from(selectedIds), folderId }),
      });
      showToast("Moved videos");
      setSelectedIds(new Set());
      fetchVideosAndLimits(searchQuery, activeFilter);
      fetchFoldersAndTags();
    } catch {
      showToast("Failed to move videos");
    } finally {
      setBatchLoading(false);
    }
  }

  async function batchSetTags(tagIds: string[]) {
    setBatchLoading(true);
    try {
      await apiFetch("/api/videos/batch/tags", {
        method: "POST",
        body: JSON.stringify({ videoIds: Array.from(selectedIds), tagIds }),
      });
      showToast("Updated tags");
      setSelectedIds(new Set());
      fetchVideosAndLimits(searchQuery, activeFilter);
      fetchFoldersAndTags();
    } catch {
      showToast("Failed to update tags");
    } finally {
      setBatchLoading(false);
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
            to="/upload"
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
    <div className="page-container">
      <div className="library-layout">
        <nav className="library-sidebar">
          <button
            className={`sidebar-item${activeFilter === "all" ? " sidebar-item--active" : ""}`}
            onClick={() => handleFilterChange("all")}
          >
            All Videos
          </button>
          <button
            className={`sidebar-item${activeFilter === "unfiled" ? " sidebar-item--active" : ""}`}
            onClick={() => handleFilterChange("unfiled")}
          >
            Unfiled
          </button>

          <div className="sidebar-section">
            <div className="sidebar-section-header">
              <span>Folders</span>
              <button className="sidebar-add-btn" onClick={() => setCreatingFolder(true)} title="New folder">+</button>
            </div>
            {creatingFolder && (
              <div style={{ padding: "4px 8px" }}>
                <input
                  autoFocus
                  value={newFolderName}
                  onChange={(e) => setNewFolderName(e.target.value)}
                  onKeyDown={(e) => { if (e.key === "Enter") createFolder(); if (e.key === "Escape") setCreatingFolder(false); }}
                  placeholder="Folder name"
                  maxLength={100}
                  style={{ width: "100%", padding: "4px 8px", fontSize: 13, background: "var(--color-background)", border: "1px solid var(--color-border)", borderRadius: 4, color: "var(--color-text)" }}
                />
              </div>
            )}
            {folders.map((folder) => (
              <div key={folder.id} className="sidebar-item-wrapper" onMouseLeave={() => { if (sidebarMenuId === `folder-${folder.id}`) setSidebarMenuId(null); }}>
                {editingSidebarId === `folder-${folder.id}` ? (
                  <input
                    autoFocus
                    value={editingSidebarName}
                    onChange={(e) => setEditingSidebarName(e.target.value)}
                    onKeyDown={(e) => { if (e.key === "Enter") renameSidebarItem("folder", folder.id); if (e.key === "Escape") setEditingSidebarId(null); }}
                    onBlur={() => renameSidebarItem("folder", folder.id)}
                    style={{ width: "100%", padding: "4px 8px", fontSize: 13, background: "var(--color-background)", border: "1px solid var(--color-border)", borderRadius: 4, color: "var(--color-text)" }}
                  />
                ) : (
                  <button
                    className={`sidebar-item${activeFilter === `folder:${folder.id}` ? " sidebar-item--active" : ""}`}
                    onClick={() => handleFilterChange(`folder:${folder.id}`)}
                  >
                    <span className="sidebar-item-name">{folder.name}</span>
                    <span className="sidebar-item-count">{folder.videoCount}</span>
                  </button>
                )}
                <button
                  className="sidebar-item-menu-btn"
                  onClick={(e) => { e.stopPropagation(); setSidebarMenuId(sidebarMenuId === `folder-${folder.id}` ? null : `folder-${folder.id}`); }}
                >
                  &#x22EE;
                </button>
                {sidebarMenuId === `folder-${folder.id}` && (
                  <div className="sidebar-item-menu" onClick={(e) => e.stopPropagation()}>
                    <button onClick={() => { setEditingSidebarId(`folder-${folder.id}`); setEditingSidebarName(folder.name); setSidebarMenuId(null); }}>Rename</button>
                    <button onClick={() => { deleteSidebarItem("folder", folder.id); setSidebarMenuId(null); }}>Delete</button>
                  </div>
                )}
              </div>
            ))}
          </div>

          <div className="sidebar-section">
            <div className="sidebar-section-header">
              <span>Tags</span>
              <button className="sidebar-add-btn" onClick={() => setCreatingTag(true)} title="New tag">+</button>
            </div>
            {creatingTag && (
              <div style={{ padding: "4px 8px" }}>
                <div style={{ display: "flex", gap: 4, alignItems: "center" }}>
                  <input
                    type="color"
                    value={newTagColor}
                    onChange={(e) => setNewTagColor(e.target.value)}
                    style={{ width: 24, height: 24, padding: 0, border: "none", cursor: "pointer" }}
                  />
                  <input
                    autoFocus
                    value={newTagName}
                    onChange={(e) => setNewTagName(e.target.value)}
                    onKeyDown={(e) => { if (e.key === "Enter") createTag(); if (e.key === "Escape") setCreatingTag(false); }}
                    placeholder="Tag name"
                    maxLength={50}
                    style={{ flex: 1, padding: "4px 8px", fontSize: 13, background: "var(--color-background)", border: "1px solid var(--color-border)", borderRadius: 4, color: "var(--color-text)" }}
                  />
                </div>
              </div>
            )}
            {tags.map((tag) => (
              <div key={tag.id} className="sidebar-item-wrapper" onMouseLeave={() => { if (sidebarMenuId === `tag-${tag.id}`) setSidebarMenuId(null); }}>
                {editingSidebarId === `tag-${tag.id}` ? (
                  <input
                    autoFocus
                    value={editingSidebarName}
                    onChange={(e) => setEditingSidebarName(e.target.value)}
                    onKeyDown={(e) => { if (e.key === "Enter") renameSidebarItem("tag", tag.id); if (e.key === "Escape") setEditingSidebarId(null); }}
                    onBlur={() => renameSidebarItem("tag", tag.id)}
                    style={{ width: "100%", padding: "4px 8px", fontSize: 13, background: "var(--color-background)", border: "1px solid var(--color-border)", borderRadius: 4, color: "var(--color-text)" }}
                  />
                ) : (
                  <button
                    className={`sidebar-item${activeFilter === `tag:${tag.id}` ? " sidebar-item--active" : ""}`}
                    onClick={() => handleFilterChange(`tag:${tag.id}`)}
                  >
                    <span className="tag-dot" style={{ background: tag.color ?? "var(--color-text-secondary)" }} />
                    <span className="sidebar-item-name">{tag.name}</span>
                    <span className="sidebar-item-count">{tag.videoCount}</span>
                  </button>
                )}
                <button
                  className="sidebar-item-menu-btn"
                  onClick={(e) => { e.stopPropagation(); setSidebarMenuId(sidebarMenuId === `tag-${tag.id}` ? null : `tag-${tag.id}`); }}
                >
                  &#x22EE;
                </button>
                {sidebarMenuId === `tag-${tag.id}` && (
                  <div className="sidebar-item-menu" onClick={(e) => e.stopPropagation()}>
                    <button onClick={() => { setEditingSidebarId(`tag-${tag.id}`); setEditingSidebarName(tag.name); setSidebarMenuId(null); }}>Rename</button>
                    <button onClick={() => { deleteSidebarItem("tag", tag.id); setSidebarMenuId(null); }}>Delete</button>
                  </div>
                )}
              </div>
            ))}
          </div>
        </nav>

        <div className="library-main">
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

      {selectedIds.size > 0 && (
        <div className="batch-toolbar">
          <span style={{ fontWeight: 600, fontSize: 14 }}>
            {selectedIds.size} selected
          </span>
          <button onClick={selectAll} className="detail-btn">Select all</button>
          <button onClick={deselectAll} className="detail-btn">Deselect all</button>
          <select
            onChange={(e) => {
              const val = e.target.value;
              batchSetFolder(val === "__none__" ? null : val);
              e.target.value = "";
            }}
            defaultValue=""
            className="detail-btn"
            style={{ cursor: "pointer" }}
          >
            <option value="" disabled>Move to folder...</option>
            <option value="__none__">No folder</option>
            {folders.map(f => <option key={f.id} value={f.id}>{f.name}</option>)}
          </select>
          <button onClick={batchDelete} className="detail-btn detail-btn--danger" disabled={batchLoading}>
            Delete
          </button>
        </div>
      )}

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
            <input
              type="checkbox"
              checked={selectedIds.has(video.id)}
              onChange={() => toggleSelect(video.id)}
              className={`video-select-checkbox${selectedIds.size > 0 ? " video-select-checkbox--visible" : ""}`}
              aria-label={`Select ${video.title}`}
            />
            <div className="video-card-top">
              {video.thumbnailUrl && (
                <Link to={`/videos/${video.id}`} state={{ video }} style={{ flexShrink: 0 }}>
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
                </Link>
              )}
              <div style={{ minWidth: 0, flex: 1 }}>
                <Link
                  to={`/videos/${video.id}`}
                  state={{ video }}
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
                </Link>
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
              </div>
            </div>

            {video.tags.length > 0 && (
              <div style={{ display: "flex", flexWrap: "wrap", gap: 4, marginTop: 6 }}>
                {video.tags.map((tag) => (
                  <span
                    key={tag.id}
                    style={{
                      display: "inline-flex", alignItems: "center", gap: 4,
                      padding: "2px 8px", borderRadius: 12, fontSize: 11, fontWeight: 500,
                      background: "var(--color-background)", border: "1px solid var(--color-border)",
                      color: "var(--color-text-secondary)",
                    }}
                  >
                    <span style={{ width: 6, height: 6, borderRadius: "50%", background: tag.color ?? "var(--color-text-secondary)" }} />
                    {tag.name}
                  </span>
                ))}
              </div>
            )}

            {video.status === "ready" && (
              <div className="video-card-actions" style={{ borderTop: "1px solid var(--color-border)", marginTop: 12, paddingTop: 10 }}>
                <div style={{ display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
                  <button
                    onClick={() => copyLink(video.shareUrl)}
                    className="action-link"
                  >
                    Copy link
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
                        className="dropdown-menu"
                        style={{
                          position: "absolute",
                          top: "100%",
                          right: 0,
                          marginTop: 4,
                          background: "var(--color-surface)",
                          border: "1px solid var(--color-border)",
                          borderRadius: 8,
                          padding: "4px 0",
                          minWidth: 160,
                          zIndex: 50,
                          boxShadow: "0 4px 16px var(--color-shadow)",
                        }}
                      >
                        <Link
                          to={`/videos/${video.id}/analytics`}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px", textDecoration: "none" }}
                          onClick={() => setOpenMenuId(null)}
                        >
                          Analytics
                        </Link>
                        <button
                          onClick={() => { downloadVideo(video.id); setOpenMenuId(null); }}
                          disabled={downloadingId === video.id}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px", opacity: downloadingId === video.id ? 0.5 : undefined }}
                        >
                          {downloadingId === video.id ? "Downloading..." : "Download"}
                        </button>
                        <div style={{ borderTop: "1px solid var(--color-border)", margin: "4px 0" }} />
                        <button
                          onClick={() => { deleteVideo(video.id); setOpenMenuId(null); }}
                          disabled={deletingId === video.id}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px", color: "var(--color-error)", opacity: deletingId === video.id ? 0.5 : undefined }}
                        >
                          {deletingId === video.id ? "Deleting..." : "Delete"}
                        </button>
                      </div>
                    )}
                  </div>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>
        </div>
      </div>

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
