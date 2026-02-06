import { useEffect, useState } from "react";
import { Routes, Route, Navigate } from "react-router-dom";
import { getAccessToken, tryRefreshToken } from "./api/client";
import { Layout } from "./components/Layout";
import { Login } from "./pages/Login";
import { Library } from "./pages/Library";
import { Record } from "./pages/Record";
import { Register } from "./pages/Register";
import { ForgotPassword } from "./pages/ForgotPassword";
import { ResetPassword } from "./pages/ResetPassword";
import { NotFound } from "./pages/NotFound";
import { Settings } from "./pages/Settings";

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const [checking, setChecking] = useState(!getAccessToken());
  const [authenticated, setAuthenticated] = useState(!!getAccessToken());

  useEffect(() => {
    if (!getAccessToken()) {
      tryRefreshToken().then((ok) => {
        setAuthenticated(ok);
        setChecking(false);
      });
    }
  }, []);

  if (checking) {
    return null;
  }

  if (!authenticated) {
    return <Navigate to="/login" replace />;
  }

  return <Layout>{children}</Layout>;
}

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/register" element={<Register />} />
      <Route path="/forgot-password" element={<ForgotPassword />} />
      <Route path="/reset-password" element={<ResetPassword />} />
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <Record />
          </ProtectedRoute>
        }
      />
      <Route
        path="/library"
        element={
          <ProtectedRoute>
            <Library />
          </ProtectedRoute>
        }
      />
      <Route
        path="/settings"
        element={
          <ProtectedRoute>
            <Settings />
          </ProtectedRoute>
        }
      />
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
}
