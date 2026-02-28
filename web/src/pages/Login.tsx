import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { ApiError, apiFetch, setAccessToken } from "../api/client";
import { AuthForm } from "../components/AuthForm";

export function Login() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  async function handleLogin(data: {
    email: string;
    password: string;
  }) {
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
      throw err;
    }
  }

  const redirect = searchParams.get("redirect");
  const registerPath = redirect ? `/register?redirect=${encodeURIComponent(redirect)}` : "/register";

  return (
    <AuthForm
      title="Sign in"
      submitLabel="Sign in"
      onSubmit={handleLogin}
      footer={
        <>
          <Link to="/forgot-password" className="auth-footer-link-block">
            Forgot password?
          </Link>
          Don&apos;t have an account? <Link to={registerPath}>Sign up</Link>
        </>
      }
    />
  );
}
