import { type FormEvent, useState, useEffect } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";

export function CheckEmail() {
  const location = useLocation();
  const navigate = useNavigate();
  const email = (location.state as { email?: string })?.email;
  const [cooldown, setCooldown] = useState(0);
  const [error, setError] = useState("");
  const [sent, setSent] = useState(false);

  useEffect(() => {
    if (!email) {
      navigate("/register", { replace: true });
    }
  }, [email, navigate]);

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setTimeout(() => setCooldown(cooldown - 1), 1000);
    return () => clearTimeout(timer);
  }, [cooldown]);

  if (!email) return null;

  async function handleResend(event: FormEvent) {
    event.preventDefault();
    setError("");
    setSent(false);

    try {
      const response = await fetch("/api/auth/resend-confirmation", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });

      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error || "Etwas ist schiefgelaufen");
      }

      setSent(true);
      setCooldown(60);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Etwas ist schiefgelaufen");
    }
  }

  return (
    <main className="auth-container">
      <div className="auth-brand">
        <span className="auth-logo">
          <span className="auth-logo-send">99tools</span>
          <span className="auth-logo-rec"> Record</span>
        </span>
      </div>
      <div className="auth-card auth-centered">
        <h1>Prüfe dein E-Mail-Postfach</h1>
        <p className="auth-subtitle">
          Wir haben einen Bestätigungslink an <strong>{email}</strong> gesendet. Klicke
          auf den Link, um dein Konto zu aktivieren. Der Link ist 24 Stunden gültig.
        </p>

        {sent && (
          <p className="auth-success-text">
            Bestätigungs-E-Mail erneut gesendet.
          </p>
        )}

        {error && (
          <div className="auth-error-banner">
            {error}
          </div>
        )}

        <button onClick={handleResend} disabled={cooldown > 0}>
          {cooldown > 0 ? `Erneut senden in ${cooldown}s` : "Bestätigungs-E-Mail erneut senden"}
        </button>

        <div className="auth-footer">
          <Link to="/login">Zurück zur Anmeldung</Link>
        </div>
      </div>
    </main>
  );
}
