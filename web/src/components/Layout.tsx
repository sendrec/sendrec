import { type ReactNode, useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { apiFetch, setAccessToken } from "../api/client";

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

  useEffect(() => {
    apiFetch<BillingResponse>("/api/settings/billing")
      .then((res) => { if (res?.plan) setPlan(res.plan); })
      .catch(() => {});
  }, []);

  function isActive(path: string): boolean {
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
        </Link>

        {plan && (
          <span className={`plan-badge${plan === "pro" ? " plan-badge--pro" : ""}`}>
            {plan === "pro" ? "Pro" : "Free"}
          </span>
        )}

        <button
          className="nav-hamburger"
          onClick={() => setMenuOpen(!menuOpen)}
          aria-label="Toggle menu"
        >
          <span />
          <span />
          <span />
        </button>

        <div className={`nav-links${menuOpen ? " nav-links--open" : ""}`}>
          <Link
            to="/"
            className={`nav-link${isActive("/") ? " nav-link--active" : ""}`}
            onClick={handleNavClick}
          >
            Record
          </Link>

          <Link
            to="/upload"
            className={`nav-link${isActive("/upload") ? " nav-link--active" : ""}`}
            onClick={handleNavClick}
          >
            Upload
          </Link>

          <Link
            to="/library"
            className={`nav-link${isActive("/library") ? " nav-link--active" : ""}`}
            onClick={handleNavClick}
          >
            Library
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
      </nav>

      <main>{children}</main>
    </>
  );
}
