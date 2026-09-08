import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { apiFetch, getAccessToken, tryRefreshToken } from "../api/client";

type PageState = "checking" | "unauthenticated" | "accepting" | "success" | "error";

export function AcceptInvite() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const token = searchParams.get("token");

  const [pageState, setPageState] = useState<PageState>(token ? "checking" : "error");
  const [errorMessage, setErrorMessage] = useState(token ? "" : "Einladungstoken fehlt.");

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
        setErrorMessage(err instanceof Error ? err.message : "Einladung konnte nicht angenommen werden");
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
          <span className="auth-logo-send">99tools</span>
          <span className="auth-logo-rec"> Record</span>
        </span>
      </div>
      <div className="auth-card auth-centered">
        {pageState === "checking" && (
          <h1>Anmeldung wird geprüft...</h1>
        )}

        {pageState === "unauthenticated" && (
          <>
            <h1>Du wurdest eingeladen</h1>
            <p className="auth-subtitle">
              Du wurdest zu einem Arbeitsbereich eingeladen. Melde dich an oder erstelle ein Konto, um die Einladung anzunehmen.
            </p>
            <div className="auth-footer" style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              <Link to={`/login?redirect=${encodeURIComponent(redirectPath)}`} className="btn btn--primary" style={{ display: "block", textAlign: "center" }}>
                Anmelden
              </Link>
              <Link to={`/register?redirect=${encodeURIComponent(redirectPath)}`}>
                Konto erstellen
              </Link>
            </div>
          </>
        )}

        {pageState === "accepting" && (
          <h1>Einladung wird angenommen...</h1>
        )}

        {pageState === "success" && (
          <>
            <h1>Einladung angenommen</h1>
            <p className="auth-subtitle">
              Du bist dem Arbeitsbereich beigetreten. Weiterleitung...
            </p>
          </>
        )}

        {pageState === "error" && (
          <>
            <h1>Einladung fehlgeschlagen</h1>
            <div className="auth-error-banner">
              {errorMessage}
            </div>
            <div className="auth-footer">
              <Link to="/">Zum Dashboard</Link>
            </div>
          </>
        )}
      </div>
    </main>
  );
}
