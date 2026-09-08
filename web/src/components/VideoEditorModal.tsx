import { useEffect, useRef, useState } from "react";
import { apiFetch } from "../api/client";
import { formatDuration } from "../utils/format";

interface VideoEditorModalProps {
  videoId: string;
  duration: number;
  onClose: () => void;
  onTrimStarted?: () => void;
}

export function VideoEditorModal({
  videoId,
  duration,
  onClose,
  onTrimStarted,
}: VideoEditorModalProps) {
  const [videoUrl, setVideoUrl] = useState<string | null>(null);
  const [currentTime, setCurrentTime] = useState(0);
  const [trimStart, setTrimStart] = useState(0);
  const [trimEnd, setTrimEnd] = useState(duration);
  const [trimming, setTrimming] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const videoRef = useRef<HTMLVideoElement>(null);
  const timelineRef = useRef<HTMLDivElement>(null);
  const draggingTrimRef = useRef<"start" | "end" | null>(null);

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
    setTrimEnd(duration);
  }, [duration]);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  function timeFromClientX(clientX: number) {
    const timeline = timelineRef.current;
    if (!timeline || !duration) return 0;

    const rect = timeline.getBoundingClientRect();
    const x = Math.max(0, Math.min(clientX - rect.left, rect.width));
    return (x / rect.width) * duration;
  }

  function handleTrimPointerDown(handle: "start" | "end") {
    return (e: React.MouseEvent | React.TouchEvent) => {
      e.preventDefault();
      e.stopPropagation();
      draggingTrimRef.current = handle;

      const fixedStart = trimStart;
      const fixedEnd = trimEnd;
      const minimumGap = 1;

      function onMove(ev: MouseEvent | TouchEvent) {
        const point = "touches" in ev ? ev.touches[0] : ev;
        if (!point) return;

        const rawTime = timeFromClientX(point.clientX);

        let nextTime = rawTime;

        if (handle === "start") {
          nextTime = Math.max(
            0,
            Math.min(rawTime, fixedEnd - minimumGap)
          );
          setTrimStart(nextTime);
        } else {
          nextTime = Math.min(
            duration,
            Math.max(rawTime, fixedStart + minimumGap)
          );
          setTrimEnd(nextTime);
        }

        setCurrentTime(nextTime);

        if (videoRef.current) {
          videoRef.current.currentTime = nextTime;
        }
      }

      function onUp() {
        draggingTrimRef.current = null;
        document.removeEventListener("mousemove", onMove);
        document.removeEventListener("mouseup", onUp);
        document.removeEventListener("touchmove", onMove);
        document.removeEventListener("touchend", onUp);
      }

      document.addEventListener("mousemove", onMove);
      document.addEventListener("mouseup", onUp);
      document.addEventListener("touchmove", onMove, { passive: false });
      document.addEventListener("touchend", onUp);
    };
  }

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

  function handleResetTrim() {
    setTrimStart(0);
    setTrimEnd(duration);
    setCurrentTime(0);

    if (videoRef.current) {
      videoRef.current.currentTime = 0;
    }
  }

  async function handleApplyTrim() {
    if (trimEnd - trimStart < 1) {
      setError("Der verbleibende Bereich muss mindestens 1 Sekunde lang sein.");
      return;
    }

    const confirmed = window.confirm(
      "Das aktuelle Video wird durch die getrimmte Version ersetzt. Möchtest du fortfahren?"
    );

    if (!confirmed) return;

    setTrimming(true);
    setError(null);

    try {
      await apiFetch(`/api/videos/${videoId}/trim`, {
        method: "POST",
        body: JSON.stringify({
          startSeconds: trimStart,
          endSeconds: trimEnd,
        }),
      });

      onTrimStarted?.();
      onClose();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Trimmen fehlgeschlagen."
      );
      setTrimming(false);
    }
  }

  const playheadPct =
    duration > 0
      ? Math.max(0, Math.min(100, (currentTime / duration) * 100))
      : 0;

  const trimStartPct =
    duration > 0 ? (trimStart / duration) * 100 : 0;

  const trimEndPct =
    duration > 0 ? (trimEnd / duration) * 100 : 100;

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
            alignItems: "center",
            gap: 12,
            marginBottom: 16,
          }}
        >
          <button
            type="button"
            style={{
              border: "none",
              borderRadius: 8,
              padding: "8px 14px",
              background: "var(--color-accent)",
              color: "var(--color-text)",
              fontWeight: 600,
              cursor: "default",
            }}
          >
            ✂ Trimmen
          </button>

          <span
            style={{
              fontSize: 13,
              color: "var(--color-text-secondary)",
            }}
          >
            Bereich mit den beiden Griffen auswählen – noch nicht gespeichert
          </span>
        </div>

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
          ref={timelineRef}
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
              left: 0,
              width: `${trimStartPct}%`,
              background: "rgba(15, 23, 42, 0.6)",
              pointerEvents: "none",
            }}
          />

          <div
            style={{
              position: "absolute",
              top: 0,
              bottom: 0,
              left: `${trimEndPct}%`,
              right: 0,
              background: "rgba(15, 23, 42, 0.6)",
              pointerEvents: "none",
            }}
          />

          <div
            onMouseDown={handleTrimPointerDown("start")}
            onTouchStart={handleTrimPointerDown("start")}
            title="Trim-Anfang"
            style={{
              position: "absolute",
              top: 4,
              bottom: 4,
              left: `${trimStartPct}%`,
              width: 14,
              transform: "translateX(-7px)",
              background: "#fff",
              border: "2px solid var(--color-accent)",
              borderRadius: 5,
              cursor: "ew-resize",
              zIndex: 4,
              touchAction: "none",
            }}
          />

          <div
            onMouseDown={handleTrimPointerDown("end")}
            onTouchStart={handleTrimPointerDown("end")}
            title="Trim-Ende"
            style={{
              position: "absolute",
              top: 4,
              bottom: 4,
              left: `${trimEndPct}%`,
              width: 14,
              transform: "translateX(-7px)",
              background: "#fff",
              border: "2px solid var(--color-accent)",
              borderRadius: 5,
              cursor: "ew-resize",
              zIndex: 4,
              touchAction: "none",
            }}
          />

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
            display: "flex",
            justifyContent: "space-between",
            gap: 16,
            marginTop: 10,
            fontSize: 12,
            color: "var(--color-text-secondary)",
          }}
        >
          <span>Anfang: {formatDuration(trimStart)}</span>
          <span>
            Auswahl: {formatDuration(Math.max(0, trimEnd - trimStart))}
          </span>
          <span>Ende: {formatDuration(trimEnd)}</span>
        </div>

        <div
          style={{
            display: "flex",
            justifyContent: "flex-end",
            gap: 10,
            marginTop: 18,
          }}
        >
          <button
            type="button"
            onClick={handleResetTrim}
            disabled={trimming}
            style={{
              background: "transparent",
              color: "var(--color-text-secondary)",
              border: "1px solid var(--color-border)",
              borderRadius: 8,
              padding: "9px 16px",
              fontWeight: 600,
              cursor: trimming ? "default" : "pointer",
            }}
          >
            Zurücksetzen
          </button>

          <button
            type="button"
            onClick={handleApplyTrim}
            disabled={
              trimming ||
              (trimStart <= 0.001 && trimEnd >= duration - 0.001)
            }
            style={{
              background: "var(--color-accent)",
              color: "var(--color-text)",
              border: "none",
              borderRadius: 8,
              padding: "9px 18px",
              fontWeight: 600,
              cursor: trimming ? "default" : "pointer",
              opacity:
                trimming ||
                (trimStart <= 0.001 && trimEnd >= duration - 0.001)
                  ? 0.6
                  : 1,
            }}
          >
            {trimming ? "Wird getrimmt..." : "Trimmen anwenden"}
          </button>
        </div>
      </div>
    </div>
  );
}
