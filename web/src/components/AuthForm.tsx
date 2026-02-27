import { type FormEvent, type ReactNode, useState } from "react";
import { useTheme } from "../hooks/useTheme";

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
  const { resolvedTheme, setTheme } = useTheme();

  function toggleTheme() {
    setTheme(resolvedTheme === "dark" ? "light" : "dark");
  }

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
      <button
        className="auth-theme-toggle"
        onClick={toggleTheme}
        aria-label="Toggle theme"
      >
        {resolvedTheme === "dark" ? (
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
          </svg>
        ) : (
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="12" cy="12" r="5"/>
            <line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/>
            <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/>
            <line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/>
            <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
          </svg>
        )}
      </button>
      <div className="auth-brand">
        <img src="/images/logo.png" alt="" width="48" height="48" className="auth-logo-img" />
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
            <span className="form-hint">
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
          <div className="auth-error-banner">
            {error}
          </div>
        )}

        <button type="submit" disabled={loading}>
          {loading ? "Loading..." : submitLabel}
        </button>

        <div className="auth-footer">
          {footer}
        </div>
      </form>
    </main>
  );
}
