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
        throw new Error(data.error || "Something went wrong");
      }

      setSent(true);
      setCooldown(60);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
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
      <div className="auth-card auth-centered">
        <h1>Check your email</h1>
        <p className="auth-subtitle">
          We&apos;ve sent a confirmation link to <strong>{email}</strong>. Click
          the link to activate your account. The link expires in 24 hours.
        </p>

        {sent && (
          <p className="auth-success-text">
            Confirmation email resent.
          </p>
        )}

        {error && (
          <div className="auth-error-banner">
            {error}
          </div>
        )}

        <button onClick={handleResend} disabled={cooldown > 0}>
          {cooldown > 0 ? `Resend in ${cooldown}s` : "Resend confirmation email"}
        </button>

        <div className="auth-footer">
          <Link to="/login">Back to sign in</Link>
        </div>
      </div>
    </main>
  );
}
