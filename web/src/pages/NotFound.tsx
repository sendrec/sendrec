import { Link } from "react-router-dom";

export function NotFound() {
  return (
    <main
      style={{
        maxWidth: 400,
        margin: "120px auto",
        padding: 24,
        textAlign: "center",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
      }}
    >
      <svg
        width="64"
        height="48"
        viewBox="0 0 64 48"
        fill="none"
        aria-hidden="true"
        style={{ marginBottom: 32 }}
      >
        <g opacity="0.35" stroke="var(--color-text-secondary)" strokeWidth="2.5" strokeLinecap="square">
          <polyline points="4,20 4,4 20,4" />
          <polyline points="44,4 60,4 60,20" />
          <polyline points="4,30 4,44 20,44" />
          <polyline points="44,44 60,44 60,30" />
        </g>
        <g opacity="0.45" stroke="var(--color-text-secondary)" strokeWidth="2" strokeLinecap="round">
          <line x1="26" y1="18" x2="38" y2="30" />
          <line x1="38" y1="18" x2="26" y2="30" />
        </g>
      </svg>

      <p
        style={{
          fontSize: 72,
          fontWeight: 700,
          lineHeight: 1,
          color: "var(--color-text-secondary)",
          letterSpacing: -2,
          marginBottom: 16,
          opacity: 0.5,
        }}
        aria-hidden="true"
      >
        404
      </p>

      <h1
        style={{
          fontSize: 24,
          fontWeight: 700,
          color: "var(--color-text)",
          marginBottom: 8,
          lineHeight: 1.2,
        }}
      >
        Page not found
      </h1>

      <p
        style={{
          fontSize: 14,
          color: "var(--color-text-secondary)",
          lineHeight: 1.6,
          marginBottom: 32,
          maxWidth: 320,
        }}
      >
        The page you're looking for doesn't exist or has been moved.
      </p>

      <div style={{ display: "flex", gap: 10, flexWrap: "wrap", justifyContent: "center" }}>
        <Link
          to="/library"
          className="detail-btn detail-btn--accent"
        >
          Go to Library
        </Link>
        <Link
          to="/"
          className="detail-btn"
        >
          Start Recording
        </Link>
      </div>
    </main>
  );
}
