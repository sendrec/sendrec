import { type ReactNode, useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { apiFetch, setAccessToken } from "../api/client";
import { useTheme } from "../hooks/useTheme";

interface BillingResponse {
  plan: string;
}

interface LayoutProps {
  children: ReactNode;
}

export function Layout({ children }: LayoutProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const [menuOpen, setMenuOpen] = useState(false);
  const [plan, setPlan] = useState<string | null>(null);
  const { resolvedTheme, setTheme } = useTheme();

  function toggleTheme() {
    setTheme(resolvedTheme === "dark" ? "light" : "dark");
  }

  useEffect(() => {
    apiFetch<BillingResponse>("/api/settings/billing")
      .then((res) => { if (res?.plan) setPlan(res.plan); else setPlan("free"); })
      .catch(() => setPlan("free"));
  }, []);

  function isActive(path: string): boolean {
    if (path === "/analytics") {
      return location.pathname === "/analytics" || location.pathname.endsWith("/analytics");
    }
    return location.pathname === path;
  }

  async function signOut() {
    await fetch("/api/auth/logout", { method: "POST", credentials: "include" }).catch(() => {});
    setAccessToken(null);
    navigate("/login");
  }

  function handleNavClick() {
    setMenuOpen(false);
  }

  return (
    <>
      <nav className="nav-bar">
        <Link to="/" className="nav-logo" onClick={handleNavClick}>
          <img src="/images/logo.png" alt="SendRec" width="48" height="48" />
          SendRec
          {plan && (
            <span className={`plan-badge${plan === "pro" ? " plan-badge--pro" : ""}`}>
              {plan === "pro" ? "Pro" : "Free"}
            </span>
          )}
        </Link>

        <div className={`nav-links${menuOpen ? " nav-links--open" : ""}`}>
          <Link
            to="/"
            className={`nav-link${isActive("/") ? " nav-link--active" : ""}`}
            onClick={handleNavClick}
          >
            Record
          </Link>

          <Link
            to="/library"
            className={`nav-link${isActive("/library") ? " nav-link--active" : ""}`}
            onClick={handleNavClick}
          >
            Library
          </Link>

          <Link
            to="/playlists"
            className={`nav-link${isActive("/playlists") ? " nav-link--active" : ""}`}
            onClick={handleNavClick}
          >
            Playlists
          </Link>

          <Link
            to="/analytics"
            className={`nav-link${isActive("/analytics") ? " nav-link--active" : ""}`}
            onClick={handleNavClick}
          >
            Analytics
          </Link>

          <Link
            to="/settings"
            className={`nav-link${isActive("/settings") ? " nav-link--active" : ""}`}
            onClick={handleNavClick}
          >
            Settings
          </Link>

          <button className="nav-signout" onClick={signOut}>
            Sign out
          </button>
        </div>

        <button
          className="nav-theme-toggle"
          onClick={toggleTheme}
          aria-label="Toggle theme"
        >
          {resolvedTheme === "dark" ? (
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
            </svg>
          ) : (
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="5"/>
              <line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/>
              <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/>
              <line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/>
              <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
            </svg>
          )}
        </button>

        <button
          className="nav-hamburger"
          onClick={() => setMenuOpen(!menuOpen)}
          aria-label="Toggle menu"
        >
          <span />
          <span />
          <span />
        </button>
      </nav>

      <main>{children}</main>
    </>
  );
}
