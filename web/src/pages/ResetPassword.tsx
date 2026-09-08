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
            <span className="auth-logo-send">99tools</span>
            <span className="auth-logo-rec"> Record</span>
          </span>
        </div>
        <div className="auth-card auth-centered">
          <h1>Ungültiger Link zum Zurücksetzen</h1>
          <p className="auth-subtitle">
            Dieser Link zum Zurücksetzen des Passworts ist ungültig. Bitte fordere einen neuen an.
          </p>
          <div className="auth-footer">
            <Link to="/forgot-password">Neuen Link zum Zurücksetzen anfordern</Link>
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
            <span className="auth-logo-send">99tools</span>
            <span className="auth-logo-rec"> Record</span>
          </span>
        </div>
        <div className="auth-card auth-centered">
          <h1>Passwort aktualisiert</h1>
          <p className="auth-subtitle">
            Dein Passwort wurde erfolgreich zurückgesetzt.
          </p>
          <div className="auth-footer">
            <Link to="/login">Anmelden</Link>
          </div>
        </div>
      </main>
    );
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");

    if (password !== confirmPassword) {
      setError("Die Passwörter stimmen nicht überein");
      return;
    }

    if (password.length < 8) {
      setError("Das Passwort muss mindestens 8 Zeichen lang sein");
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
        throw new Error(data.error || "Etwas ist schiefgelaufen");
      }

      setSuccess(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Etwas ist schiefgelaufen");
    } finally {
      setLoading(false);
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
      <form onSubmit={handleSubmit} className="auth-card">
        <h1>Neues Passwort festlegen</h1>

        <label>
          <span>Neues Passwort</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
          />
        </label>

        <label>
          <span>Passwort bestätigen</span>
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
          {loading ? "Wird aktualisiert..." : "Passwort zurücksetzen"}
        </button>

        <div className="auth-footer">
          <Link to="/forgot-password">Neuen Link zum Zurücksetzen anfordern</Link>
        </div>
      </form>
    </main>
  );
}
