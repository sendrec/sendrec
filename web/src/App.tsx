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
import { CheckEmail } from "./pages/CheckEmail";
import { ConfirmEmail } from "./pages/ConfirmEmail";
import { NotFound } from "./pages/NotFound";
import { Settings } from "./pages/Settings";
import { Analytics } from "./pages/Analytics";
import { VideoDetail } from "./pages/VideoDetail";
import { Upload } from "./pages/Upload";
import { Playlists } from "./pages/Playlists";
import { PlaylistDetail } from "./pages/PlaylistDetail";
import { useTheme } from "./hooks/useTheme";

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
  useTheme();
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/register" element={<Register />} />
      <Route path="/forgot-password" element={<ForgotPassword />} />
      <Route path="/reset-password" element={<ResetPassword />} />
      <Route path="/check-email" element={<CheckEmail />} />
      <Route path="/confirm-email" element={<ConfirmEmail />} />
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <Record />
          </ProtectedRoute>
        }
      />
      <Route
        path="/upload"
        element={
          <ProtectedRoute>
            <Upload />
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
        path="/playlists"
        element={
          <ProtectedRoute>
            <Playlists />
          </ProtectedRoute>
        }
      />
      <Route
        path="/playlists/:id"
        element={
          <ProtectedRoute>
            <PlaylistDetail />
          </ProtectedRoute>
        }
      />
      <Route
        path="/videos/:id"
        element={
          <ProtectedRoute>
            <VideoDetail />
          </ProtectedRoute>
        }
      />
      <Route
        path="/videos/:id/analytics"
        element={
          <ProtectedRoute>
            <Analytics />
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
