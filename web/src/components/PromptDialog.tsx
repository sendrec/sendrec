import { useEffect, useRef, useState } from "react";
import { useFocusTrap } from "../hooks/useFocusTrap";

interface PromptDialogProps {
  title: string;
  onSubmit: (value: string) => void;
  onCancel: () => void;
  placeholder?: string;
  submitLabel?: string;
}

export function PromptDialog({
  title,
  onSubmit,
  onCancel,
  placeholder = "",
  submitLabel = "Submit",
}: PromptDialogProps) {
  const [value, setValue] = useState("");
  const contentRef = useRef<HTMLDivElement>(null);

  useFocusTrap(contentRef);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onCancel();
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onCancel]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!value.trim()) return;
    onSubmit(value.trim());
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
        aria-label={title}
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: 12,
          padding: 24,
          width: 400,
          maxWidth: "90vw",
        }}
      >
        <p
          style={{
            color: "var(--color-text)",
            fontSize: 15,
            fontWeight: 600,
            margin: "0 0 12px",
          }}
        >
          {title}
        </p>
        <form onSubmit={handleSubmit}>
          <input
            type="text"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder={placeholder}
            aria-label={title}
            autoFocus
            style={{
              width: "100%",
              padding: "8px 12px",
              fontSize: 14,
              background: "var(--color-background)",
              border: "1px solid var(--color-border)",
              borderRadius: 4,
              color: "var(--color-text)",
              boxSizing: "border-box",
            }}
          />
          <div
            style={{
              display: "flex",
              gap: 8,
              justifyContent: "flex-end",
              marginTop: 16,
            }}
          >
            <button
              type="button"
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
              type="submit"
              disabled={!value.trim()}
              style={{
                background: "var(--color-accent)",
                color: "#fff",
                borderRadius: 4,
                padding: "8px 16px",
                fontSize: 13,
                fontWeight: 600,
                border: "none",
                cursor: "pointer",
                opacity: value.trim() ? 1 : 0.5,
              }}
            >
              {submitLabel}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
