import { useEffect, useState } from "react";
import { apiFetch } from "../api/client";

interface SilenceSegment {
  start: number;
  end: number;
}

interface SilenceRemovalModalProps {
  videoId: string;
  shareToken: string;
  duration: number;
  onClose: () => void;
  onRemovalStarted: () => void;
}

function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${String(s).padStart(2, "0")}`;
}

export function SilenceRemovalModal({ videoId, onClose, onRemovalStarted }: SilenceRemovalModalProps) {
  const [segments, setSegments] = useState<SilenceSegment[]>([]);
  const [loadError, setLoadError] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [checked, setChecked] = useState<Set<number>>(new Set());
  const [removing, setRemoving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<{ segments: SilenceSegment[] }>(`/api/videos/${videoId}/detect-silence`, {
      method: "POST",
      body: JSON.stringify({ noiseDB: -30, minDuration: 1.0 }),
    })
      .then(data => {
        if (data?.segments) {
          setSegments(data.segments);
          setChecked(new Set(data.segments.map((_, i) => i)));
        }
        setLoaded(true);
      })
      .catch(() => {
        setLoadError(true);
        setLoaded(true);
      });
  }, [videoId]);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  const totalSeconds = segments.reduce((sum, seg) => sum + (seg.end - seg.start), 0);
  const savedSeconds = segments
    .filter((_, i) => checked.has(i))
    .reduce((sum, seg) => sum + (seg.end - seg.start), 0);

  function toggleSegment(index: number) {
    setChecked(prev => {
      const next = new Set(prev);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  }

  function selectAll() {
    setChecked(new Set(segments.map((_, i) => i)));
  }

  function deselectAll() {
    setChecked(new Set());
  }

  async function handleRemove() {
    setRemoving(true);
    setError(null);
    try {
      const selectedSegments = segments
        .filter((_, i) => checked.has(i))
        .map(seg => ({ start: seg.start, end: seg.end }));
      await apiFetch(`/api/videos/${videoId}/remove-segments`, {
        method: "POST",
        body: JSON.stringify({ segments: selectedSegments }),
      });
      onRemovalStarted();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to remove silence");
    } finally {
      setRemoving(false);
    }
  }

  if (!loaded) {
    return (
      <div
        style={{
          position: "fixed", inset: 0, background: "var(--color-overlay)",
          display: "flex", alignItems: "center", justifyContent: "center", zIndex: 1000,
        }}
      >
        <div style={{
          background: "var(--color-surface)", border: "1px solid var(--color-border)",
          borderRadius: 12, padding: 24, width: 500, maxWidth: "90vw",
        }}>
          <p style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Detecting silence...</p>
        </div>
      </div>
    );
  }

  if (loadError) {
    return (
      <div
        style={{
          position: "fixed", inset: 0, background: "var(--color-overlay)",
          display: "flex", alignItems: "center", justifyContent: "center", zIndex: 1000,
        }}
        onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
      >
        <div style={{
          background: "var(--color-surface)", border: "1px solid var(--color-border)",
          borderRadius: 12, padding: 24, width: 500, maxWidth: "90vw",
        }}>
          <h2 style={{ color: "var(--color-text)", fontSize: 18, margin: "0 0 16px" }}>Remove Silent Pauses</h2>
          <p style={{ color: "var(--color-text-secondary)", fontSize: 14, margin: "0 0 16px" }}>
            Failed to detect silence.
          </p>
          <button
            onClick={onClose}
            style={{
              background: "transparent", color: "var(--color-text-secondary)",
              border: "1px solid var(--color-border)", borderRadius: 4,
              padding: "8px 16px", fontSize: 13, fontWeight: 600, cursor: "pointer",
            }}
          >
            Close
          </button>
        </div>
      </div>
    );
  }

  if (segments.length === 0) {
    return (
      <div
        style={{
          position: "fixed", inset: 0, background: "var(--color-overlay)",
          display: "flex", alignItems: "center", justifyContent: "center", zIndex: 1000,
        }}
        onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
      >
        <div style={{
          background: "var(--color-surface)", border: "1px solid var(--color-border)",
          borderRadius: 12, padding: 24, width: 500, maxWidth: "90vw",
        }}>
          <h2 style={{ color: "var(--color-text)", fontSize: 18, margin: "0 0 16px" }}>Remove Silent Pauses</h2>
          <p style={{ color: "var(--color-text-secondary)", fontSize: 14, margin: "0 0 16px" }}>
            No silent pauses detected.
          </p>
          <button
            onClick={onClose}
            style={{
              background: "transparent", color: "var(--color-text-secondary)",
              border: "1px solid var(--color-border)", borderRadius: 4,
              padding: "8px 16px", fontSize: 13, fontWeight: 600, cursor: "pointer",
            }}
          >
            Close
          </button>
        </div>
      </div>
    );
  }

  return (
    <div
      style={{
        position: "fixed", inset: 0, background: "var(--color-overlay)",
        display: "flex", alignItems: "center", justifyContent: "center", zIndex: 1000,
      }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div style={{
        background: "var(--color-surface)", border: "1px solid var(--color-border)",
        borderRadius: 12, padding: 24, width: 500, maxWidth: "90vw",
      }}>
        <h2 style={{ color: "var(--color-text)", fontSize: 18, margin: "0 0 12px" }}>
          Remove Silent Pauses
        </h2>

        <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 12px" }}>
          Found {segments.length} silent pause{segments.length !== 1 ? "s" : ""} ({totalSeconds.toFixed(1)}s total)
        </p>

        <button
          onClick={checked.size === segments.length ? deselectAll : selectAll}
          style={{
            background: "transparent", color: "var(--color-accent)", border: "none",
            padding: 0, fontSize: 12, cursor: "pointer", marginBottom: 8,
          }}
        >
          {checked.size === segments.length ? "Deselect all" : "Select all"}
        </button>

        <div style={{ maxHeight: 300, overflowY: "auto", border: "1px solid var(--color-border)", borderRadius: 8, marginBottom: 16 }}>
          {segments.map((segment, index) => (
            <label
              key={index}
              style={{
                display: "flex", alignItems: "center", gap: 8, padding: "6px 10px",
                cursor: "pointer", fontSize: 13,
                borderBottom: "1px solid var(--color-border)",
              }}
            >
              <input
                type="checkbox"
                checked={checked.has(index)}
                onChange={() => toggleSegment(index)}
                style={{ flexShrink: 0 }}
              />
              <span style={{ color: "var(--color-text-secondary)", fontFamily: "monospace", fontSize: 12, flexShrink: 0 }}>
                [{formatTime(segment.start)} &ndash; {formatTime(segment.end)}]
              </span>
              <span style={{ color: "var(--color-text-secondary)", fontSize: 12 }}>
                {(segment.end - segment.start).toFixed(1)}s
              </span>
            </label>
          ))}
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
              background: "transparent", color: "var(--color-text-secondary)",
              border: "1px solid var(--color-border)", borderRadius: 4,
              padding: "8px 16px", fontSize: 13, fontWeight: 600, cursor: "pointer",
            }}
          >
            Cancel
          </button>
          <button
            onClick={handleRemove}
            disabled={checked.size === 0 || removing}
            style={{
              background: "var(--color-accent)", color: "#fff", borderRadius: 4,
              padding: "8px 16px", fontSize: 13, fontWeight: 600, border: "none",
              cursor: (checked.size === 0 || removing) ? "default" : "pointer",
              opacity: (checked.size === 0 || removing) ? 0.5 : 1,
            }}
          >
            {removing ? "Removing..." : `Remove ${checked.size} pause${checked.size !== 1 ? "s" : ""} (${savedSeconds.toFixed(1)}s)`}
          </button>
        </div>
      </div>
    </div>
  );
}
