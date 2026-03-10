import { type RefObject } from "react";
import { Link } from "react-router-dom";
import type { Video, Folder } from "../../types/video";
import type { LimitsResponse } from "../../types/limits";
import { formatDuration, formatDate, expiryLabel } from "../../utils/format";

interface VideoGridProps {
  sortedVideos: Video[];
  videos: Video[];
  limits: LimitsResponse | null;
  folders: Folder[];
  isViewer: boolean;
  searchQuery: string;
  onSearchChange: (value: string) => void;
  sortBy: "newest" | "oldest" | "most-viewed" | "title";
  onSortByChange: (value: "newest" | "oldest" | "most-viewed" | "title") => void;
  viewMode: "grid" | "list";
  onSetViewMode: (mode: "grid" | "list") => void;
  selectedIds: Set<string>;
  onToggleSelect: (id: string) => void;
  onSelectAll: () => void;
  onDeselectAll: () => void;
  batchLoading: boolean;
  onBatchDelete: () => void;
  onBatchSetFolder: (folderId: string | null) => void;
  openMenuId: string | null;
  onSetOpenMenuId: (id: string | null) => void;
  menuRef: RefObject<HTMLDivElement | null>;
  deletingId: string | null;
  downloadingId: string | null;
  onDeleteVideo: (id: string) => void;
  onCopyLink: (shareUrl: string) => void;
  onTogglePin: (id: string) => void;
  onDownloadVideo: (id: string) => void;
  onSetTransferVideoId: (id: string) => void;
}

