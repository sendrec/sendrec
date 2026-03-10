export function Toast({ message }: { message: string | null }) {
  if (!message) return null;
  return (
    <div
      role="status"
      aria-live="polite"
      style={{
        position: "fixed",
        bottom: 24,
        left: "50%",
        transform: "translateX(-50%)",
        background: "var(--color-surface)",
        color: "var(--color-text)",
        border: "1px solid var(--color-border)",
        borderRadius: 8,
        padding: "10px 20px",
        fontSize: 14,
        fontWeight: 500,
        zIndex: 200,
        boxShadow: "0 4px 16px var(--color-shadow)",
        pointerEvents: "none" as const,
      }}
    >
      {message}
    </div>
  );
}
