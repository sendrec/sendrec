import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { apiFetch } from "../api/client";
import { AuthForm } from "../components/AuthForm";

export function Register() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    fetch("/api/health")
      .then((res) => res.json())
      .then((data: { registrationEnabled?: boolean }) => {
        if (data.registrationEnabled === false) {
          navigate("/login", { replace: true });
        } else {
          setReady(true);
        }
      })
      .catch(() => setReady(true));
  }, [navigate]);

  if (!ready) return null;

  async function handleRegister(data: {
    email: string;
    password: string;
    name: string;
  }) {
    const res = await apiFetch<{
      message: string;
      requiresEmailConfirmation?: boolean;
    }>("/api/auth/register", {
      method: "POST",
      body: JSON.stringify({
        email: data.email,
        password: data.password,
        name: data.name,
      }),
    });

    // Strict `=== false` is intentional: undefined (older server, malformed
    // response) routes to /check-email so the user is told to look for the
    // confirmation link, matching the pre-flag behaviour. Only an explicit
    // `false` short-circuits to /login.
    if (res?.requiresEmailConfirmation === false) {
      navigate("/login", { state: { email: data.email, justRegistered: true } });
    } else {
      navigate("/check-email", { state: { email: data.email } });
    }
  }

  const redirect = searchParams.get("redirect");
  const loginPath = redirect ? `/login?redirect=${encodeURIComponent(redirect)}` : "/login";

  return (
    <AuthForm
      title="Create account"
      submitLabel="Create account"
      showName
      showPasswordConfirm
      onSubmit={handleRegister}
      footer={
        <>
          Already have an account? <Link to={loginPath}>Sign in</Link>
        </>
      }
    />
  );
}
