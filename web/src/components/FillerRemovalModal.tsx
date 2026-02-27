import { useEffect, useRef, useState } from "react";
import { apiFetch } from "../api/client";
import { useFocusTrap } from "../hooks/useFocusTrap";

interface TranscriptSegment {
  start: number;
  end: number;
  text: string;
}

interface FillerRemovalModalProps {
  videoId: string;
  shareToken: string;
  duration: number;
  onClose: () => void;
  onRemovalStarted: () => void;
}

const FILLER_PATTERN = /^\s*(?:um+|uh+|uhh+|hmm+|ah+|er+|you know|like|so|basically|actually|right|i mean)[.,!?\s]*$/i;

function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${String(s).padStart(2, "0")}`;
}

export function FillerRemovalModal({ videoId, shareToken, onClose, onRemovalStarted }: FillerRemovalModalProps) {
  const [segments, setSegments] = useState<TranscriptSegment[]>([]);
  const [loadError, setLoadError] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [checked, setChecked] = useState<Set<number>>(new Set());
  const [removing, setRemoving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const contentRef = useRef<HTMLDivElement>(null);

  useFocusTrap(contentRef);

  useEffect(() => {
    fetch(`/api/watch/${shareToken}`)
      .then(resp => resp.ok ? resp.json() : null)
      .then(data => {
        if (data?.segments) {
          setSegments(data.segments);
          const fillerIndices = new Set<number>();
          (data.segments as TranscriptSegment[]).forEach((seg, i) => {
            if (FILLER_PATTERN.test(seg.text)) fillerIndices.add(i);
          });
          setChecked(fillerIndices);
        } else {
          setLoadError(true);
        }
        setLoaded(true);
      })
      .catch(() => {
        setLoadError(true);
        setLoaded(true);
      });
  }, [shareToken]);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  const fillers = segments
    .map((seg, index) => ({ index, segment: seg }))
    .filter(({ segment }) => FILLER_PATTERN.test(segment.text));

  const totalSeconds = fillers.reduce((sum, f) => sum + (f.segment.end - f.segment.start), 0);
  const savedSeconds = fillers
    .filter(f => checked.has(f.index))
    .reduce((sum, f) => sum + (f.segment.end - f.segment.start), 0);

  function toggleFiller(index: number) {
    setChecked(prev => {
      const next = new Set(prev);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  }

  function selectAll() {
    setChecked(new Set(fillers.map(f => f.index)));
  }

  function deselectAll() {
    setChecked(new Set());
  }

  async function handleRemove() {
    setRemoving(true);
    setError(null);
    try {
      const selectedSegments = fillers
        .filter(f => checked.has(f.index))
        .map(f => ({ start: f.segment.start, end: f.segment.end }));
      await apiFetch(`/api/videos/${videoId}/remove-segments`, {
        method: "POST",
        body: JSON.stringify({ segments: selectedSegments }),
      });
      onRemovalStarted();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to remove fillers");
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
        <div role="dialog" aria-modal="true" aria-label="Remove Filler Words" style={{
          background: "var(--color-surface)", border: "1px solid var(--color-border)",
          borderRadius: 12, padding: 24, width: 500, maxWidth: "90vw",
        }}>
          <p style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>Loading transcript...</p>
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
        <div role="dialog" aria-modal="true" aria-label="Remove Filler Words" style={{
          background: "var(--color-surface)", border: "1px solid var(--color-border)",
          borderRadius: 12, padding: 24, width: 500, maxWidth: "90vw",
        }}>
          <h2 style={{ color: "var(--color-text)", fontSize: 18, margin: "0 0 16px" }}>Remove Filler Words</h2>
          <p style={{ color: "var(--color-text-secondary)", fontSize: 14, margin: "0 0 16px" }}>
            Unable to load transcript.
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

  if (fillers.length === 0) {
    return (
      <div
        style={{
          position: "fixed", inset: 0, background: "var(--color-overlay)",
          display: "flex", alignItems: "center", justifyContent: "center", zIndex: 1000,
        }}
        onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
      >
        <div role="dialog" aria-modal="true" aria-label="Remove Filler Words" style={{
          background: "var(--color-surface)", border: "1px solid var(--color-border)",
          borderRadius: 12, padding: 24, width: 500, maxWidth: "90vw",
        }}>
          <h2 style={{ color: "var(--color-text)", fontSize: 18, margin: "0 0 16px" }}>Remove Filler Words</h2>
          <p style={{ color: "var(--color-text-secondary)", fontSize: 14, margin: "0 0 16px" }}>
            No filler words detected in this video's transcript.
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
      <div ref={contentRef} role="dialog" aria-modal="true" aria-label="Remove Filler Words" style={{
        background: "var(--color-surface)", border: "1px solid var(--color-border)",
        borderRadius: 12, padding: 24, width: 500, maxWidth: "90vw",
      }}>
        <h2 style={{ color: "var(--color-text)", fontSize: 18, margin: "0 0 12px" }}>
          Remove Filler Words
        </h2>

        <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 12px" }}>
          Found {fillers.length} filler word{fillers.length !== 1 ? "s" : ""} ({totalSeconds.toFixed(1)}s total)
        </p>

        <button
          onClick={checked.size === fillers.length ? deselectAll : selectAll}
          style={{
            background: "transparent", color: "var(--color-accent)", border: "none",
            padding: 0, fontSize: 12, cursor: "pointer", marginBottom: 8,
          }}
        >
          {checked.size === fillers.length ? "Deselect all" : "Select all"}
        </button>

        <div style={{ maxHeight: 300, overflowY: "auto", border: "1px solid var(--color-border)", borderRadius: 8, marginBottom: 16 }}>
          {fillers.map((filler) => (
            <label
              key={filler.index}
              style={{
                display: "flex", alignItems: "center", gap: 8, padding: "6px 10px",
                cursor: "pointer", fontSize: 13,
                borderBottom: "1px solid var(--color-border)",
              }}
            >
              <input
                type="checkbox"
                checked={checked.has(filler.index)}
                onChange={() => toggleFiller(filler.index)}
                style={{ flexShrink: 0 }}
              />
              <span style={{ color: "var(--color-text-secondary)", fontFamily: "monospace", fontSize: 12, flexShrink: 0 }}>
                [{formatTime(filler.segment.start)}]
              </span>
              <span style={{ color: "var(--color-error)" }}>
                {filler.segment.text}
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
            {removing ? "Removing..." : `Remove ${checked.size} filler${checked.size !== 1 ? "s" : ""} (${savedSeconds.toFixed(1)}s)`}
          </button>
        </div>
      </div>
    </div>
  );
}
