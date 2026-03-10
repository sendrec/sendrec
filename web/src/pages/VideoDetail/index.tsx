import { useEffect, useState } from "react";
import { Link, useParams, useNavigate } from "react-router-dom";
import { apiFetch } from "../../api/client";
import { useOrganization } from "../../hooks/useOrganization";
import { useUnsavedChanges } from "../../hooks/useUnsavedChanges";
import { useToast } from "../../hooks/useToast";
import { TrimModal } from "../../components/TrimModal";
import { FillerRemovalModal } from "../../components/FillerRemovalModal";
import { SilenceRemovalModal } from "../../components/SilenceRemovalModal";
import { Toast } from "../../components/Toast";
import { ConfirmDialog, ConfirmDialogState } from "../../components/ConfirmDialog";
import type { Video, Folder, Tag } from "../../types/video";
import { LimitsResponse } from "../../types/limits";
import { formatDuration, formatDate, expiryLabel } from "../../utils/format";
import { copyToClipboard } from "../../utils/clipboard";
import { SharingSection } from "./SharingSection";
import { TranscriptSection } from "./TranscriptSection";
import { CommentsSection } from "./CommentsSection";

interface PlaylistInfo {
  id: string;
  title: string;
  videoCount: number;
}

interface TranscriptSegment {
  start: number;
  end: number;
  text: string;
}

interface TranscriptResponse {
  status: string;
  segments: TranscriptSegment[];
}

interface Comment {
  id: string;
  authorName: string;
  body: string;
  isPrivate: boolean;
  isOwner: boolean;
  createdAt: string;
  videoTimestamp: number | null;
}

interface CommentsResponse {
  comments: Comment[];
  commentMode: string;
}


function viewCountLabel(viewCount: number, uniqueViewCount: number): string {
  if (viewCount === 0) {
    return "No views yet";
  }
  if (viewCount === uniqueViewCount) {
    return `${viewCount} view${viewCount !== 1 ? "s" : ""}`;
  }
  return `${viewCount} views (${uniqueViewCount} unique)`;
}

