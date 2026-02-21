import { useEffect, useRef, useState } from "react";
import { Link, useParams, useNavigate } from "react-router-dom";
import { apiFetch } from "../api/client";
import { TrimModal } from "../components/TrimModal";
import { FillerRemovalModal } from "../components/FillerRemovalModal";

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
  folderId: string | null;
  tags: VideoTag[];
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

interface LimitsResponse {
  maxVideosPerMonth: number;
  maxVideoDurationSeconds: number;
  videosUsedThisMonth: number;
  brandingEnabled: boolean;
  aiEnabled: boolean;
}

interface VideoBranding {
  companyName: string | null;
  colorBackground: string | null;
  colorSurface: string | null;
  colorText: string | null;
  colorAccent: string | null;
  footerText: string | null;
}

function formatDuration(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = Math.floor(seconds % 60);
  return `${minutes}:${String(remainingSeconds).padStart(2, "0")}`;
}

function formatDate(isoDate: string): string {
  return new Date(isoDate).toLocaleDateString();
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

export function VideoDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [video, setVideo] = useState<Video | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [limits, setLimits] = useState<LimitsResponse | null>(null);
  const [folders, setFolders] = useState<Folder[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);

  const [toast, setToast] = useState<string | null>(null);
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [editingTitle, setEditingTitle] = useState(false);
  const [editTitle, setEditTitle] = useState("");

  const [ctaFormOpen, setCtaFormOpen] = useState(false);
  const [ctaText, setCtaText] = useState("");
  const [ctaUrl, setCtaUrl] = useState("");

  const [showTrimModal, setShowTrimModal] = useState(false);
  const [showFillerModal, setShowFillerModal] = useState(false);

  const [uploadingThumbnail, setUploadingThumbnail] = useState(false);

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
        const [videos, limitsData, foldersData, tagsData] =
          await Promise.all([
            apiFetch<Video[]>("/api/videos"),
            apiFetch<LimitsResponse>("/api/videos/limits"),
            apiFetch<Folder[]>("/api/folders"),
            apiFetch<Tag[]>("/api/tags"),
          ]);
        const found = videos?.find((v) => v.id === id) ?? null;
        setVideo(found);
        setLimits(limitsData ?? null);
        setFolders(foldersData ?? []);
        setTags(tagsData ?? []);
        if (!found) {
          setNotFound(true);
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

  async function addPassword() {
    if (!video) return;
    const password = window.prompt("Enter a password for this video:");
    if (!password) return;
    await apiFetch(`/api/videos/${video.id}/password`, {
      method: "PUT",
      body: JSON.stringify({ password }),
    });
    setVideo((prev) => (prev ? { ...prev, hasPassword: true } : prev));
  }

  async function removePassword() {
    if (!video) return;
    if (!window.confirm("Remove the password from this video?")) return;
    await apiFetch(`/api/videos/${video.id}/password`, {
      method: "PUT",
      body: JSON.stringify({ password: "" }),
    });
    setVideo((prev) => (prev ? { ...prev, hasPassword: false } : prev));
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
    await apiFetch(`/api/videos/${video.id}/retranscribe`, { method: "POST" });
    setVideo((prev) =>
      prev ? { ...prev, transcriptStatus: "pending" } : prev,
    );
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

  async function acceptSuggestedTitle() {
    if (!video || !video.suggestedTitle) return;
    await apiFetch(`/api/videos/${video.id}`, {
      method: "PATCH",
      body: JSON.stringify({ title: video.suggestedTitle }),
    });
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

  async function deleteVideo() {
    if (!video) return;
    if (!window.confirm("Delete this recording? This cannot be undone."))
      return;
    await apiFetch(`/api/videos/${video.id}`, { method: "DELETE" });
    navigate("/library");
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
        <Link
          to="/library"
          style={{
            color: "var(--color-text-secondary)",
            textDecoration: "none",
            fontSize: 14,
          }}
        >
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

      <div style={{ display: "flex", gap: 24, alignItems: "flex-start", marginBottom: 8 }}>
        {video.thumbnailUrl && (
          <img
            src={video.thumbnailUrl}
            alt="Video thumbnail"
            style={{
              width: 240,
              height: 135,
              objectFit: "cover",
              borderRadius: 8,
              background: "var(--color-border)",
              flexShrink: 0,
            }}
          />
        )}

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

          <p
            style={{
              color: "var(--color-text-secondary)",
              fontSize: 14,
              margin: "8px 0 0",
            }}
          >
            {formatDuration(video.duration)} &middot;{" "}
            {formatDate(video.createdAt)} &middot;{" "}
            {viewCountLabel(video.viewCount, video.uniqueViewCount)} &middot;{" "}
            <span
              style={{
                color:
                  video.shareExpiresAt === null
                    ? "var(--color-accent)"
                    : expiry.expired
                      ? "var(--color-error)"
                      : "var(--color-text-secondary)",
              }}
            >
              {expiry.text}
            </span>
          </p>

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

      {/* Section 1: Sharing */}
      <div className="video-detail-section">
        <h2 className="video-detail-section-title">Sharing</h2>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Share link</span>
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

      {/* Section 2: Editing */}
      <div className="video-detail-section">
        <h2 className="video-detail-section-title">Editing</h2>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Transcript</span>
          <div className="detail-setting-value">
            <span>
              {video.transcriptStatus === "none" && "Not started"}
              {video.transcriptStatus === "pending" && "Pending..."}
              {video.transcriptStatus === "processing" && "Transcribing..."}
              {video.transcriptStatus === "ready" && "Ready"}
              {video.transcriptStatus === "failed" && "Failed"}
            </span>
            {video.transcriptStatus === "none" && (
              <button onClick={retranscribe} className="detail-btn">
                Transcribe
              </button>
            )}
            {video.transcriptStatus === "ready" && (
              <button onClick={retranscribe} className="detail-btn">
                Redo transcript
              </button>
            )}
            {video.transcriptStatus === "failed" && (
              <button onClick={retranscribe} className="detail-btn">
                Retry transcript
              </button>
            )}
          </div>
        </div>

        {limits?.aiEnabled && (
          <div className="detail-setting-row">
            <span className="detail-setting-label">Summary</span>
            <div className="detail-setting-value">
              <span>
                {video.summaryStatus === "none" && "Not started"}
                {video.summaryStatus === "pending" && "Pending..."}
                {video.summaryStatus === "processing" && "Summarizing..."}
                {video.summaryStatus === "ready" && "Ready"}
                {video.summaryStatus === "failed" && "Failed"}
              </span>
              <button
                onClick={summarize}
                disabled={
                  video.summaryStatus === "pending" ||
                  video.summaryStatus === "processing"
                }
                className="detail-btn"
                style={{
                  opacity:
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
          >
            Trim video
          </button>
        </div>

        {video.transcriptStatus === "ready" && (
          <div className="detail-setting-row">
            <span className="detail-setting-label">Fillers</span>
            <button
              onClick={() => setShowFillerModal(true)}
              className="detail-btn"
            >
              Remove fillers
            </button>
          </div>
        )}
      </div>

      {/* Section 3: Customization */}
      <div className="video-detail-section">
        <h2 className="video-detail-section-title">Customization</h2>

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

      {/* Section 4: Organization */}
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
      </div>

      {/* Footer: Delete */}
      <div style={{ marginTop: 32, paddingTop: 16, borderTop: "1px solid var(--color-border)" }}>
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
