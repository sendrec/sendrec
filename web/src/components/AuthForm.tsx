import { type FormEvent, type ReactNode, useState } from "react";

interface AuthFormProps {
  title: string;
  submitLabel: string;
  showName?: boolean;
  onSubmit: (data: {
    email: string;
    password: string;
    name: string;
  }) => Promise<void>;
  footer: ReactNode;
}

export function AuthForm({
  title,
  submitLabel,
  showName,
  onSubmit,
  footer,
}: AuthFormProps) {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");
    setLoading(true);

    try {
      await onSubmit({ email, password, name });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div
      style={{
        maxWidth: 400,
        margin: "80px auto",
        padding: 24,
      }}
    >
      <h1
        style={{
          color: "var(--color-text)",
          fontSize: 24,
          marginBottom: 24,
          textAlign: "center",
        }}
      >
        {title}
      </h1>

      <form
        onSubmit={handleSubmit}
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: 8,
          padding: 24,
          display: "flex",
          flexDirection: "column",
          gap: 16,
        }}
      >
        {showName && (
          <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>
              Name
            </span>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              style={{
                background: "var(--color-bg)",
                border: "1px solid var(--color-border)",
                borderRadius: 4,
                color: "var(--color-text)",
                padding: "8px 12px",
                fontSize: 14,
              }}
            />
          </label>
        )}

        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>
            Email
          </span>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            style={{
              background: "var(--color-bg)",
              border: "1px solid var(--color-border)",
              borderRadius: 4,
              color: "var(--color-text)",
              padding: "8px 12px",
              fontSize: 14,
            }}
          />
        </label>

        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>
            Password
          </span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
            style={{
              background: "var(--color-bg)",
              border: "1px solid var(--color-border)",
              borderRadius: 4,
              color: "var(--color-text)",
              padding: "8px 12px",
              fontSize: 14,
            }}
          />
        </label>

        {error && (
          <p
            style={{
              color: "var(--color-error)",
              fontSize: 14,
              margin: 0,
            }}
          >
            {error}
          </p>
        )}

        <button
          type="submit"
          disabled={loading}
          style={{
            background: "var(--color-accent)",
            color: "var(--color-text)",
            borderRadius: 4,
            padding: "10px 16px",
            fontSize: 14,
            fontWeight: 600,
            opacity: loading ? 0.7 : 1,
          }}
        >
          {loading ? "Loading..." : submitLabel}
        </button>

        <p
          style={{
            color: "var(--color-text-secondary)",
            fontSize: 14,
            textAlign: "center",
            margin: 0,
          }}
        >
          {footer}
        </p>
      </form>
    </div>
  );
}