export function VideoDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { selectedOrg } = useOrganization();
  const isViewer = selectedOrg?.role === "viewer";

  const [video, setVideo] = useState<Video | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [limits, setLimits] = useState<LimitsResponse | null>(null);
  const [retranscribeLanguage, setRetranscribeLanguage] = useState("auto");
  const [folders, setFolders] = useState<Folder[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);
  const [playlists, setPlaylists] = useState<PlaylistInfo[]>([]);
  const [playlistSearch, setPlaylistSearch] = useState("");
  const [transcriptSegments, setTranscriptSegments] = useState<TranscriptSegment[]>([]);
  const [comments, setComments] = useState<Comment[]>([]);

  const toast = useToast();

  const [editingTitle, setEditingTitle] = useState(false);
  const [editTitle, setEditTitle] = useState("");

  const [ctaFormOpen, setCtaFormOpen] = useState(false);
  const [ctaText, setCtaText] = useState("");
  const [ctaUrl, setCtaUrl] = useState("");

  const [showTrimModal, setShowTrimModal] = useState(false);
  const [showFillerModal, setShowFillerModal] = useState(false);
  const [showSilenceModal, setShowSilenceModal] = useState(false);
  const [confirmDialog, setConfirmDialog] = useState<ConfirmDialogState | null>(null);

  const [videoUrl, setVideoUrl] = useState<string | null>(null);
  const [videoError, setVideoError] = useState(false);
  const titleIsDirty = editingTitle && video !== null && editTitle !== video.title;
  useUnsavedChanges(titleIsDirty);

  const [integrations, setIntegrations] = useState<{ provider: string }[]>([]);
  const [issueDropdownOpen, setIssueDropdownOpen] = useState(false);
  const [creatingIssue, setCreatingIssue] = useState(false);

  useEffect(() => {
    async function fetchData() {
      try {
        const [videos, limitsData, foldersData, tagsData, playlistsData] =
          await Promise.all([
            apiFetch<Video[]>("/api/videos"),
            apiFetch<LimitsResponse>("/api/videos/limits"),
            apiFetch<Folder[]>("/api/folders"),
            apiFetch<Tag[]>("/api/tags"),
            apiFetch<PlaylistInfo[]>("/api/playlists"),
          ]);
        const found = videos?.find((v) => v.id === id) ?? null;
        setVideo(found);
        if (found?.transcriptionLanguage) {
          setRetranscribeLanguage(found.transcriptionLanguage);
        }
        setLimits(limitsData ?? null);
        setFolders(foldersData ?? []);
        setTags(tagsData ?? []);
        setPlaylists(playlistsData ?? []);
        if (!found) {
          setNotFound(true);
        }

        if (found) {
          apiFetch<CommentsResponse>(`/api/videos/${found.id}/comments`)
            .then((data) => setComments(data?.comments ?? []))
            .catch(() => {});

          if (found.transcriptStatus === "ready") {
            apiFetch<TranscriptResponse>(`/api/videos/${found.id}/transcript`)
              .then((data) => setTranscriptSegments(data?.segments ?? []))
              .catch(() => {});
          }
        }

        try {
          const intgData = await apiFetch<{ provider: string }[]>(
            "/api/settings/integrations",
          );
          if (intgData) setIntegrations(intgData);
        } catch {
          /* integrations not configured — ok */
        }
      } catch {
        if (!video) {
          setNotFound(true);
        }
      } finally {
        setLoading(false);
      }
    }

    fetchData();
    apiFetch<{ downloadUrl: string }>(`/api/videos/${id}/download`)
      .then(resp => setVideoUrl(resp?.downloadUrl ?? null))
      .catch(() => {});
  }, [id]);

  async function createIssue(provider: string) {
    if (!video) return;
    setCreatingIssue(true);
    setIssueDropdownOpen(false);
    try {
      const result = await apiFetch<{ issueUrl: string; issueKey: string }>(
        `/api/videos/${video.id}/create-issue`,
        { method: "POST", body: JSON.stringify({ provider }) },
      );
      if (result) {
        toast.show(`Issue created: ${result.issueKey}`);
        window.open(result.issueUrl, "_blank", "noopener");
      }
    } catch (err) {
      toast.show(err instanceof Error ? err.message : "Failed to create issue");
    } finally {
      setCreatingIssue(false);
    }
  }

  useEffect(() => {
    if (!issueDropdownOpen) return;
    const close = () => setIssueDropdownOpen(false);
    document.addEventListener("click", close);
    return () => document.removeEventListener("click", close);
  }, [issueDropdownOpen]);

  async function refetchVideo() {
    const videos = await apiFetch<Video[]>("/api/videos");
    const found = videos?.find((v) => v.id === id) ?? null;
    if (found) {
      setVideo(found);
    }
  }

  async function copyLink() {
    if (!video) return;
    await copyToClipboard(video.shareUrl);
    toast.show("Link copied");
  }

  async function togglePin() {
    if (!video) return;
    const resp = await apiFetch<{ pinned: boolean }>(`/api/videos/${video.id}/pin`, {
      method: "PUT",
    });
    if (resp) {
      setVideo((prev) => (prev ? { ...prev, pinned: resp.pinned } : prev));
      toast.show(resp.pinned ? "Video pinned" : "Video unpinned");
    }
  }

  async function saveCTA() {
    if (!video) return;
    try {
      await apiFetch(`/api/videos/${video.id}/cta`, {
        method: "PUT",
        body: JSON.stringify({ text: ctaText, url: ctaUrl }),
      });
      setVideo((prev) =>
        prev ? { ...prev, ctaText, ctaUrl } : prev,
      );
      setCtaFormOpen(false);
      toast.show("CTA saved");
    } catch (err) {
      toast.show(err instanceof Error ? err.message : "Failed to save CTA");
    }
  }

  async function clearCTA() {
    if (!video) return;
    await apiFetch(`/api/videos/${video.id}/cta`, {
      method: "PUT",
      body: JSON.stringify({ text: null, url: null }),
    });
    setVideo((prev) =>
      prev ? { ...prev, ctaText: null, ctaUrl: null } : prev,
    );
    setCtaFormOpen(false);
    toast.show("CTA removed");
  }

  async function saveTitle() {
    if (!video) return;
    if (!editTitle.trim() || editTitle === video.title) {
      setEditingTitle(false);
      return;
    }
    await apiFetch(`/api/videos/${video.id}`, {
      method: "PATCH",
      body: JSON.stringify({ title: editTitle }),
    });
    setVideo((prev) => (prev ? { ...prev, title: editTitle } : prev));
    setEditingTitle(false);
  }

  useEffect(() => {
    if (
      video?.status === "processing" ||
      video?.documentStatus === "pending" ||
      video?.documentStatus === "processing"
    ) {
      const interval = setInterval(() => refetchVideo(), 3000);
      return () => clearInterval(interval);
    }
  }, [video?.status, video?.documentStatus]);

  async function acceptSuggestedTitle() {
    if (!video || !video.suggestedTitle) return;
    await apiFetch(`/api/videos/${video.id}`, {
      method: "PATCH",
      body: JSON.stringify({ title: video.suggestedTitle }),
    });
    await apiFetch(`/api/videos/${video.id}/dismiss-title`, { method: "PUT" });
    setVideo((prev) =>
      prev
        ? { ...prev, title: prev.suggestedTitle!, suggestedTitle: null }
        : prev,
    );
    toast.show("Title updated");
  }

  async function dismissSuggestedTitle() {
    if (!video) return;
    await apiFetch(`/api/videos/${video.id}/dismiss-title`, { method: "PUT" });
    setVideo((prev) =>
      prev ? { ...prev, suggestedTitle: null } : prev,
    );
  }

  async function moveToFolder(folderId: string | null) {
    if (!video) return;
    await apiFetch(`/api/videos/${video.id}/folder`, {
      method: "PUT",
      body: JSON.stringify({ folderId }),
    });
    setVideo((prev) => (prev ? { ...prev, folderId } : prev));
  }

  async function toggleVideoTag(tagId: string) {
    if (!video) return;
    const currentIds = video.tags.map((t) => t.id);
    const newIds = currentIds.includes(tagId)
      ? currentIds.filter((tid) => tid !== tagId)
      : [...currentIds, tagId];
    await apiFetch(`/api/videos/${video.id}/tags`, {
      method: "PUT",
      body: JSON.stringify({ tagIds: newIds }),
    });
    const matchingTags = tags
      .filter((t) => newIds.includes(t.id))
      .map((t) => ({ id: t.id, name: t.name, color: t.color }));
    setVideo((prev) => (prev ? { ...prev, tags: matchingTags } : prev));
  }

  async function togglePlaylist(playlistId: string) {
    if (!video) return;
    const isInPlaylist = (video.playlists ?? []).some((p) => p.id === playlistId);
    if (isInPlaylist) {
      await apiFetch(`/api/playlists/${playlistId}/videos/${video.id}`, {
        method: "DELETE",
      });
      setVideo((prev) =>
        prev
          ? { ...prev, playlists: (prev.playlists ?? []).filter((p) => p.id !== playlistId) }
          : prev,
      );
    } else {
      await apiFetch(`/api/playlists/${playlistId}/videos`, {
        method: "POST",
        body: JSON.stringify({ videoIds: [video.id] }),
      });
      const playlist = playlists.find((p) => p.id === playlistId);
      if (playlist) {
        setVideo((prev) =>
          prev
            ? { ...prev, playlists: [...(prev.playlists ?? []), { id: playlist.id, title: playlist.title }] }
            : prev,
        );
      }
    }
  }

  function deleteVideo() {
    if (!video) return;
    setConfirmDialog({
      message: "Delete this recording? This cannot be undone.",
      confirmLabel: "Delete",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        await apiFetch(`/api/videos/${video.id}`, { method: "DELETE" });
        navigate("/library");
      },
    });
  }

  async function handleDeleteComment(commentId: string) {
    if (!video) return;
    try {
      await apiFetch(`/api/videos/${video.id}/comments/${commentId}`, { method: "DELETE" });
      setComments((prev) => prev.filter((c) => c.id !== commentId));
    } catch {
      // ignore
    }
  }

  if (loading) {
    return (
      <div className="page-container">
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 24 }}>
          <div className="skeleton" style={{ height: 16, width: 80, borderRadius: 4 }} />
        </div>
        <div className="video-detail-hero">
          <div className="skeleton skeleton-thumb" style={{ aspectRatio: "16/9" }} />
          <div style={{ minWidth: 0, flex: 1 }}>
            <div className="skeleton skeleton-title" style={{ width: "60%", marginBottom: 12 }} />
            <div className="skeleton skeleton-meta" style={{ width: "40%" }} />
          </div>
        </div>
        <div style={{ display: "flex", gap: 8, marginTop: 16, marginBottom: 24 }}>
          <div className="skeleton skeleton-btn" />
          <div className="skeleton skeleton-btn" />
          <div className="skeleton skeleton-btn" />
        </div>
        <div className="skeleton-section">
          <div className="skeleton skeleton-section-title" />
          <div className="skeleton skeleton-row" />
          <div className="skeleton skeleton-row" />
        </div>
        <div className="skeleton-section">
          <div className="skeleton skeleton-section-title" />
          <div className="skeleton skeleton-row" />
        </div>
        <div className="skeleton-section">
          <div className="skeleton skeleton-section-title" />
          <div className="skeleton skeleton-row" />
          <div className="skeleton skeleton-row" />
          <div className="skeleton skeleton-row" />
        </div>
      </div>
    );
  }

  if (notFound || !video) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16 }}>
          Video not found
        </p>
        <Link
          to="/library"
          style={{
            color: "var(--color-accent)",
            textDecoration: "none",
            fontSize: 14,
            marginTop: 8,
          }}
        >
          Back to Library
        </Link>
      </div>
    );
  }

  const expiry = expiryLabel(video.shareExpiresAt);

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
        <Link to="/library" className="back-link">
          &larr; Library
        </Link>
        <a
          href={`/watch/${video.shareToken}`}
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
      </div>

      <div className="video-detail-hero">
        <div style={{ position: "relative" }}>
          {videoUrl ? (
            videoError ? (
              <div className="video-detail-thumbnail video-error-placeholder">
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-secondary)" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <polygon points="23 7 16 12 23 17 23 7" />
                  <rect x="1" y="5" width="15" height="14" rx="2" ry="2" />
                  <line x1="1" y1="1" x2="23" y2="23" />
                </svg>
                <p style={{ color: "var(--color-text-secondary)", fontSize: 14, marginTop: 12 }}>
                  Video failed to load
                </p>
              </div>
            ) : (
              <video
                src={videoUrl}
                controls
                className="video-detail-thumbnail"
                poster={video.thumbnailUrl}
                onError={() => setVideoError(true)}
              />
            )
          ) : video.thumbnailUrl ? (
            <img
              src={video.thumbnailUrl}
              alt="Video thumbnail"
              className="video-detail-thumbnail"
            />
          ) : (
            <div className="video-detail-thumbnail video-thumbnail-placeholder">
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-secondary)" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <polygon points="23 7 16 12 23 17 23 7" />
                <rect x="1" y="5" width="15" height="14" rx="2" ry="2" />
              </svg>
            </div>
          )}
          {video.status === "processing" && (
            <div className="hero-processing-overlay">
              <p className="hero-processing-pulse">Processing video...</p>
              <p className="hero-processing-sub">This usually takes a minute or two</p>
            </div>
          )}
        </div>

        <div style={{ minWidth: 0, flex: 1 }}>
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
                  {video.title}
                </h1>
                {!isViewer && (
                  <button
                    onClick={() => {
                      setEditingTitle(true);
                      setEditTitle(video.title);
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
                )}
              </>
            )}
          </div>

          <p className="video-detail-meta">
            <span>{formatDuration(video.duration)}</span>
            <span>&middot;</span>
            <span>{formatDate(video.createdAt)}</span>
            <span>&middot;</span>
            <span>{viewCountLabel(video.viewCount, video.uniqueViewCount)}</span>
            <span>&middot;</span>
            <span
              style={{
                color:
                  video.shareExpiresAt === null
                    ? "var(--color-accent)"
                    : expiry.expired
                      ? "var(--color-error)"
                      : undefined,
              }}
            >
              {expiry.text}
            </span>
            {video.noiseReduction && (
              <>
                <span>&middot;</span>
                <span style={{ color: "var(--color-accent)" }}>Noise reduced</span>
              </>
            )}
            {video.pinned && (
              <>
                <span>&middot;</span>
                <span style={{ color: "var(--color-accent)" }}>Pinned</span>
              </>
            )}
          </p>

          {video.status === "processing" && (
            <span className="status-badge status-badge--processing">
              <span className="status-badge-dot" />
              Processing
            </span>
          )}

          {video.tags.length > 0 && (
            <div
              style={{
                display: "flex",
                flexWrap: "wrap",
                gap: 4,
                marginTop: 8,
              }}
            >
              {video.tags.map((tag) => (
                <span
                  key={tag.id}
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 4,
                    padding: "2px 8px",
                    borderRadius: 12,
                    fontSize: 11,
                    fontWeight: 500,
                    background: "var(--color-bg)",
                    border: "1px solid var(--color-border)",
                    color: "var(--color-text-secondary)",
                  }}
                >
                  <span
                    style={{
                      width: 6,
                      height: 6,
                      borderRadius: "50%",
                      background:
                        tag.color ?? "var(--color-text-secondary)",
                    }}
                  />
                  {tag.name}
                </span>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Primary Actions Bar */}
      <div className="detail-actions">
        <button
          className="detail-btn detail-btn--accent"
          onClick={copyLink}
          disabled={video.status === "processing"}
          style={{ opacity: video.status === "processing" ? 0.5 : undefined }}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg>
          Copy share link
        </button>
        <button
          className="detail-btn"
          onClick={() => navigate(`/videos/${video.id}/analytics`)}
          disabled={video.status === "processing"}
          style={{ opacity: video.status === "processing" ? 0.5 : undefined }}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M18 20V10"/><path d="M12 20V4"/><path d="M6 20v-6"/></svg>
          View analytics
        </button>
        {video.status === "ready" && videoUrl && (
          <a href={videoUrl} download className="detail-btn" style={{ textDecoration: "none" }}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
            Download
          </a>
        )}
        {!isViewer && (
          <button
            className="detail-btn"
            onClick={togglePin}
            aria-label={video.pinned ? "Unpin video" : "Pin video"}
          >
            <svg viewBox="0 0 24 24" fill={video.pinned ? "currentColor" : "none"} stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="12" y1="17" x2="12" y2="22"/><path d="M5 17h14v-1.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V6h-6v4.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24Z"/><line x1="14" y1="6" x2="14" y2="2"/><line x1="10" y1="6" x2="10" y2="2"/></svg>
            {video.pinned ? "Unpin" : "Pin"}
          </button>
        )}
        {!isViewer && integrations.length > 0 &&
          (integrations.length === 1 ? (
            <button
              onClick={() => createIssue(integrations[0].provider)}
              className="detail-btn"
              disabled={creatingIssue}
            >
              <svg
                width="16"
                height="16"
                viewBox="0 0 16 16"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <circle cx="8" cy="8" r="6" />
                <line x1="8" y1="5" x2="8" y2="11" />
                <line x1="5" y1="8" x2="11" y2="8" />
              </svg>
              {creatingIssue
                ? "Creating..."
                : `Create ${integrations[0].provider === "github" ? "GitHub" : "Jira"} Issue`}
            </button>
          ) : (
            <div style={{ position: "relative" }}>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  setIssueDropdownOpen(!issueDropdownOpen);
                }}
                className="detail-btn"
                disabled={creatingIssue}
              >
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 16 16"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                >
                  <circle cx="8" cy="8" r="6" />
                  <line x1="8" y1="5" x2="8" y2="11" />
                  <line x1="5" y1="8" x2="11" y2="8" />
                </svg>
                {creatingIssue ? "Creating..." : "Create Issue"}
              </button>
              {issueDropdownOpen && (
                <div
                  style={{
                    position: "absolute",
                    top: "100%",
                    left: 0,
                    zIndex: 10,
                    background: "var(--color-surface, #fff)",
                    border: "1px solid var(--color-border, #ddd)",
                    borderRadius: "0.5rem",
                    marginTop: "0.25rem",
                    minWidth: "160px",
                    boxShadow: "0 4px 12px rgba(0,0,0,0.1)",
                  }}
                >
                  {integrations.map((intg) => (
                    <button
                      key={intg.provider}
                      onClick={() => createIssue(intg.provider)}
                      style={{
                        display: "block",
                        width: "100%",
                        padding: "0.5rem 1rem",
                        border: "none",
                        background: "none",
                        cursor: "pointer",
                        textAlign: "left",
                        fontSize: "0.875rem",
                        color: "inherit",
                      }}
                    >
                      {intg.provider === "github"
                        ? "GitHub Issue"
                        : "Jira Issue"}
                    </button>
                  ))}
                </div>
              )}
            </div>
          ))}
      </div>

      {/* Share Settings */}
      <SharingSection
        video={video}
        limits={limits}
        isViewer={isViewer}
        onVideoUpdate={setVideo}
        onRefetchVideo={refetchVideo}
      />

      {/* AI */}
      <TranscriptSection
        video={video}
        limits={limits}
        isViewer={isViewer}
        transcriptSegments={transcriptSegments}
        retranscribeLanguage={retranscribeLanguage}
        onRetranscribeLanguageChange={setRetranscribeLanguage}
        onVideoUpdate={setVideo}
        onTranscriptClear={() => setTranscriptSegments([])}
      />

      {/* Editing */}
      {!isViewer && <div className="video-detail-section">
        <h2 className="video-detail-section-title">Editing</h2>

        {video.suggestedTitle && (
          <div className="detail-setting-row">
            <span className="detail-setting-label">Suggested title</span>
            <div className="detail-setting-value">
              <span
                style={{
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                  minWidth: 0,
                }}
              >
                {video.suggestedTitle}
              </span>
              <button
                onClick={acceptSuggestedTitle}
                className="detail-btn detail-btn--accent"
              >
                Accept
              </button>
              <button onClick={dismissSuggestedTitle} className="detail-btn">
                Dismiss
              </button>
            </div>
          </div>
        )}

        <div className="detail-setting-row">
          <span className="detail-setting-label">Trim</span>
          <button
            onClick={() => setShowTrimModal(true)}
            className="detail-btn"
            disabled={video.status === "processing"}
          >
            Trim video
          </button>
        </div>

        {video.status === "ready" && (
          <div className="detail-setting-row">
            <span className="detail-setting-label">Silence</span>
            <button
              onClick={() => setShowSilenceModal(true)}
              className="detail-btn"
            >
              Remove silence
            </button>
          </div>
        )}

        {video.transcriptStatus === "ready" && (
          <div className="detail-setting-row">
            <span className="detail-setting-label">Fillers</span>
            <button
              onClick={() => setShowFillerModal(true)}
              className="detail-btn"
              disabled={video.status === "processing"}
            >
              Remove fillers
            </button>
          </div>
        )}
      </div>}

      {/* Organize */}
      {!isViewer && <div className="video-detail-section">
        <h2 className="video-detail-section-title">Organize</h2>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Folder</span>
          <select
            aria-label="Folder"
            value={video.folderId ?? ""}
            onChange={(e) => moveToFolder(e.target.value || null)}
            style={{
              background: "var(--color-surface)",
              border: "1px solid var(--color-border)",
              borderRadius: 4,
              color: video.folderId
                ? "var(--color-accent)"
                : "var(--color-text-secondary)",
              fontSize: 13,
              padding: "4px 8px",
              cursor: "pointer",
            }}
          >
            <option value="">None</option>
            {folders.map((f) => (
              <option key={f.id} value={f.id}>
                {f.name}
              </option>
            ))}
          </select>
        </div>

        {tags.length > 0 && (
          <div className="detail-setting-row">
            <span className="detail-setting-label">Tags</span>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
              {tags.map((tag) => {
                const active = video.tags.some((vt) => vt.id === tag.id);
                return (
                  <button
                    key={tag.id}
                    onClick={() => toggleVideoTag(tag.id)}
                    aria-label={`Tag ${tag.name}`}
                    style={{
                      display: "inline-flex",
                      alignItems: "center",
                      gap: 4,
                      padding: "2px 8px",
                      borderRadius: 12,
                      fontSize: 11,
                      fontWeight: 500,
                      background: active
                        ? "var(--color-accent)"
                        : "var(--color-bg)",
                      color: active
                        ? "var(--color-on-accent)"
                        : "var(--color-text-secondary)",
                      border: active
                        ? "1px solid var(--color-accent)"
                        : "1px solid var(--color-border)",
                      cursor: "pointer",
                    }}
                  >
                    <span
                      style={{
                        width: 6,
                        height: 6,
                        borderRadius: "50%",
                        background:
                          tag.color ?? "var(--color-text-secondary)",
                      }}
                    />
                    {tag.name}
                  </button>
                );
              })}
            </div>
          </div>
        )}

        {playlists.length > 0 && (
          <div className="detail-setting-row" style={{ flexDirection: "column", alignItems: "stretch", gap: 8 }}>
            <span className="detail-setting-label">Playlists</span>
            {playlists.length > 5 && (
              <input
                type="text"
                value={playlistSearch}
                onChange={(e) => setPlaylistSearch(e.target.value)}
                placeholder="Search playlists..."
                style={{
                  padding: "4px 8px",
                  fontSize: 13,
                  background: "var(--color-surface)",
                  border: "1px solid var(--color-border)",
                  borderRadius: 4,
                  color: "var(--color-text)",
                }}
              />
            )}
            <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
              {playlists
                .filter((p) => !playlistSearch || p.title.toLowerCase().includes(playlistSearch.toLowerCase()))
                .map((playlist) => {
                  const active = (video.playlists ?? []).some((vp) => vp.id === playlist.id);
                  return (
                    <button
                      key={playlist.id}
                      onClick={() => togglePlaylist(playlist.id)}
                      aria-label={`Playlist ${playlist.title}`}
                      style={{
                        display: "inline-flex",
                        alignItems: "center",
                        padding: "2px 8px",
                        borderRadius: 12,
                        fontSize: 11,
                        fontWeight: 500,
                        background: active ? "var(--color-accent)" : "var(--color-bg)",
                        color: active ? "var(--color-on-accent)" : "var(--color-text-secondary)",
                        border: active ? "1px solid var(--color-accent)" : "1px solid var(--color-border)",
                        cursor: "pointer",
                      }}
                    >
                      {playlist.title}
                    </button>
                  );
                })}
            </div>
          </div>
        )}
      </div>}

      {/* Call to Action */}
      {!isViewer && <div className="video-detail-section">
        <h2 className="video-detail-section-title">Call to Action</h2>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Call to action</span>
          <div className="detail-setting-value">
            <span>{video.ctaText ?? "None"}</span>
            <button
              onClick={() => {
                setCtaFormOpen(true);
                setCtaText(video.ctaText ?? "");
                setCtaUrl(video.ctaUrl ?? "https://");
              }}
              className="detail-btn"
            >
              {video.ctaText ? "Edit CTA" : "Add CTA"}
            </button>
          </div>
        </div>

        {ctaFormOpen && (
          <div
            style={{
              padding: 12,
              background: "var(--color-surface)",
              borderRadius: 8,
              border: "1px solid var(--color-border)",
              marginTop: 8,
            }}
          >
            <input
              type="text"
              placeholder="Button text (e.g. Book a demo)"
              value={ctaText}
              onChange={(e) => setCtaText(e.target.value)}
              maxLength={100}
              aria-label="CTA text"
              style={{
                width: "100%",
                padding: "8px 10px",
                marginBottom: 8,
                background: "var(--color-bg)",
                border: "1px solid var(--color-border)",
                borderRadius: 6,
                color: "var(--color-text)",
                fontSize: 13,
              }}
            />
            <input
              type="url"
              placeholder="URL (e.g. https://example.com/demo)"
              value={ctaUrl}
              onChange={(e) => setCtaUrl(e.target.value)}
              maxLength={2000}
              aria-label="CTA URL"
              style={{
                width: "100%",
                padding: "8px 10px",
                marginBottom: 8,
                background: "var(--color-bg)",
                border: "1px solid var(--color-border)",
                borderRadius: 6,
                color: "var(--color-text)",
                fontSize: 13,
              }}
            />
            <div style={{ display: "flex", gap: 8 }}>
              <button
                onClick={saveCTA}
                disabled={!ctaText.trim() || !ctaUrl.trim()}
                className="detail-btn detail-btn--accent"
              >
                Save
              </button>
              {video.ctaText && (
                <button
                  onClick={clearCTA}
                  className="detail-btn detail-btn--danger"
                >
                  Remove
                </button>
              )}
              <button
                onClick={() => setCtaFormOpen(false)}
                className="detail-btn"
              >
                Cancel
              </button>
            </div>
          </div>
        )}
      </div>}

      {/* Comments */}
      <CommentsSection
        comments={comments}
        isViewer={isViewer}
        onDeleteComment={handleDeleteComment}
      />

      {/* Danger Zone */}
      {!isViewer && (
        <div className="danger-zone">
          <button
            onClick={deleteVideo}
            className="detail-btn detail-btn--danger"
            style={{ padding: "8px 20px" }}
          >
            Delete video
          </button>
        </div>
      )}

      {/* Trim Modal */}
      {showTrimModal && (
        <TrimModal
          videoId={video.id}
          shareToken={video.shareToken}
          duration={video.duration}
          onClose={() => setShowTrimModal(false)}
          onTrimStarted={() => {
            setVideo((prev) =>
              prev ? { ...prev, status: "processing" } : prev,
            );
            setShowTrimModal(false);
          }}
        />
      )}

      {/* Filler Removal Modal */}
      {showFillerModal && (
        <FillerRemovalModal
          videoId={video.id}
          shareToken={video.shareToken}
          duration={video.duration}
          onClose={() => setShowFillerModal(false)}
          onRemovalStarted={() => {
            setVideo((prev) =>
              prev ? { ...prev, status: "processing" } : prev,
            );
            setShowFillerModal(false);
            toast.show("Removing filler words...");
          }}
        />
      )}

      {/* Silence Removal Modal */}
      {showSilenceModal && (
        <SilenceRemovalModal
          videoId={video.id}
          shareToken={video.shareToken}
          duration={video.duration}
          onClose={() => setShowSilenceModal(false)}
          onRemovalStarted={() => {
            setVideo((prev) =>
              prev ? { ...prev, status: "processing" } : prev,
            );
            setShowSilenceModal(false);
            toast.show("Removing silent pauses...");
          }}
        />
      )}

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
    </div>
  );
}
