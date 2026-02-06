import { Link } from "react-router-dom";

export function NotFound() {
  return (
    <main
      style={{
        maxWidth: 400,
        margin: "120px auto",
        padding: 24,
        textAlign: "center",
      }}
    >
      <h1
        style={{
          color: "var(--color-text)",
          fontSize: 48,
          marginBottom: 8,
        }}
      >
        404
      </h1>
      <p
        style={{
          color: "var(--color-text-secondary)",
          fontSize: 16,
          marginBottom: 24,
        }}
      >
        Page not found
      </p>
      <Link
        to="/"
        style={{
          background: "var(--color-accent)",
          color: "var(--color-text)",
          borderRadius: 8,
          padding: "10px 24px",
          fontSize: 14,
          fontWeight: 600,
          textDecoration: "none",
        }}
      >
        Go home
      </Link>
    </main>
  );
}
