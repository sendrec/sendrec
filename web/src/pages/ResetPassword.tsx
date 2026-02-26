import { type FormEvent, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";

export function ResetPassword() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get("token");

  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState(false);

  if (!token) {
    return (
      <main className="auth-container">
        <div className="auth-brand">
          <span className="auth-logo">
            <span className="auth-logo-send">Send</span>
            <span className="auth-logo-rec">Rec</span>
          </span>
        </div>
        <div className="auth-card auth-centered">
          <h1>Invalid reset link</h1>
          <p className="auth-subtitle">
            This password reset link is invalid. Please request a new one.
          </p>
          <div className="auth-footer">
            <Link to="/forgot-password">Request new reset link</Link>
          </div>
        </div>
      </main>
    );
  }

  if (success) {
    return (
      <main className="auth-container">
        <div className="auth-brand">
          <span className="auth-logo">
            <span className="auth-logo-send">Send</span>
            <span className="auth-logo-rec">Rec</span>
          </span>
        </div>
        <div className="auth-card auth-centered">
          <h1>Password updated</h1>
          <p className="auth-subtitle">
            Your password has been reset successfully.
          </p>
          <div className="auth-footer">
            <Link to="/login">Sign in</Link>
          </div>
        </div>
      </main>
    );
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");

    if (password !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }

    if (password.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }

    setLoading(true);

    try {
      const response = await fetch("/api/auth/reset-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token, password }),
      });

      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error || "Something went wrong");
      }

      setSuccess(true);
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
        <h1>Set new password</h1>

        <label>
          <span>New password</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
          />
        </label>

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

        {error && (
          <div className="auth-error-banner">
            {error}
          </div>
        )}

        <button type="submit" disabled={loading}>
          {loading ? "Updating..." : "Reset password"}
        </button>

        <div className="auth-footer">
          <Link to="/forgot-password">Request new reset link</Link>
        </div>
      </form>
    </main>
  );
}
