import { type FormEvent, useState } from "react";
import { Link } from "react-router-dom";

export function ForgotPassword() {
  const [email, setEmail] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [sent, setSent] = useState(false);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");
    setLoading(true);

    try {
      const response = await fetch("/api/auth/forgot-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });

      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error || "Something went wrong");
      }

      setSent(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }

  if (sent) {
    return (
      <main className="auth-container">
        <div className="auth-brand">
          <span className="auth-logo">
            <span className="auth-logo-send">Send</span>
            <span className="auth-logo-rec">Rec</span>
          </span>
        </div>
        <div className="auth-card" style={{ textAlign: "center" }}>
          <h1>Check your email</h1>
          <p style={{ color: "var(--color-text-secondary)", fontSize: 14 }}>
            If an account with that email exists, we&apos;ve sent a password
            reset link. The link expires in 1 hour.
          </p>
          <Link to="/login" style={{ color: "var(--color-accent)", fontSize: 14 }}>
            Back to sign in
          </Link>
        </div>
      </main>
    );
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
        <h1>Reset password</h1>

        <label>
          <span>Email</span>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </label>

        {error && (
          <p style={{ color: "var(--color-error)", fontSize: 14, margin: 0 }}>
            {error}
          </p>
        )}

        <button type="submit" disabled={loading}>
          {loading ? "Sending..." : "Send reset link"}
        </button>

        <p
          style={{
            color: "var(--color-text-secondary)",
            fontSize: 14,
            textAlign: "center",
            margin: 0,
          }}
        >
          <Link to="/login" style={{ color: "var(--color-accent)" }}>
            Back to sign in
          </Link>
        </p>
      </form>
    </main>
  );
}
