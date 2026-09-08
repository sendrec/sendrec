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
        throw new Error(data.error || "Etwas ist schiefgelaufen");
      }

      setSent(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Etwas ist schiefgelaufen");
    } finally {
      setLoading(false);
    }
  }

  if (sent) {
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
            Wenn ein Konto mit dieser E-Mail-Adresse existiert, haben wir einen Link
            zum Zurücksetzen des Passworts gesendet. Der Link ist 1 Stunde gültig.
          </p>
          <div className="auth-footer">
            <Link to="/login">Zurück zur Anmeldung</Link>
          </div>
        </div>
      </main>
    );
  }

  return (
    <main className="auth-container">
      <div className="auth-brand">
        <span className="auth-logo">
          <span className="auth-logo-send">99tools</span>
          <span className="auth-logo-rec"> Record</span>
        </span>
      </div>
      <form onSubmit={handleSubmit} className="auth-card">
        <h1>Passwort zurücksetzen</h1>

        <label>
          <span>E-Mail</span>
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
          {loading ? "Wird gesendet..." : "Link zum Zurücksetzen senden"}
        </button>

        <div className="auth-footer">
          <Link to="/login">Zurück zur Anmeldung</Link>
        </div>
      </form>
    </main>
  );
}
