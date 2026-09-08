import { useEffect, useRef, useState } from "react";
import { apiFetch } from "../api/client";
import { formatDuration } from "../utils/format";

interface VideoEditorModalProps {
  videoId: string;
  duration: number;
  onClose: () => void;
}

export function VideoEditorModal({
  videoId,
  duration,
  onClose,
}: VideoEditorModalProps) {
  const [videoUrl, setVideoUrl] = useState<string | null>(null);
  const [currentTime, setCurrentTime] = useState(0);
  const [error, setError] = useState<string | null>(null);

  const videoRef = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    apiFetch<{ downloadUrl: string }>(`/api/videos/${videoId}/download`)
      .then((res) => {
        if (res?.downloadUrl) {
          setVideoUrl(res.downloadUrl);
        } else {
          setError("Video konnte nicht geladen werden.");
        }
      })
      .catch(() => setError("Video konnte nicht geladen werden."));
  }, [videoId]);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  function handleTimelineClick(e: React.MouseEvent<HTMLDivElement>) {
    if (!duration) return;

    const rect = e.currentTarget.getBoundingClientRect();
    const x = Math.max(0, Math.min(e.clientX - rect.left, rect.width));
    const nextTime = (x / rect.width) * duration;

    setCurrentTime(nextTime);

    if (videoRef.current) {
      videoRef.current.currentTime = nextTime;
    }
  }

  const playheadPct =
    duration > 0
      ? Math.max(0, Math.min(100, (currentTime / duration) * 100))
      : 0;

  return (
    <div
      style={{
        position: "fixed",
        inset: 0,
        background: "var(--color-overlay)",
        zIndex: 1000,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: 24,
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="video-editor-title"
        style={{
          width: "min(1000px, 96vw)",
          maxHeight: "94vh",
          overflowY: "auto",
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: 12,
          padding: 24,
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            marginBottom: 16,
          }}
        >
          <h2
            id="video-editor-title"
            style={{
              margin: 0,
              fontSize: 20,
              color: "var(--color-text)",
            }}
          >
            Video bearbeiten
          </h2>

          <button
            type="button"
            onClick={onClose}
            style={{
              background: "transparent",
              border: "1px solid var(--color-border)",
              borderRadius: 8,
              padding: "6px 12px",
              color: "var(--color-text-secondary)",
              cursor: "pointer",
            }}
          >
            Schließen
          </button>
        </div>

        {error && (
          <div
            style={{
              color: "var(--color-error)",
              marginBottom: 16,
            }}
          >
            {error}
          </div>
        )}

        {videoUrl && (
          <video
            ref={videoRef}
            src={videoUrl}
            controls
            onTimeUpdate={(e) =>
              setCurrentTime(e.currentTarget.currentTime)
            }
            style={{
              width: "100%",
              maxHeight: 480,
              background: "#000",
              borderRadius: 8,
              marginBottom: 24,
            }}
          />
        )}

        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            fontSize: 13,
            color: "var(--color-text-secondary)",
            marginBottom: 8,
          }}
        >
          <span>{formatDuration(currentTime)}</span>
          <span>{formatDuration(duration)}</span>
        </div>

        <div
          onClick={handleTimelineClick}
          style={{
            position: "relative",
            height: 64,
            borderRadius: 8,
            background: "var(--color-border)",
            cursor: "pointer",
            overflow: "hidden",
            userSelect: "none",
          }}
        >
          <div
            style={{
              position: "absolute",
              inset: "10px 0",
              background: "var(--color-accent)",
              opacity: 0.8,
              borderRadius: 6,
            }}
          />

          <div
            style={{
              position: "absolute",
              inset: "10px 12px",
              display: "flex",
              alignItems: "center",
              color: "#fff",
              fontSize: 13,
              fontWeight: 600,
              pointerEvents: "none",
            }}
          >
            Clip 1
          </div>

          <div
            style={{
              position: "absolute",
              top: 0,
              bottom: 0,
              left: `${playheadPct}%`,
              width: 2,
              background: "#fff",
              transform: "translateX(-1px)",
              pointerEvents: "none",
            }}
          />

          <div
            style={{
              position: "absolute",
              top: 0,
              left: `${playheadPct}%`,
              width: 10,
              height: 10,
              borderRadius: "50%",
              background: "#fff",
              transform: "translate(-5px, -2px)",
              pointerEvents: "none",
            }}
          />
        </div>

        <div
          style={{
            marginTop: 10,
            fontSize: 12,
            color: "var(--color-text-secondary)",
          }}
        >
          Timeline – Klick setzt die Abspielposition
        </div>
      </div>
    </div>
  );
}
