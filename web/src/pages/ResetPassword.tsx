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
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 24,
            textAlign: "center",
          }}
        >
          <h1
            style={{
              color: "var(--color-text)",
              fontSize: 24,
              marginBottom: 16,
            }}
          >
            Invalid reset link
          </h1>
          <p
            style={{
              color: "var(--color-text-secondary)",
              fontSize: 14,
              marginBottom: 24,
            }}
          >
            This password reset link is invalid. Please request a new one.
          </p>
          <Link
            to="/forgot-password"
            style={{ color: "var(--color-accent)", fontSize: 14 }}
          >
            Request new reset link
          </Link>
        </div>
      </main>
    );
  }

  if (success) {
    return (
      <main className="auth-container">
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 24,
            textAlign: "center",
          }}
        >
          <h1
            style={{
              color: "var(--color-text)",
              fontSize: 24,
              marginBottom: 16,
            }}
          >
            Password updated
          </h1>
          <p
            style={{
              color: "var(--color-text-secondary)",
              fontSize: 14,
              marginBottom: 24,
            }}
          >
            Your password has been reset successfully.
          </p>
          <Link
            to="/login"
            style={{ color: "var(--color-accent)", fontSize: 14 }}
          >
            Sign in
          </Link>
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
    <main style={{ maxWidth: 400, margin: "80px auto", padding: 24 }}>
      <h1
        style={{
          color: "var(--color-text)",
          fontSize: 24,
          marginBottom: 24,
          textAlign: "center",
        }}
      >
        Set new password
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
        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span
            style={{ color: "var(--color-text-secondary)", fontSize: 14 }}
          >
            New password
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

        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span
            style={{ color: "var(--color-text-secondary)", fontSize: 14 }}
          >
            Confirm password
          </span>
          <input
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
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
          <p style={{ color: "var(--color-error)", fontSize: 14, margin: 0 }}>
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
          {loading ? "Updating..." : "Reset password"}
        </button>

        <p
          style={{
            color: "var(--color-text-secondary)",
            fontSize: 14,
            textAlign: "center",
            margin: 0,
          }}
        >
          <Link
            to="/forgot-password"
            style={{ color: "var(--color-accent)" }}
          >
            Request new reset link
          </Link>
        </p>
      </form>
    </main>
  );
}
