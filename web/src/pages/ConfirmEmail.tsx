import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";

export function ConfirmEmail() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get("token");

  const [status, setStatus] = useState<"loading" | "success" | "error">(token ? "loading" : "error");
  const [errorMessage, setErrorMessage] = useState(token ? "" : "Missing confirmation token.");

  useEffect(() => {
    if (!token) return;

    async function confirm() {
      try {
        const response = await fetch("/api/auth/confirm-email", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ token }),
        });

        if (!response.ok) {
          const data = await response.json();
          throw new Error(data.error || "Confirmation failed");
        }

        setStatus("success");
      } catch (err) {
        setErrorMessage(err instanceof Error ? err.message : "Confirmation failed");
        setStatus("error");
      }
    }

    confirm();
  }, [token]);

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
        {status === "loading" && (
          <>
            <h1
              style={{
                color: "var(--color-text)",
                fontSize: 24,
                marginBottom: 16,
              }}
            >
              Confirming your email...
            </h1>
          </>
        )}

        {status === "success" && (
          <>
            <h1
              style={{
                color: "var(--color-text)",
                fontSize: 24,
                marginBottom: 16,
              }}
            >
              Email confirmed
            </h1>
            <p
              style={{
                color: "var(--color-text-secondary)",
                fontSize: 14,
                marginBottom: 24,
              }}
            >
              Your account is now active. You can sign in.
            </p>
            <Link
              to="/login"
              style={{ color: "var(--color-accent)", fontSize: 14 }}
            >
              Sign in
            </Link>
          </>
        )}

        {status === "error" && (
          <>
            <h1
              style={{
                color: "var(--color-text)",
                fontSize: 24,
                marginBottom: 16,
              }}
            >
              Confirmation failed
            </h1>
            <p
              style={{
                color: "var(--color-error)",
                fontSize: 14,
                marginBottom: 24,
              }}
            >
              {errorMessage}
            </p>
            <Link
              to="/register"
              style={{ color: "var(--color-accent)", fontSize: 14 }}
            >
              Try again
            </Link>
          </>
        )}
      </div>
    </main>
  );
}
