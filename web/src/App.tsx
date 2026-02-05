import { Routes, Route, Navigate } from "react-router-dom";
import { getAccessToken } from "./api/client";
import { Layout } from "./components/Layout";
import { Login } from "./pages/Login";
import { Library } from "./pages/Library";
import { Record } from "./pages/Record";
import { Register } from "./pages/Register";

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  if (!getAccessToken()) {
    return <Navigate to="/login" replace />;
  }
  return <Layout>{children}</Layout>;
}

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/register" element={<Register />} />
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
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
