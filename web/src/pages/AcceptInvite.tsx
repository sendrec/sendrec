import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { apiFetch, getAccessToken, tryRefreshToken } from "../api/client";

type PageState = "checking" | "unauthenticated" | "accepting" | "success" | "error";

export function AcceptInvite() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const token = searchParams.get("token");

  const [pageState, setPageState] = useState<PageState>(token ? "checking" : "error");
  const [errorMessage, setErrorMessage] = useState(token ? "" : "Missing invite token.");

  useEffect(() => {
    if (!token) return;

    async function checkAuthAndAccept() {
      let authenticated = !!getAccessToken();

      if (!authenticated) {
        authenticated = await tryRefreshToken();
      }

      if (!authenticated) {
        setPageState("unauthenticated");
        return;
      }

      setPageState("accepting");
      await acceptInvite();
    }

    async function acceptInvite() {
      try {
        await apiFetch("/api/invites/accept", {
          method: "POST",
          body: JSON.stringify({ token }),
        });
        setPageState("success");
        setTimeout(() => navigate("/"), 2000);
      } catch (err) {
        setErrorMessage(err instanceof Error ? err.message : "Failed to accept invite");
        setPageState("error");
      }
    }

    checkAuthAndAccept();
  }, [token, navigate]);

  const redirectPath = `/invites/accept?token=${encodeURIComponent(token ?? "")}`;

  return (
    <main className="auth-container">
      <div className="auth-brand">
        <span className="auth-logo">
          <span className="auth-logo-send">Send</span>
          <span className="auth-logo-rec">Rec</span>
        </span>
      </div>
      <div className="auth-card auth-centered">
        {pageState === "checking" && (
          <h1>Checking authentication...</h1>
        )}

        {pageState === "unauthenticated" && (
          <>
            <h1>You've been invited</h1>
            <p className="auth-subtitle">
              You've been invited to join an organization. Sign in or create an account to accept.
            </p>
            <div className="auth-footer" style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              <Link to={`/login?redirect=${encodeURIComponent(redirectPath)}`} className="btn btn--primary" style={{ display: "block", textAlign: "center" }}>
                Sign in
              </Link>
              <Link to={`/register?redirect=${encodeURIComponent(redirectPath)}`}>
                Create account
              </Link>
            </div>
          </>
        )}

        {pageState === "accepting" && (
          <h1>Accepting invite...</h1>
        )}

        {pageState === "success" && (
          <>
            <h1>Invite accepted</h1>
            <p className="auth-subtitle">
              You have joined the organization. Redirecting...
            </p>
          </>
        )}

        {pageState === "error" && (
          <>
            <h1>Invite failed</h1>
            <div className="auth-error-banner">
              {errorMessage}
            </div>
            <div className="auth-footer">
              <Link to="/">Go to dashboard</Link>
            </div>
          </>
        )}
      </div>
    </main>
  );
}
