import { Link, useNavigate } from "react-router-dom";
import { apiFetch, setAccessToken } from "../api/client";
import { AuthForm } from "../components/AuthForm";

export function Login() {
  const navigate = useNavigate();

  async function handleLogin(data: {
    email: string;
    password: string;
  }) {
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
      navigate("/");
    }
  }

  return (
    <AuthForm
      title="Sign in"
      submitLabel="Sign in"
      onSubmit={handleLogin}
      footer={
        <>
          Don&apos;t have an account? <Link to="/register">Sign up</Link>
        </>
      }
    />
  );
}
