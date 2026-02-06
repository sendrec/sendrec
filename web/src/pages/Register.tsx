import { Link, useNavigate } from "react-router-dom";
import { apiFetch, setAccessToken } from "../api/client";
import { AuthForm } from "../components/AuthForm";

export function Register() {
  const navigate = useNavigate();

  async function handleRegister(data: {
    email: string;
    password: string;
    name: string;
  }) {
    const result = await apiFetch<{ accessToken: string }>(
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

    if (result) {
      setAccessToken(result.accessToken);
      navigate("/");
    }
  }

  return (
    <AuthForm
      title="Create account"
      submitLabel="Create account"
      showName
      showPasswordConfirm
      onSubmit={handleRegister}
      footer={
        <>
          Already have an account? <Link to="/login">Sign in</Link>
        </>
      }
    />
  );
}
