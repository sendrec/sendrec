import { type ReactNode } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { setAccessToken } from "../api/client";

interface LayoutProps {
  children: ReactNode;
}

export function Layout({ children }: LayoutProps) {
  const location = useLocation();
  const navigate = useNavigate();

  function isActive(path: string): boolean {
    return location.pathname === path;
  }

  function signOut() {
    setAccessToken(null);
    navigate("/login");
  }

  const activeLinkStyle = {
    color: "var(--color-text)",
    fontWeight: 700 as const,
    fontSize: 14,
    textDecoration: "none" as const,
  };

  const inactiveLinkStyle = {
    color: "var(--color-text-secondary)",
    fontWeight: 400 as const,
    fontSize: 14,
    textDecoration: "none" as const,
  };

  return (
    <>
      <nav
        style={{
          display: "flex",
          alignItems: "center",
          gap: 24,
          padding: "0 24px",
          height: 56,
          borderBottom: "1px solid var(--color-border)",
        }}
      >
        <span
          style={{
            fontWeight: 700,
            fontSize: "1.125rem",
            color: "var(--color-text)",
            marginRight: 16,
          }}
        >
          SendRec
        </span>

        <Link to="/" style={isActive("/") ? activeLinkStyle : inactiveLinkStyle}>
          Record
        </Link>

        <Link
          to="/library"
          style={isActive("/library") ? activeLinkStyle : inactiveLinkStyle}
        >
          Library
        </Link>

        <button
          onClick={signOut}
          style={{
            marginLeft: "auto",
            background: "transparent",
            color: "var(--color-text-secondary)",
            fontSize: 14,
            padding: "6px 12px",
            borderRadius: 4,
          }}
        >
          Sign out
        </button>
      </nav>

      <main>{children}</main>
    </>
  );
}
