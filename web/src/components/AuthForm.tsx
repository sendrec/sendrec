import { type FormEvent, type ReactNode, useState } from "react";

interface AuthFormProps {
  title: string;
  submitLabel: string;
  showName?: boolean;
  showPasswordConfirm?: boolean;
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
  showPasswordConfirm,
  onSubmit,
  footer,
}: AuthFormProps) {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");

    if (showPasswordConfirm && password !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }

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
    <main className="auth-container">
      <div className="auth-brand">
        <span className="auth-logo">
          <span className="auth-logo-send">Send</span>
          <span className="auth-logo-rec">Rec</span>
        </span>
      </div>

      <form onSubmit={handleSubmit} className="auth-card">
        <h1>{title}</h1>

        {showName && (
          <label>
            <span>Name</span>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </label>
        )}

        <label>
          <span>Email</span>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </label>

        <label>
          <span>Password</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
          />
          {showPasswordConfirm && (
            <span style={{ fontSize: 12, marginTop: 2 }}>
              Must be at least 8 characters
            </span>
          )}
        </label>

        {showPasswordConfirm && (
          <label>
            <span>Confirm password</span>
            <input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              minLength={8}
            />
          </label>
        )}

        {error && (
          <p style={{ color: "var(--color-error)", fontSize: 14, margin: 0 }}>
            {error}
          </p>
        )}

        <button type="submit" disabled={loading}>
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
    </main>
  );
}
