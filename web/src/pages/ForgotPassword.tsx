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
        <div className="auth-card auth-centered">
          <h1>Check your email</h1>
          <p className="auth-subtitle">
            If an account with that email exists, we&apos;ve sent a password
            reset link. The link expires in 1 hour.
          </p>
          <div className="auth-footer">
            <Link to="/login">Back to sign in</Link>
          </div>
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
          <div className="auth-error-banner">
            {error}
          </div>
        )}

        <button type="submit" disabled={loading}>
          {loading ? "Sending..." : "Send reset link"}
        </button>

        <div className="auth-footer">
          <Link to="/login">Back to sign in</Link>
        </div>
      </form>
    </main>
  );
}
