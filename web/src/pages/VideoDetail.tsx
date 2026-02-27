import { useEffect, useRef, useState } from "react";
import { Link, useParams, useNavigate } from "react-router-dom";
import { apiFetch } from "../api/client";
import { useUnsavedChanges } from "../hooks/useUnsavedChanges";
import { TrimModal } from "../components/TrimModal";
import { FillerRemovalModal } from "../components/FillerRemovalModal";
import { SilenceRemovalModal } from "../components/SilenceRemovalModal";
import { DocumentModal } from "../components/DocumentModal";
import { TRANSCRIPTION_LANGUAGES } from "../constants/languages";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { PromptDialog } from "../components/PromptDialog";

interface VideoTag {
  id: string;
  name: string;
  color: string | null;
}

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
  document?: string;
  documentStatus: string;
  folderId: string | null;
  transcriptionLanguage: string | null;
  tags: VideoTag[];
  playlists: { id: string; title: string }[];
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

interface PlaylistInfo {
  id: string;
  title: string;
  videoCount: number;
}

interface LimitsResponse {
  maxVideosPerMonth: number;
  maxVideoDurationSeconds: number;
  videosUsedThisMonth: number;
  brandingEnabled: boolean;
  aiEnabled: boolean;
  transcriptionEnabled: boolean;
}

