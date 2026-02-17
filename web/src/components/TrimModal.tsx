import { useCallback, useEffect, useRef, useState } from "react";
import { apiFetch } from "../api/client";

interface TrimModalProps {
  videoId: string;
  duration: number;
  onClose: () => void;
  onTrimStarted: () => void;
}

function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${String(s).padStart(2, "0")}`;
}

export function TrimModal({ videoId, duration, onClose, onTrimStarted }: TrimModalProps) {
  const [startSeconds, setStartSeconds] = useState(0);
  const [endSeconds, setEndSeconds] = useState(duration);
  const [trimming, setTrimming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [videoUrl, setVideoUrl] = useState<string | null>(null);

  const videoRef = useRef<HTMLVideoElement>(null);
  const trackRef = useRef<HTMLDivElement>(null);
  const draggingRef = useRef<"start" | "end" | null>(null);

  useEffect(() => {
    apiFetch<{ downloadUrl: string }>(`/api/videos/${videoId}/download`)
      .then(resp => setVideoUrl(resp?.downloadUrl ?? null));
  }, [videoId]);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  const secondsToPercent = useCallback((s: number) => (s / duration) * 100, [duration]);
  const percentToSeconds = useCallback((p: number) => (p / 100) * duration, [duration]);

  function getPercentFromEvent(e: MouseEvent | Touch): number {
    const track = trackRef.current;
    if (!track) return 0;
    const rect = track.getBoundingClientRect();
    const x = Math.max(0, Math.min(e.clientX - rect.left, rect.width));
    return (x / rect.width) * 100;
  }

  function handlePointerDown(handle: "start" | "end") {
    return (e: React.MouseEvent | React.TouchEvent) => {
      e.preventDefault();
      draggingRef.current = handle;

      function onMove(ev: MouseEvent | TouchEvent) {
        const point = "touches" in ev ? ev.touches[0] : ev;
        const pct = getPercentFromEvent(point);
        const secs = Math.round(percentToSeconds(pct) * 10) / 10;

        if (draggingRef.current === "start") {
          const clamped = Math.min(secs, endSeconds - 1);
          setStartSeconds(Math.max(0, clamped));
          if (videoRef.current) videoRef.current.currentTime = Math.max(0, clamped);
        } else {
          const clamped = Math.max(secs, startSeconds + 1);
          setEndSeconds(Math.min(duration, clamped));
          if (videoRef.current) videoRef.current.currentTime = Math.min(duration, clamped);
        }
      }

      function onUp() {
        draggingRef.current = null;
        document.removeEventListener("mousemove", onMove);
        document.removeEventListener("mouseup", onUp);
        document.removeEventListener("touchmove", onMove);
        document.removeEventListener("touchend", onUp);
      }

      document.addEventListener("mousemove", onMove);
      document.addEventListener("mouseup", onUp);
      document.addEventListener("touchmove", onMove);
      document.addEventListener("touchend", onUp);
    };
  }

  function handleTrackClick(e: React.MouseEvent<HTMLDivElement>) {
    if (draggingRef.current) return;
    if (e.target !== e.currentTarget) return;
    const pct = getPercentFromEvent(e.nativeEvent);
    const secs = Math.round(percentToSeconds(pct) * 10) / 10;

    const distToStart = Math.abs(secs - startSeconds);
    const distToEnd = Math.abs(secs - endSeconds);

    if (distToStart <= distToEnd) {
      setStartSeconds(Math.max(0, Math.min(secs, endSeconds - 1)));
      if (videoRef.current) videoRef.current.currentTime = secs;
    } else {
      setEndSeconds(Math.min(duration, Math.max(secs, startSeconds + 1)));
      if (videoRef.current) videoRef.current.currentTime = secs;
    }
  }

  async function handleTrim() {
    setTrimming(true);
    setError(null);
    try {
      await apiFetch(`/api/videos/${videoId}/trim`, {
        method: "POST",
        body: JSON.stringify({ startSeconds, endSeconds }),
      });
      onTrimStarted();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Trim failed");
    } finally {
      setTrimming(false);
    }
  }

  const startPct = secondsToPercent(startSeconds);
  const endPct = secondsToPercent(endSeconds);

  return (
    <div
      style={{
        position: "fixed",
        inset: 0,
        background: "rgba(0, 0, 0, 0.6)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 1000,
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: 12,
          padding: 24,
          width: 640,
          maxWidth: "90vw",
        }}
      >
        <h2 style={{ color: "var(--color-text)", fontSize: 18, margin: "0 0 16px" }}>
          Trim Video
        </h2>

        {videoUrl && (
          <video
            ref={videoRef}
            src={videoUrl}
            style={{
              width: "100%",
              borderRadius: 8,
              background: "#000",
              marginBottom: 16,
              maxHeight: 300,
            }}
          />
        )}

        <div
          data-testid="trim-track"
          ref={trackRef}
          onClick={handleTrackClick}
          style={{
            position: "relative",
            height: 32,
            marginBottom: 12,
            cursor: "pointer",
            userSelect: "none",
          }}
        >
          <div
            style={{
              position: "absolute",
              top: 12,
              left: 0,
              right: 0,
              height: 8,
              background: "var(--color-border)",
              borderRadius: 4,
            }}
          />
          <div
            style={{
              position: "absolute",
              top: 12,
              left: `${startPct}%`,
              width: `${endPct - startPct}%`,
              height: 8,
              background: "var(--color-accent)",
              borderRadius: 4,
            }}
          />
          <div
            data-testid="trim-handle-start"
            onMouseDown={handlePointerDown("start")}
            onTouchStart={handlePointerDown("start")}
            style={{
              position: "absolute",
              top: -4,
              left: `${startPct}%`,
              width: 20,
              height: 40,
              marginLeft: -10,
              background: "var(--color-text)",
              borderRadius: 4,
              cursor: "ew-resize",
              touchAction: "none",
            }}
          />
          <div
            data-testid="trim-handle-end"
            onMouseDown={handlePointerDown("end")}
            onTouchStart={handlePointerDown("end")}
            style={{
              position: "absolute",
              top: -4,
              left: `${endPct}%`,
              width: 20,
              height: 40,
              marginLeft: -10,
              background: "var(--color-text)",
              borderRadius: 4,
              cursor: "ew-resize",
              touchAction: "none",
            }}
          />
        </div>

        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            color: "var(--color-text-secondary)",
            fontSize: 13,
            marginBottom: 16,
          }}
        >
          <span>Start: {formatTime(startSeconds)}</span>
          <span>Duration: {formatTime(endSeconds - startSeconds)}</span>
          <span>End: {formatTime(endSeconds)}</span>
        </div>

        {error && (
          <p style={{ color: "var(--color-error)", fontSize: 13, margin: "0 0 12px" }}>
            {error}
          </p>
        )}

        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <button
            onClick={onClose}
            style={{
              background: "transparent",
              color: "var(--color-text-secondary)",
              border: "1px solid var(--color-border)",
              borderRadius: 4,
              padding: "8px 16px",
              fontSize: 13,
              fontWeight: 600,
              cursor: "pointer",
            }}
          >
            Cancel
          </button>
          <button
            onClick={handleTrim}
            disabled={trimming}
            style={{
              background: "var(--color-accent)",
              color: "var(--color-text)",
              borderRadius: 4,
              padding: "8px 16px",
              fontSize: 13,
              fontWeight: 600,
              border: "none",
              cursor: trimming ? "default" : "pointer",
              opacity: trimming ? 0.7 : 1,
            }}
          >
            {trimming ? "Trimming..." : "Trim"}
          </button>
        </div>
      </div>
    </div>
  );
}
