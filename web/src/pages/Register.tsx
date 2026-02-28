import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { apiFetch } from "../api/client";
import { AuthForm } from "../components/AuthForm";

export function Register() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  async function handleRegister(data: {
    email: string;
    password: string;
    name: string;
  }) {
    await apiFetch<{ message: string }>(
      "/api/auth/register",
      {
        method: "POST",
        body: JSON.stringify({
          email: data.email,
          password: data.password,
          name: data.name,
        }),
      }
    );

    navigate("/check-email", { state: { email: data.email } });
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
