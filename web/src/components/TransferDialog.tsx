import { useEffect, useRef, useState } from "react";
import { apiFetch } from "../api/client";
import { useOrganization } from "../hooks/useOrganization";
import { useFocusTrap } from "../hooks/useFocusTrap";

interface TransferDialogProps {
  videoId: string;
  videoTitle: string;
  onTransferred: () => void;
  onCancel: () => void;
}

export function TransferDialog({ videoId, videoTitle, onTransferred, onCancel }: TransferDialogProps) {
  const { orgs, selectedOrgId } = useOrganization();
  const [targetOrgId, setTargetOrgId] = useState<string | null>(null);
  const [transferring, setTransferring] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const contentRef = useRef<HTMLDivElement>(null);

  useFocusTrap(contentRef);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onCancel();
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onCancel]);

  // Build destination options: personal + all non-viewer workspaces, excluding current scope
  const destinations: { id: string | null; label: string }[] = [];
  if (selectedOrgId) {
    destinations.push({ id: null, label: "Personal" });
  }
  for (const org of orgs) {
    if (org.id === selectedOrgId) continue;
    if (org.role === "viewer") continue;
    destinations.push({ id: org.id, label: org.name });
  }

  async function handleTransfer() {
    setTransferring(true);
    setError(null);
    try {
      await apiFetch(`/api/videos/${videoId}/transfer`, {
        method: "POST",
        body: JSON.stringify({ organizationId: targetOrgId }),
      });
      onTransferred();
    } catch {
      setError("Failed to transfer video");
      setTransferring(false);
    }
  }

  return (
    <div
      style={{
        position: "fixed",
        inset: 0,
        background: "var(--color-overlay)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 1000,
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onCancel();
      }}
    >
      <div
        ref={contentRef}
        role="dialog"
        aria-modal="true"
        aria-label="Transfer video"
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: 12,
          padding: 24,
          width: 400,
          maxWidth: "90vw",
        }}
      >
        <h3 style={{ color: "var(--color-text)", fontSize: 16, fontWeight: 600, margin: "0 0 4px" }}>
          Move video
        </h3>
        <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 16px" }}>
          {videoTitle}
        </p>

        {destinations.length === 0 ? (
          <p style={{ color: "var(--color-text-secondary)", fontSize: 14, margin: "0 0 16px" }}>
            No other workspaces available.
          </p>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 6, marginBottom: 16 }}>
            {destinations.map((dest) => (
              <label
                key={dest.id ?? "personal"}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 8,
                  padding: "8px 12px",
                  borderRadius: 8,
                  border: `1px solid ${targetOrgId === dest.id ? "var(--color-accent)" : "var(--color-border)"}`,
                  background: targetOrgId === dest.id ? "var(--color-accent-soft)" : "transparent",
                  cursor: "pointer",
                  fontSize: 14,
                  color: "var(--color-text)",
                }}
              >
                <input
                  type="radio"
                  name="transfer-dest"
                  checked={targetOrgId === dest.id}
                  onChange={() => { setTargetOrgId(dest.id); setError(null); }}
                  style={{ accentColor: "var(--color-accent)" }}
                />
                {dest.label}
              </label>
            ))}
          </div>
        )}

        {error && (
          <p style={{ color: "var(--color-error)", fontSize: 13, margin: "0 0 12px" }}>{error}</p>
        )}

        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <button
            onClick={onCancel}
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
            onClick={handleTransfer}
            disabled={transferring || (targetOrgId === null && !selectedOrgId)}
            style={{
              background: "var(--color-accent)",
              color: "#fff",
              borderRadius: 4,
              padding: "8px 16px",
              fontSize: 13,
              fontWeight: 600,
              border: "none",
              cursor: transferring ? "not-allowed" : "pointer",
              opacity: (destinations.length === 0 || transferring) ? 0.5 : 1,
            }}
          >
            {transferring ? "Moving..." : "Move"}
          </button>
        </div>
      </div>
    </div>
  );
}
