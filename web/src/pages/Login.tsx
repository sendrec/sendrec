import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { ApiError, apiFetch, setAccessToken } from "../api/client";
import { AuthForm } from "../components/AuthForm";
import { providerLabel } from "../utils/sso";

export function Login() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [registrationEnabled, setRegistrationEnabled] = useState(true);
  const [ssoProviders, setSsoProviders] = useState<string[]>([]);
  const [ssoError, setSsoError] = useState("");

  useEffect(() => {
    fetch("/api/health")
      .then((res) => res.json())
      .then((data: { registrationEnabled?: boolean }) => {
        setRegistrationEnabled(data.registrationEnabled !== false);
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    fetch("/api/auth/sso/providers")
      .then((res) => {
        if (!res.ok) return;
        return res.json();
      })
      .then((data: { providers?: string[] } | undefined) => {
        if (data?.providers?.length) {
          setSsoProviders(data.providers);
        }
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const token = params.get("sso_token");
    const error = params.get("sso_error");

    if (token) {
      setAccessToken(token);
      window.history.replaceState({}, "", window.location.pathname);
      const redirect = params.get("redirect");
      navigate(redirect || "/");
      return;
    }

    if (error) {
      setSsoError(error);
      window.history.replaceState({}, "", window.location.pathname);
    }
  }, [navigate]);

  const [ssoEmail, setSsoEmail] = useState("");

  async function handleLogin(data: {
    email: string;
    password: string;
  }) {
    setSsoEmail("");
    try {
      const result = await apiFetch<{ accessToken: string }>(
        "/api/auth/login",
        {
          method: "POST",
          body: JSON.stringify({
            email: data.email,
            password: data.password,
          }),
        }
      );

      if (result) {
        setAccessToken(result.accessToken);
        const redirect = searchParams.get("redirect");
        navigate(redirect || "/");
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 403 && err.message === "email_not_verified") {
        navigate("/check-email", { state: { email: data.email } });
        return;
      }
      if (err instanceof ApiError && err.status === 403 && err.message === "sso_required") {
        setSsoEmail(data.email);
        throw new Error("Your workspace requires SSO sign-in. Use the button below.");
      }
      throw err;
    }
  }

  const redirect = searchParams.get("redirect");
  const registerPath = redirect ? `/register?redirect=${encodeURIComponent(redirect)}` : "/register";

  const ssoSection = (ssoProviders.length > 0 || ssoError || ssoEmail) ? (
    <>
      {ssoError && (
        <div className="auth-error-banner">{ssoError}</div>
      )}
      {(ssoProviders.length > 0 || ssoEmail) && (
        <>
          <div className="auth-divider">or</div>
          <div className="sso-buttons">
            {ssoEmail && (
              <a
                href={`/api/auth/sso/org?email=${encodeURIComponent(ssoEmail)}`}
                className="btn btn--secondary btn--sso"
              >
                Sign in with workspace SSO
              </a>
            )}
            {ssoProviders.map((provider) => (
              <a
                key={provider}
                href={`/api/auth/sso/${provider}`}
                className="btn btn--secondary btn--sso"
              >
                Continue with {providerLabel(provider)}
              </a>
            ))}
          </div>
        </>
      )}
    </>
  ) : null;

  return (
    <AuthForm
      title="Sign in"
      submitLabel="Sign in"
      onSubmit={handleLogin}
      afterSubmit={ssoSection}
      footer={
        <>
          <Link to="/forgot-password" className="auth-footer-link-block">
            Forgot password?
          </Link>
          {registrationEnabled && (
            <>
              Don&apos;t have an account? <Link to={registerPath}>Sign up</Link>
            </>
          )}
        </>
      }
    />
  );
}