interface VideoBranding {
  companyName: string | null;
  colorBackground: string | null;
  colorSurface: string | null;
  colorText: string | null;
  colorAccent: string | null;
  footerText: string | null;
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

function formatDuration(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = Math.floor(seconds % 60);
  return `${minutes}:${String(remainingSeconds).padStart(2, "0")}`;
}

function formatDate(isoDate: string): string {
  return new Date(isoDate).toLocaleDateString("en-GB");
}

function expiryLabel(shareExpiresAt: string | null): {
  text: string;
  expired: boolean;
} {
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

function viewCountLabel(viewCount: number, uniqueViewCount: number): string {
  if (viewCount === 0) {
    return "No views yet";
  }
  if (viewCount === uniqueViewCount) {
    return `${viewCount} view${viewCount !== 1 ? "s" : ""}`;
  }
  return `${viewCount} views (${uniqueViewCount} unique)`;
}

function formatTimestamp(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, "0")}`;
}

function getInitials(name: string): string {
  if (!name) return "?";
  return name.split(" ").map((w) => w[0]).join("").toUpperCase().slice(0, 2);
}

function relativeTime(isoDate: string): string {
  const diff = Date.now() - new Date(isoDate).getTime();
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(isoDate).toLocaleDateString("en-GB");
}

export function VideoDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

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

  const [toast, setToast] = useState<string | null>(null);
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [editingTitle, setEditingTitle] = useState(false);
  const [editTitle, setEditTitle] = useState("");

  const [ctaFormOpen, setCtaFormOpen] = useState(false);
  const [ctaText, setCtaText] = useState("");
  const [ctaUrl, setCtaUrl] = useState("");

  const [showTrimModal, setShowTrimModal] = useState(false);
  const [showFillerModal, setShowFillerModal] = useState(false);
  const [showSilenceModal, setShowSilenceModal] = useState(false);
  const [showDocumentModal, setShowDocumentModal] = useState(false);
  const [documentContent, setDocumentContent] = useState<string | null>(null);
  const [confirmDialog, setConfirmDialog] = useState<{
    message: string;
    onConfirm: () => void;
    confirmLabel?: string;
    danger?: boolean;
  } | null>(null);
  const [promptDialog, setPromptDialog] = useState<{
    title: string;
    onSubmit: (value: string) => void;
    placeholder?: string;
    submitLabel?: string;
  } | null>(null);

  const [videoUrl, setVideoUrl] = useState<string | null>(null);
  const [videoError, setVideoError] = useState(false);
  const [uploadingThumbnail, setUploadingThumbnail] = useState(false);

  const titleIsDirty = editingTitle && video !== null && editTitle !== video.title;
  useUnsavedChanges(titleIsDirty);

  const [brandingOpen, setBrandingOpen] = useState(false);
  const [videoBranding, setVideoBranding] = useState<VideoBranding>({
    companyName: null,
    colorBackground: null,
    colorSurface: null,
    colorText: null,
    colorAccent: null,
    footerText: null,
  });
  const [savingBranding, setSavingBranding] = useState(false);
  const [brandingMessage, setBrandingMessage] = useState("");

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

  function showToast(message: string) {
    if (toastTimer.current) clearTimeout(toastTimer.current);
    setToast(message);
    toastTimer.current = setTimeout(() => setToast(null), 2000);
  }

  async function refetchVideo() {
    const videos = await apiFetch<Video[]>("/api/videos");
    const found = videos?.find((v) => v.id === id) ?? null;
    if (found) {
      setVideo(found);
    }
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

  async function copyLink() {
    if (!video) return;
    await copyToClipboard(video.shareUrl);
    showToast("Link copied");
  }

  async function copyEmbed() {
    if (!video) return;
    const snippet = `<iframe src="${window.location.origin}/embed/${video.shareToken}" width="640" height="360" frameborder="0" allowfullscreen></iframe>`;
    await copyToClipboard(snippet);
    showToast("Embed code copied");
  }

  async function toggleDownload() {
    if (!video) return;
    const newValue = !video.downloadEnabled;
    await apiFetch(`/api/videos/${video.id}/download-enabled`, {
      method: "PUT",
      body: JSON.stringify({ downloadEnabled: newValue }),
    });
    setVideo((prev) =>
      prev ? { ...prev, downloadEnabled: newValue } : prev,
    );
  }

  async function toggleEmailGate() {
    if (!video) return;
    const newValue = !video.emailGateEnabled;
    await apiFetch(`/api/videos/${video.id}/email-gate`, {
      method: "PUT",
      body: JSON.stringify({ enabled: newValue }),
    });
    setVideo((prev) =>
      prev ? { ...prev, emailGateEnabled: newValue } : prev,
    );
  }

  async function toggleLinkExpiry() {
    if (!video) return;
    const neverExpires = video.shareExpiresAt !== null;
    await apiFetch(`/api/videos/${video.id}/link-expiry`, {
      method: "PUT",
      body: JSON.stringify({ neverExpires }),
    });
    await refetchVideo();
  }

  async function extendVideo() {
    if (!video) return;
    await apiFetch(`/api/videos/${video.id}/extend`, { method: "POST" });
    await refetchVideo();
    showToast("Link extended");
  }

  function addPassword() {
    if (!video) return;
    setPromptDialog({
      title: "Enter a password for this video:",
      placeholder: "Password",
      submitLabel: "Set password",
      onSubmit: async (password) => {
        setPromptDialog(null);
        await apiFetch(`/api/videos/${video.id}/password`, {
          method: "PUT",
          body: JSON.stringify({ password }),
        });
        setVideo((prev) => (prev ? { ...prev, hasPassword: true } : prev));
      },
    });
  }

  function removePassword() {
    if (!video) return;
    setConfirmDialog({
      message: "Remove the password from this video?",
      confirmLabel: "Remove",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        await apiFetch(`/api/videos/${video.id}/password`, {
          method: "PUT",
          body: JSON.stringify({ password: "" }),
        });
        setVideo((prev) => (prev ? { ...prev, hasPassword: false } : prev));
      },
    });
  }

  async function changeCommentMode(mode: string) {
    if (!video) return;
    try {
      await apiFetch(`/api/videos/${video.id}/comment-mode`, {
        method: "PUT",
        body: JSON.stringify({ commentMode: mode }),
      });
      setVideo((prev) => (prev ? { ...prev, commentMode: mode } : prev));
    } catch {
      // select stays at previous value
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
      showToast("CTA saved");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Failed to save CTA");
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
    showToast("CTA removed");
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

  async function retranscribe() {
    if (!video) return;
    const body = retranscribeLanguage !== "auto" ? { language: retranscribeLanguage } : undefined;
    await apiFetch(`/api/videos/${video.id}/retranscribe`, {
      method: "POST",
      ...(body && { body: JSON.stringify(body) }),
    });
    setVideo((prev) =>
      prev ? { ...prev, transcriptStatus: "pending" } : prev,
    );
    setTranscriptSegments([]);
    showToast("Transcription queued");
  }

  async function summarize() {
    if (!video) return;
    await apiFetch(`/api/videos/${video.id}/summarize`, { method: "POST" });
    setVideo((prev) =>
      prev ? { ...prev, summaryStatus: "pending" } : prev,
    );
    showToast("Summary queued");
  }

  async function viewDocument() {
    if (!video) return;
    const data = await apiFetch<{ document?: string }>(`/api/watch/${video.shareToken}`);
    if (data?.document) {
      setDocumentContent(data.document);
      setShowDocumentModal(true);
    }
  }

  async function generateDocument() {
    if (!video) return;
    await apiFetch(`/api/videos/${video.id}/generate-document`, { method: "POST" });
    setVideo((prev) =>
      prev ? { ...prev, documentStatus: "pending" } : prev,
    );
    showToast("Document generation queued");
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
    showToast("Title updated");
  }

  async function dismissSuggestedTitle() {
    if (!video) return;
    await apiFetch(`/api/videos/${video.id}/dismiss-title`, { method: "PUT" });
    setVideo((prev) =>
      prev ? { ...prev, suggestedTitle: null } : prev,
    );
  }

  async function uploadThumbnail(file: File) {
    if (!video) return;
    if (file.size > 2 * 1024 * 1024) return;
    const validTypes = ["image/jpeg", "image/png", "image/webp"];
    if (!validTypes.includes(file.type)) return;
    setUploadingThumbnail(true);
    try {
      const result = await apiFetch<{ uploadUrl: string }>(
        `/api/videos/${video.id}/thumbnail`,
        {
          method: "POST",
          body: JSON.stringify({
            contentType: file.type,
            contentLength: file.size,
          }),
        },
      );
      if (!result) return;
      const uploadResp = await fetch(result.uploadUrl, {
        method: "PUT",
        headers: { "Content-Type": file.type },
        body: file,
      });
      if (!uploadResp.ok) return;
      await refetchVideo();
      showToast("Thumbnail updated");
    } finally {
      setUploadingThumbnail(false);
    }
  }

  async function resetThumbnail() {
    if (!video) return;
    setUploadingThumbnail(true);
    try {
      await apiFetch(`/api/videos/${video.id}/thumbnail`, {
        method: "DELETE",
      });
      await refetchVideo();
      showToast("Thumbnail reset");
    } finally {
      setUploadingThumbnail(false);
    }
  }

  async function changeNotification(value: string) {
    if (!video) return;
    const viewNotification = value === "" ? null : value;
    try {
      await apiFetch(`/api/videos/${video.id}/notifications`, {
        method: "PUT",
        body: JSON.stringify({ viewNotification }),
      });
      setVideo((prev) =>
        prev ? { ...prev, viewNotification } : prev,
      );
    } catch {
      // select stays at previous value
    }
  }

  async function openBranding() {
    if (!video) return;
    setBrandingOpen(true);
    setBrandingMessage("");
    try {
      const data = await apiFetch<VideoBranding>(
        `/api/videos/${video.id}/branding`,
      );
      if (data) {
        setVideoBranding(data);
      } else {
        setVideoBranding({
          companyName: null,
          colorBackground: null,
          colorSurface: null,
          colorText: null,
          colorAccent: null,
          footerText: null,
        });
      }
    } catch {
      setVideoBranding({
        companyName: null,
        colorBackground: null,
        colorSurface: null,
        colorText: null,
        colorAccent: null,
        footerText: null,
      });
    }
  }

  async function saveBranding() {
    if (!video) return;
    setSavingBranding(true);
    setBrandingMessage("");
    try {
      await apiFetch(`/api/videos/${video.id}/branding`, {
        method: "PUT",
        body: JSON.stringify({
          companyName: videoBranding.companyName || null,
          colorBackground: videoBranding.colorBackground || null,
          colorSurface: videoBranding.colorSurface || null,
          colorText: videoBranding.colorText || null,
          colorAccent: videoBranding.colorAccent || null,
          footerText: videoBranding.footerText || null,
        }),
      });
      setBrandingMessage("Saved");
      setTimeout(() => setBrandingOpen(false), 1000);
    } catch (err) {
      setBrandingMessage(
        err instanceof Error ? err.message : "Failed to save",
      );
    } finally {
      setSavingBranding(false);
    }
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
    const isInPlaylist = video.playlists.some((p) => p.id === playlistId);
    if (isInPlaylist) {
      await apiFetch(`/api/playlists/${playlistId}/videos/${video.id}`, {
        method: "DELETE",
      });
      setVideo((prev) =>
        prev
          ? { ...prev, playlists: prev.playlists.filter((p) => p.id !== playlistId) }
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
            ? { ...prev, playlists: [...prev.playlists, { id: playlist.id, title: playlist.title }] }
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
  const embedSnippet = `<iframe src="${window.location.origin}/embed/${video.shareToken}" width="640" height="360" frameborder="0" allowfullscreen></iframe>`;

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
      </div>

      {/* Share Settings */}
      <div className="video-detail-section">
        <h2 className="video-detail-section-title">Share Settings</h2>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Share link</span>
          {video.status === "processing" ? (
            <span style={{ color: "var(--color-text-secondary)", fontSize: 13 }}>
              Available once processing completes
            </span>
          ) : (
            <div style={{ display: "flex", gap: 8, flex: 1, minWidth: 0 }}>
              <input
                type="text"
                readOnly
                value={video.shareUrl}
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
              <button onClick={copyLink} className="detail-btn">
                Copy link
              </button>
            </div>
          )}
        </div>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Embed</span>
          <div style={{ display: "flex", gap: 8, flex: 1, minWidth: 0 }}>
            <input
              type="text"
              readOnly
              value={embedSnippet}
              aria-label="Embed code"
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
            <button onClick={copyEmbed} className="detail-btn">
              Copy embed
            </button>
          </div>
        </div>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Password</span>
          <div className="detail-setting-value">
            <span>
              {video.hasPassword ? "Password set" : "No password"}
            </span>
            {video.hasPassword ? (
              <button onClick={removePassword} className="detail-btn">
                Remove password
              </button>
            ) : (
              <button onClick={addPassword} className="detail-btn">
                Set password
              </button>
            )}
          </div>
        </div>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Expiry</span>
          <div className="detail-setting-value">
            <span>{expiry.text}</span>
            <button onClick={toggleLinkExpiry} className="detail-btn">
              {video.shareExpiresAt === null ? "Set expiry" : "Remove expiry"}
            </button>
            {video.shareExpiresAt !== null && (
              <button onClick={extendVideo} className="detail-btn">
                Extend
              </button>
            )}
          </div>
        </div>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Downloads</span>
          <button
            onClick={toggleDownload}
            className={`detail-toggle${video.downloadEnabled ? " detail-toggle--active" : ""}`}
          >
            {video.downloadEnabled ? "Enabled" : "Disabled"}
          </button>
        </div>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Email gate</span>
          <button
            onClick={toggleEmailGate}
            className={`detail-toggle${video.emailGateEnabled ? " detail-toggle--active" : ""}`}
          >
            {video.emailGateEnabled ? "Enabled" : "Disabled"}
          </button>
        </div>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Comments</span>
          <select
            aria-label="Comment mode"
            value={video.commentMode}
            onChange={(e) => changeCommentMode(e.target.value)}
            style={{
              background: "var(--color-surface)",
              border: "1px solid var(--color-border)",
              borderRadius: 4,
              color:
                video.commentMode !== "disabled"
                  ? "var(--color-accent)"
                  : "var(--color-text-secondary)",
              fontSize: 13,
              padding: "4px 8px",
              cursor: "pointer",
            }}
          >
            <option value="disabled">Off</option>
            <option value="anonymous">Anonymous</option>
            <option value="name_required">Name required</option>
            <option value="name_email_required">Name + email</option>
          </select>
        </div>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Thumbnail</span>
          <div className="detail-setting-value">
            <label style={{ cursor: uploadingThumbnail ? "default" : "pointer" }}>
              <span className="detail-btn" role="button" tabIndex={0}>
                {uploadingThumbnail ? "Uploading..." : "Upload"}
              </span>
              <input
                type="file"
                accept="image/jpeg,image/png,image/webp"
                style={{ display: "none" }}
                disabled={uploadingThumbnail}
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) uploadThumbnail(file);
                  e.target.value = "";
                }}
              />
            </label>
            {video.thumbnailUrl && (
              <button
                onClick={resetThumbnail}
                disabled={uploadingThumbnail}
                className="detail-btn"
              >
                Reset thumbnail
              </button>
            )}
          </div>
        </div>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Notifications</span>
          <select
            aria-label="View notifications"
            value={video.viewNotification ?? ""}
            onChange={(e) => changeNotification(e.target.value)}
            style={{
              background: "var(--color-surface)",
              border: "1px solid var(--color-border)",
              borderRadius: 4,
              color: "var(--color-text-secondary)",
              fontSize: 13,
              padding: "4px 8px",
              cursor: "pointer",
            }}
          >
            <option value="">Account default</option>
            <option value="off">Off</option>
            <option value="every">Every view</option>
            <option value="digest">Daily digest</option>
          </select>
        </div>

        {limits?.brandingEnabled && (
          <div className="detail-setting-row">
            <span className="detail-setting-label">Branding</span>
            <button onClick={openBranding} className="detail-btn">
              Customize
            </button>
          </div>
        )}
      </div>

      {/* AI */}
      <div className="video-detail-section">
        <h2 className="video-detail-section-title">AI</h2>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Transcript</span>
          <div className="detail-setting-value">
            <span>
              {video.transcriptStatus === "none" && "Not started"}
              {video.transcriptStatus === "pending" && "Pending..."}
              {video.transcriptStatus === "processing" && "Transcribing..."}
              {video.transcriptStatus === "ready" && "Ready"}
              {video.transcriptStatus === "no_audio" && "No audio"}
              {video.transcriptStatus === "failed" && "Failed"}
            </span>
            {(video.transcriptStatus === "none" || video.transcriptStatus === "ready" || video.transcriptStatus === "failed" || video.transcriptStatus === "no_audio") && (
              <>
                {limits?.transcriptionEnabled && (
                  <select
                    aria-label="Transcription language"
                    value={retranscribeLanguage}
                    onChange={(e) => setRetranscribeLanguage(e.target.value)}
                    className="detail-select"
                  >
                    {TRANSCRIPTION_LANGUAGES.map((lang) => (
                      <option key={lang.code} value={lang.code}>{lang.name}</option>
                    ))}
                  </select>
                )}
                <button onClick={retranscribe} className="detail-btn">
                  {video.transcriptStatus === "none" ? "Transcribe" :
                   video.transcriptStatus === "ready" ? "Redo transcript" : "Retry transcript"}
                </button>
              </>
            )}
          </div>
        </div>

        {video.transcriptStatus === "ready" && transcriptSegments.length > 0 && (
          <div className="transcript-segments">
            {transcriptSegments.map((seg, i) => (
              <div key={i} className="transcript-segment">
                <span className="transcript-segment-time">
                  {formatTimestamp(seg.start)}
                </span>
                <span className="transcript-segment-text">{seg.text}</span>
              </div>
            ))}
          </div>
        )}

        {limits?.aiEnabled && (
          <div className="detail-setting-row">
            <span className="detail-setting-label">Summary</span>
            <div className="detail-setting-value">
              <span>
                {video.summaryStatus === "none" && "Not started"}
                {video.summaryStatus === "pending" && "Pending..."}
                {video.summaryStatus === "processing" && "Summarizing..."}
                {video.summaryStatus === "ready" && "Ready"}
                {video.summaryStatus === "too_short" && "Transcript too short"}
                {video.summaryStatus === "failed" && "Failed"}
              </span>
              <button
                onClick={summarize}
                disabled={
                  video.transcriptStatus !== "ready" ||
                  video.summaryStatus === "pending" ||
                  video.summaryStatus === "processing"
                }
                className="detail-btn"
                style={{
                  opacity:
                    video.transcriptStatus !== "ready" ||
                    video.summaryStatus === "pending" ||
                    video.summaryStatus === "processing"
                      ? 0.5
                      : undefined,
                }}
              >
                {video.summaryStatus === "ready"
                  ? "Re-summarize"
                  : "Summarize"}
              </button>
            </div>
          </div>
        )}

        {limits?.aiEnabled && (
          <div className="detail-setting-row">
            <span className="detail-setting-label">Document</span>
            <div className="detail-setting-value">
              <span>
                {video.documentStatus === "none" && "Not generated"}
                {video.documentStatus === "pending" && "Pending..."}
                {video.documentStatus === "processing" && "Generating..."}
                {video.documentStatus === "ready" && "Ready"}
                {video.documentStatus === "too_short" && "Transcript too short"}
                {video.documentStatus === "failed" && "Failed"}
              </span>
              {video.documentStatus === "ready" ? (
                <>
                  <button
                    onClick={viewDocument}
                    className="detail-btn detail-btn--accent"
                  >
                    View
                  </button>
                  <button
                    onClick={generateDocument}
                    className="detail-btn"
                  >
                    Regenerate
                  </button>
                </>
              ) : (
                <button
                  onClick={generateDocument}
                  disabled={
                    video.transcriptStatus !== "ready" ||
                    video.documentStatus === "pending" ||
                    video.documentStatus === "processing"
                  }
                  className="detail-btn"
                  style={{
                    opacity:
                      video.transcriptStatus !== "ready" ||
                      video.documentStatus === "pending" ||
                      video.documentStatus === "processing"
                        ? 0.5
                        : undefined,
                  }}
                >
                  Generate
                </button>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Editing */}
      <div className="video-detail-section">
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
      </div>

      {/* Organization */}
      <div className="video-detail-section">
        <h2 className="video-detail-section-title">Organization</h2>

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
                  const active = video.playlists.some((vp) => vp.id === playlist.id);
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
      </div>

      {/* Call to Action */}
      <div className="video-detail-section">
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
      </div>

      {/* Comments */}
      <div className="video-detail-section">
        <h2 className="video-detail-section-title">
          Comments ({comments.length})
        </h2>
        {comments.length === 0 ? (
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13 }}>
            No comments yet.
          </p>
        ) : (
          comments.map((comment) => (
            <div key={comment.id} className="comment-item">
              <div className="comment-avatar">
                {getInitials(comment.authorName)}
              </div>
              <div className="comment-content">
                <div className="comment-header">
                  <span className="comment-author">
                    {comment.authorName || "Anonymous"}
                  </span>
                  <span className="comment-date">
                    {relativeTime(comment.createdAt)}
                  </span>
                  {comment.videoTimestamp !== null && (
                    <span className="comment-timestamp">
                      @{formatTimestamp(comment.videoTimestamp)}
                    </span>
                  )}
                  {comment.isPrivate && (
                    <span className="comment-private">Private</span>
                  )}
                </div>
                <div className="comment-body">{comment.body}</div>
              </div>
              <button
                className="comment-delete"
                onClick={() => handleDeleteComment(comment.id)}
                aria-label="Delete comment"
              >
                Delete
              </button>
            </div>
          ))
        )}
      </div>

      {/* Danger Zone */}
      <div className="danger-zone">
        <button
          onClick={deleteVideo}
          className="detail-btn detail-btn--danger"
          style={{ padding: "8px 20px" }}
        >
          Delete video
        </button>
      </div>

      {/* Branding Modal */}
      {brandingOpen && (
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
          onClick={() => setBrandingOpen(false)}
        >
          <div
            onClick={(e) => e.stopPropagation()}
            style={{
              background: "var(--color-surface)",
              borderRadius: 12,
              padding: 24,
              width: "calc(100vw - 32px)",
              maxWidth: 400,
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
              Video Branding
            </h3>
            <p
              style={{
                color: "var(--color-text-secondary)",
                fontSize: 13,
                margin: "0 0 16px",
              }}
            >
              Override your account branding for this video. Leave empty to
              inherit.
            </p>

            <label
              style={{
                display: "flex",
                flexDirection: "column",
                gap: 4,
                marginBottom: 12,
              }}
            >
              <span
                style={{
                  color: "var(--color-text-secondary)",
                  fontSize: 13,
                }}
              >
                Company name
              </span>
              <input
                type="text"
                value={videoBranding.companyName ?? ""}
                onChange={(e) =>
                  setVideoBranding({
                    ...videoBranding,
                    companyName: e.target.value || null,
                  })
                }
                placeholder="Inherit from account"
                maxLength={200}
                style={{
                  background: "var(--color-bg)",
                  border: "1px solid var(--color-border)",
                  borderRadius: 4,
                  color: "var(--color-text)",
                  padding: "8px 12px",
                  fontSize: 14,
                  width: "100%",
                }}
              />
            </label>

            <div
              style={{
                display: "grid",
                gridTemplateColumns: "1fr 1fr",
                gap: 8,
                marginBottom: 12,
              }}
            >
              {(
                [
                  "colorBackground",
                  "colorSurface",
                  "colorText",
                  "colorAccent",
                ] as const
              ).map((key) => {
                const labels: Record<string, string> = {
                  colorBackground: "Background",
                  colorSurface: "Surface",
                  colorText: "Text",
                  colorAccent: "Accent",
                };
                return (
                  <label
                    key={key}
                    style={{
                      display: "flex",
                      flexDirection: "column",
                      gap: 4,
                    }}
                  >
                    <span
                      style={{
                        color: "var(--color-text-secondary)",
                        fontSize: 13,
                      }}
                    >
                      {labels[key]}
                    </span>
                    <input
                      type="text"
                      value={videoBranding[key] ?? ""}
                      onChange={(e) =>
                        setVideoBranding({
                          ...videoBranding,
                          [key]: e.target.value || null,
                        })
                      }
                      placeholder="Inherit"
                      style={{
                        background: "var(--color-bg)",
                        border: "1px solid var(--color-border)",
                        borderRadius: 4,
                        color: "var(--color-text)",
                        padding: "6px 10px",
                        fontSize: 13,
                        width: "100%",
                      }}
                    />
                  </label>
                );
              })}
            </div>

            <label
              style={{
                display: "flex",
                flexDirection: "column",
                gap: 4,
                marginBottom: 16,
              }}
            >
              <span
                style={{
                  color: "var(--color-text-secondary)",
                  fontSize: 13,
                }}
              >
                Footer text
              </span>
              <input
                type="text"
                value={videoBranding.footerText ?? ""}
                onChange={(e) =>
                  setVideoBranding({
                    ...videoBranding,
                    footerText: e.target.value || null,
                  })
                }
                placeholder="Inherit from account"
                maxLength={500}
                style={{
                  background: "var(--color-bg)",
                  border: "1px solid var(--color-border)",
                  borderRadius: 4,
                  color: "var(--color-text)",
                  padding: "8px 12px",
                  fontSize: 14,
                  width: "100%",
                }}
              />
            </label>

            {brandingMessage && (
              <p
                style={{
                  color:
                    brandingMessage === "Saved"
                      ? "var(--color-accent)"
                      : "var(--color-error)",
                  fontSize: 13,
                  margin: "0 0 12px",
                }}
              >
                {brandingMessage}
              </p>
            )}

            <div style={{ display: "flex", gap: 8 }}>
              <button
                onClick={saveBranding}
                disabled={savingBranding}
                className="detail-btn detail-btn--accent"
              >
                {savingBranding ? "Saving..." : "Save"}
              </button>
              <button
                onClick={() => setBrandingOpen(false)}
                className="detail-btn"
              >
                Cancel
              </button>
            </div>
          </div>
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
            showToast("Removing filler words...");
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
            showToast("Removing silent pauses...");
          }}
        />
      )}

      {/* Document Modal */}
      {showDocumentModal && documentContent && (
        <DocumentModal
          document={documentContent}
          onClose={() => { setShowDocumentModal(false); setDocumentContent(null); }}
        />
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
