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
      <div className="auth-brand">
        <span className="auth-logo">
          <span className="auth-logo-send">Send</span>
          <span className="auth-logo-rec">Rec</span>
        </span>
      </div>
      <div className="auth-card auth-centered">
        {status === "loading" && (
          <h1>Confirming your email...</h1>
        )}

        {status === "success" && (
          <>
            <h1>Email confirmed</h1>
            <p className="auth-subtitle">
              Your account is now active. You can sign in.
            </p>
            <div className="auth-footer">
              <Link to="/login">Sign in</Link>
            </div>
          </>
        )}

        {status === "error" && (
          <>
            <h1>Confirmation failed</h1>
            <div className="auth-error-banner">
              {errorMessage}
            </div>
            <div className="auth-footer">
              <Link to="/register">Try again</Link>
            </div>
          </>
        )}
      </div>
    </main>
  );
}
