import { useEffect, useState } from "react";
import { Link, useParams, useLocation } from "react-router-dom";
import { apiFetch } from "../api/client";

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
  const location = useLocation();
  const routerState = location.state as { video?: Video } | null;

  const [video, setVideo] = useState<Video | null>(routerState?.video ?? null);
  const [loading, setLoading] = useState(!routerState?.video);
  const [notFound, setNotFound] = useState(false);
  const [limits, setLimits] = useState<LimitsResponse | null>(null);
  const [folders, setFolders] = useState<Folder[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);

  useEffect(() => {
    async function fetchData() {
      try {
        if (!routerState?.video) {
          const [videos, limits, folders, tags] = await Promise.all([
            apiFetch<Video[]>("/api/videos"),
            apiFetch<LimitsResponse>("/api/videos/limits"),
            apiFetch<Folder[]>("/api/folders"),
            apiFetch<Tag[]>("/api/tags"),
          ]);
          const found = videos?.find((v) => v.id === id) ?? null;
          setVideo(found);
          setLimits(limits ?? null);
          setFolders(folders ?? []);
          setTags(tags ?? []);
          if (!found) {
            setNotFound(true);
          }
        } else {
          const [limits, folders, tags] = await Promise.all([
            apiFetch<LimitsResponse>("/api/videos/limits"),
            apiFetch<Folder[]>("/api/folders"),
            apiFetch<Tag[]>("/api/tags"),
          ]);
          setLimits(limits ?? null);
          setFolders(folders ?? []);
          setTags(tags ?? []);
        }
      } catch {
        if (!routerState?.video) {
          setNotFound(true);
        }
      } finally {
        setLoading(false);
      }
    }

    fetchData();
  }, [id, routerState?.video]);

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

      <div style={{ display: "flex", gap: 24, alignItems: "flex-start" }}>
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
                    background: "var(--color-background)",
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
    </div>
  );
}