export function VideoGrid({
  sortedVideos,
  videos,
  limits,
  folders,
  isViewer,
  searchQuery,
  onSearchChange,
  sortBy,
  onSortByChange,
  viewMode,
  onSetViewMode,
  selectedIds,
  onToggleSelect,
  onSelectAll,
  onDeselectAll,
  batchLoading,
  onBatchDelete,
  onBatchSetFolder,
  openMenuId,
  onSetOpenMenuId,
  menuRef,
  deletingId,
  downloadingId,
  onDeleteVideo,
  onCopyLink,
  onTogglePin,
  onDownloadVideo,
  onSetTransferVideoId,
}: VideoGridProps) {
  return (
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
          onChange={(e) => onSearchChange(e.target.value)}
          className="library-search"
        />
        {!isViewer && (
          <Link
            to="/"
            className="detail-btn detail-btn--accent"
            style={{ textDecoration: "none", whiteSpace: "nowrap" }}
          >
            New Recording
          </Link>
        )}
      </div>

      <div className="view-controls">
        <select
          className="sort-select"
          value={sortBy}
          onChange={(e) => onSortByChange(e.target.value as typeof sortBy)}
          aria-label="Sort videos"
        >
          <option value="newest">Newest first</option>
          <option value="oldest">Oldest first</option>
          <option value="most-viewed">Most viewed</option>
          <option value="title">Title A-Z</option>
        </select>
        <div className="view-toggle">
          <button
            className={`view-toggle-btn${viewMode === "grid" ? " view-toggle-btn--active" : ""}`}
            onClick={() => onSetViewMode("grid")}
            aria-label="Grid view"
          >
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><rect x="1" y="1" width="6" height="6" rx="1" stroke="currentColor" strokeWidth="1.5"/><rect x="9" y="1" width="6" height="6" rx="1" stroke="currentColor" strokeWidth="1.5"/><rect x="1" y="9" width="6" height="6" rx="1" stroke="currentColor" strokeWidth="1.5"/><rect x="9" y="9" width="6" height="6" rx="1" stroke="currentColor" strokeWidth="1.5"/></svg>
          </button>
          <button
            className={`view-toggle-btn${viewMode === "list" ? " view-toggle-btn--active" : ""}`}
            onClick={() => onSetViewMode("list")}
            aria-label="List view"
          >
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><rect x="1" y="2" width="14" height="3" rx="1" stroke="currentColor" strokeWidth="1.5"/><rect x="1" y="7" width="14" height="3" rx="1" stroke="currentColor" strokeWidth="1.5"/><rect x="1" y="12" width="14" height="3" rx="1" stroke="currentColor" strokeWidth="1.5"/></svg>
          </button>
        </div>
      </div>

      {selectedIds.size > 0 && !isViewer && (
        <div className="batch-toolbar">
          <span style={{ fontWeight: 600, fontSize: 14 }}>
            {selectedIds.size} selected
          </span>
          <button onClick={onSelectAll} className="detail-btn">Select all</button>
          <button onClick={onDeselectAll} className="detail-btn">Deselect all</button>
          <select
            onChange={(e) => {
              const val = e.target.value;
              onBatchSetFolder(val === "__none__" ? null : val);
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
          <button onClick={onBatchDelete} className="detail-btn detail-btn--danger" disabled={batchLoading}>
            Delete
          </button>
        </div>
      )}

      <div className={viewMode === "grid" ? "video-grid" : "video-list"}>
        {sortedVideos.map((video) => (
          <div key={video.id} className="video-card" style={openMenuId === video.id ? { zIndex: 10 } : undefined}>
            <Link to={`/videos/${video.id}`} state={{ video }} style={{ textDecoration: "none" }}>
              <div className="video-card-thumbnail">
                {video.thumbnailUrl && (
                  <img src={video.thumbnailUrl} alt="" />
                )}
                <div className="video-card-play" />
                {video.status === "ready" && (
                  <span className="video-card-duration">{formatDuration(video.duration)}</span>
                )}
                {(video.status === "uploading" || video.status === "processing") && (
                  <span className="video-card-status">
                    <span className="status-dot" />
                    {video.status === "uploading" ? "uploading..." : "processing..."}
                  </span>
                )}
                {video.pinned && (
                  <span
                    className="video-card-pin"
                    title="Pinned"
                    style={{
                      position: "absolute",
                      top: 6,
                      left: 6,
                      background: "rgba(0,0,0,0.6)",
                      borderRadius: 4,
                      padding: "2px 4px",
                      display: "flex",
                      alignItems: "center",
                    }}
                  >
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ color: "var(--color-accent)" }}><line x1="12" y1="17" x2="12" y2="22"/><path d="M5 17h14v-1.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V6h-6v4.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24Z"/><line x1="14" y1="6" x2="14" y2="2"/><line x1="10" y1="6" x2="10" y2="2"/></svg>
                  </span>
                )}
              </div>
            </Link>
            <input
              type="checkbox"
              checked={selectedIds.has(video.id)}
              onChange={() => onToggleSelect(video.id)}
              className={`video-select-checkbox${selectedIds.size > 0 ? " video-select-checkbox--visible" : ""}`}
              aria-label={`Select ${video.title}`}
              onClick={(e) => e.stopPropagation()}
            />
            <div className="video-card-body">
              <Link
                to={`/videos/${video.id}`}
                state={{ video }}
                className="video-card-title"
              >
                {video.title}
              </Link>
              <div className="video-card-meta">
                <span>{formatDate(video.createdAt)}</span>
                {video.status === "ready" && video.viewCount > 0 && (
                  <>
                    <span className="video-card-meta-dot" />
                    <span>
                      {video.viewCount === video.uniqueViewCount
                        ? `${video.viewCount} view${video.viewCount !== 1 ? "s" : ""}`
                        : `${video.viewCount} views (${video.uniqueViewCount} unique)`}
                    </span>
                  </>
                )}
                {video.status === "ready" && video.viewCount === 0 && (
                  <>
                    <span className="video-card-meta-dot" />
                    <span style={{ opacity: 0.6 }}>No views yet</span>
                  </>
                )}
                {video.transcriptStatus === "no_audio" && (
                  <>
                    <span className="video-card-meta-dot" />
                    <span style={{ opacity: 0.7 }}>No audio</span>
                  </>
                )}
                {video.status === "ready" && (() => {
                  const expiry = expiryLabel(video.shareExpiresAt);
                  return (
                    <>
                      <span className="video-card-meta-dot" />
                      <span style={{ color: video.shareExpiresAt === null ? "var(--color-accent)" : expiry.expired ? "var(--color-error)" : undefined }}>
                        {expiry.text}
                      </span>
                    </>
                  );
                })()}
              </div>
              {video.tags.length > 0 && (
                <div className="video-card-tags">
                  {video.tags.map((tag) => (
                    <span key={tag.id} className="video-tag">
                      <span className="video-tag-dot" style={{ background: tag.color ?? "var(--color-text-secondary)" }} />
                      {tag.name}
                    </span>
                  ))}
                </div>
              )}
            </div>
            {video.status === "ready" && (
              <div className="video-card-actions">
                <button
                  onClick={() => onCopyLink(video.shareUrl)}
                  className="card-action-btn"
                >
                  Copy link
                </button>
                <span className="card-action-spacer" />
                <div style={{ position: "relative" }} ref={openMenuId === video.id ? menuRef : undefined}>
                  <button
                    onClick={() => onSetOpenMenuId(openMenuId === video.id ? null : video.id)}
                    className="card-action-btn"
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
                      {!isViewer && (
                        <button
                          onClick={() => { onTogglePin(video.id); onSetOpenMenuId(null); }}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px" }}
                        >
                          {video.pinned ? "Unpin" : "Pin"}
                        </button>
                      )}
                      <Link
                        to={`/videos/${video.id}/analytics`}
                        className="action-link"
                        style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px", textDecoration: "none" }}
                        onClick={() => onSetOpenMenuId(null)}
                      >
                        Analytics
                      </Link>
                      <button
                        onClick={() => { onDownloadVideo(video.id); onSetOpenMenuId(null); }}
                        disabled={downloadingId === video.id}
                        className="action-link"
                        style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px", opacity: downloadingId === video.id ? 0.5 : undefined }}
                      >
                        {downloadingId === video.id ? "Downloading..." : "Download"}
                      </button>
                      {!isViewer && (
                        <button
                          onClick={() => { onSetTransferVideoId(video.id); onSetOpenMenuId(null); }}
                          className="action-link"
                          style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px" }}
                        >
                          Move to...
                        </button>
                      )}
                      {!isViewer && (
                        <>
                          <div style={{ borderTop: "1px solid var(--color-border)", margin: "4px 0" }} />
                          <button
                            onClick={() => { onDeleteVideo(video.id); onSetOpenMenuId(null); }}
                            disabled={deletingId === video.id}
                            className="action-link"
                            style={{ display: "block", width: "100%", textAlign: "left", padding: "6px 12px", color: "var(--color-error)", opacity: deletingId === video.id ? 0.5 : undefined }}
                          >
                            {deletingId === video.id ? "Deleting..." : "Delete"}
                          </button>
                        </>
                      )}
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
