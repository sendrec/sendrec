import { useEffect, useState } from "react";
import ReactMarkdown from "react-markdown";

interface DocumentModalProps {
  document: string;
  onClose: () => void;
}

export function DocumentModal({ document, onClose }: DocumentModalProps) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  async function copyToClipboard() {
    await navigator.clipboard.writeText(document);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
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
        borderRadius: 12, padding: 24, width: 720, maxWidth: "90vw", maxHeight: "80vh", overflow: "auto",
      }}>
        <h2 style={{ color: "var(--color-text)", fontSize: 18, margin: "0 0 16px" }}>Generated Document</h2>
        <div style={{ textAlign: "left", lineHeight: 1.6, color: "var(--color-text)" }}>
          <ReactMarkdown>{document}</ReactMarkdown>
        </div>
        <div style={{ marginTop: 16, display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <button
            onClick={copyToClipboard}
            style={{
              background: "var(--color-accent)", color: "#fff", borderRadius: 4,
              padding: "8px 16px", fontSize: 13, fontWeight: 600, border: "none", cursor: "pointer",
            }}
          >
            {copied ? "Copied!" : "Copy to clipboard"}
          </button>
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
    </div>
  );
}
