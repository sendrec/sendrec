import { useEffect, useState } from "react";
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

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

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
          width: 480,
          maxWidth: "90vw",
        }}
      >
        <h2 style={{ color: "var(--color-text)", fontSize: 18, margin: "0 0 16px" }}>
          Trim Video
        </h2>

        <div style={{ display: "flex", flexDirection: "column", gap: 12, marginBottom: 16 }}>
          <label style={{ color: "var(--color-text-secondary)", fontSize: 13 }}>
            Start
            <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 4 }}>
              <input
                type="range"
                min={0}
                max={duration}
                step={0.1}
                value={startSeconds}
                onChange={(e) => {
                  const val = parseFloat(e.target.value);
                  setStartSeconds(Math.min(val, endSeconds - 1));
                }}
                style={{ flex: 1 }}
              />
              <span style={{ color: "var(--color-text)", fontSize: 13, minWidth: 40 }}>
                {formatTime(startSeconds)}
              </span>
            </div>
          </label>

          <label style={{ color: "var(--color-text-secondary)", fontSize: 13 }}>
            End
            <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 4 }}>
              <input
                type="range"
                min={0}
                max={duration}
                step={0.1}
                value={endSeconds}
                onChange={(e) => {
                  const val = parseFloat(e.target.value);
                  setEndSeconds(Math.max(val, startSeconds + 1));
                }}
                style={{ flex: 1 }}
              />
              <span style={{ color: "var(--color-text)", fontSize: 13, minWidth: 40 }}>
                {formatTime(endSeconds)}
              </span>
            </div>
          </label>
        </div>

        <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 16px" }}>
          Result: {formatTime(endSeconds - startSeconds)} (from {formatTime(startSeconds)} to {formatTime(endSeconds)})
        </p>

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
