import { type ReactNode, useEffect, useRef, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { apiFetch, setAccessToken } from "../api/client";
import { useTheme } from "../hooks/useTheme";
import { useOrganization } from "../hooks/useOrganization";
import { useI18n } from "../i18n/I18nContext";
import { LanguageSelect } from "./LanguageSelect";

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
  const [planBadgeEnabled, setPlanBadgeEnabled] = useState(false);
  const { resolvedTheme, setTheme } = useTheme();
  const { t } = useI18n();
  const { orgs, selectedOrg, selectedOrgId, switchOrg, createOrg } = useOrganization();
  const [orgDropdownOpen, setOrgDropdownOpen] = useState(false);
  const [creatingWorkspace, setCreatingWorkspace] = useState(false);
  const [newWorkspaceName, setNewWorkspaceName] = useState("");
  const [createError, setCreateError] = useState<string | null>(null);
  const [activeIndex, setActiveIndex] = useState(-1);
  const orgDropdownRef = useRef<HTMLDivElement>(null);
  const createInputRef = useRef<HTMLInputElement>(null);

  function toggleTheme() {
    setTheme(resolvedTheme === "dark" ? "light" : "dark");
  }

  useEffect(() => {
    apiFetch<BillingResponse>("/api/settings/billing")
      .then((res) => { if (res?.plan) setPlan(res.plan); else setPlan("free"); })
      .catch(() => setPlan("free"));
    fetch("/api/health")
      .then((res) => res.json())
      .then((data: { planBadgeEnabled?: boolean }) => {
        setPlanBadgeEnabled(data.planBadgeEnabled === true);
      })
      .catch(() => setPlanBadgeEnabled(false));
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

  async function handleCreateWorkspace() {
    const name = newWorkspaceName.trim();
    if (!name) return;
    try {
      await createOrg(name);
      setCreatingWorkspace(false);
      setNewWorkspaceName("");
      setCreateError(null);
      setOrgDropdownOpen(false);
    } catch {
      setCreateError(t("workspace.createError"));
    }
  }

  function handleDropdownKeyDown(e: React.KeyboardEvent) {
    if (!orgDropdownOpen) {
      if (e.key === "ArrowDown" || e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        setOrgDropdownOpen(true);
        setActiveIndex(0);
      }
      return;
    }
    const totalItems = orgs.length + 2;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((prev) => (prev + 1) % totalItems);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((prev) => (prev - 1 + totalItems) % totalItems);
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (activeIndex === 0) { switchOrg(null); setOrgDropdownOpen(false); }
      else if (activeIndex <= orgs.length) { switchOrg(orgs[activeIndex - 1].id); setOrgDropdownOpen(false); }
      else if (activeIndex === orgs.length + 1) { setCreatingWorkspace(true); setCreateError(null); }
    }
  }

  useEffect(() => {
    if (!orgDropdownOpen) return;
    function handleClickOutside(e: MouseEvent) {
      if (orgDropdownRef.current && !orgDropdownRef.current.contains(e.target as Node)) {
        setOrgDropdownOpen(false);
        setCreatingWorkspace(false);
        setNewWorkspaceName("");
        setCreateError(null);
      }
    }
    function handleEscape(e: KeyboardEvent) {
      if (e.key === "Escape") {
        setOrgDropdownOpen(false);
        setCreatingWorkspace(false);
        setNewWorkspaceName("");
        setCreateError(null);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    document.addEventListener("keydown", handleEscape);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleEscape);
    };
  }, [orgDropdownOpen]);

  useEffect(() => {
    if (creatingWorkspace && createInputRef.current) {
      createInputRef.current.focus();
    }
  }, [creatingWorkspace]);

  return (
    <>
      <nav className="nav-bar">
        <Link to="/" className="nav-logo" onClick={handleNavClick}>
          <img src="/images/logo-99tools.png" alt="" width="48" height="48" />
          <span className="logo-send">99tools</span><span className="logo-rec"> Record</span>
          {plan && planBadgeEnabled && (
            <span className={`plan-badge${plan !== "free" ? " plan-badge--pro" : ""}`}>
              {plan === "business" ? "Business" : plan === "pro" ? "Pro" : t("common.free")}
            </span>
          )}
        </Link>

        <div className="org-switcher" ref={orgDropdownRef} onKeyDown={handleDropdownKeyDown}>
          <button
            className="org-switcher-trigger"
            onClick={() => {
              setOrgDropdownOpen((prev) => !prev);
              setCreatingWorkspace(false);
              setNewWorkspaceName("");
              setCreateError(null);
              setActiveIndex(-1);
            }}
            aria-haspopup="listbox"
            aria-expanded={orgDropdownOpen}
            aria-label={t("workspace.switch")}
          >
            <span className="org-switcher-label">
              {selectedOrgId
                ? orgs.find((o) => o.id === selectedOrgId)?.name ?? t("workspace.personal")
                : t("workspace.personal")}
            </span>
            <svg className="org-switcher-chevron" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M4 6l4 4 4-4" />
            </svg>
          </button>

          {orgDropdownOpen && (
            <>
              <div className="org-switcher-backdrop" onClick={() => setOrgDropdownOpen(false)} />
              <div className="org-switcher-menu" role="listbox" aria-label={t("workspace.list")}>
                {/* Personal */}
                <button
                  className={`org-switcher-item${!selectedOrgId ? " org-switcher-item--active" : ""}${activeIndex === 0 ? " org-switcher-item--focused" : ""}`}
                  role="option"
                  aria-selected={!selectedOrgId}
                  onClick={() => { switchOrg(null); setOrgDropdownOpen(false); }}
                >
                  <svg className="org-switcher-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                    <circle cx="8" cy="5" r="3" />
                    <path d="M2 14c0-3.3 2.7-6 6-6s6 2.7 6 6" />
                  </svg>
                  <span className="org-switcher-item-name">{t("workspace.personal")}</span>
                </button>

                {/* Orgs */}
                {orgs.map((org, i) => (
                  <button
                    key={org.id}
                    className={`org-switcher-item${selectedOrgId === org.id ? " org-switcher-item--active" : ""}${activeIndex === i + 1 ? " org-switcher-item--focused" : ""}`}
                    role="option"
                    aria-selected={selectedOrgId === org.id}
                    onClick={() => { switchOrg(org.id); setOrgDropdownOpen(false); }}
                  >
                    <svg className="org-switcher-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                      <rect x="1" y="4" width="14" height="11" rx="1" />
                      <path d="M4 4V2a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2" />
                    </svg>
                    <span className="org-switcher-item-name">{org.name}</span>
                    <span className="org-switcher-role-badge">{org.role}</span>
                  </button>
                ))}

                <div className="org-switcher-divider" />

                {!creatingWorkspace ? (
                  <button
                    className={`org-switcher-item org-switcher-item--action${activeIndex === orgs.length + 1 ? " org-switcher-item--focused" : ""}`}
                    onClick={() => { setCreatingWorkspace(true); setCreateError(null); }}
                  >
                    <svg className="org-switcher-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                      <line x1="8" y1="3" x2="8" y2="13" />
                      <line x1="3" y1="8" x2="13" y2="8" />
                    </svg>
                    <span className="org-switcher-item-name">{t("workspace.new")}</span>
                  </button>
                ) : (
                  <div className="org-switcher-create">
                    <input
                      ref={createInputRef}
                      className="org-switcher-create-input"
                      type="text"
                      placeholder={t("workspace.namePlaceholder")}
                      value={newWorkspaceName}
                      onChange={(e) => { setNewWorkspaceName(e.target.value); setCreateError(null); }}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" && newWorkspaceName.trim()) handleCreateWorkspace();
                        if (e.key === "Escape") { setCreatingWorkspace(false); setNewWorkspaceName(""); setCreateError(null); }
                        e.stopPropagation();
                      }}
                      autoFocus
                      maxLength={200}
                    />
                    {createError && <span className="org-switcher-create-error">{createError}</span>}
                  </div>
                )}
              </div>
            </>
          )}
        </div>

        <div className={`nav-links${menuOpen ? " nav-links--open" : ""}`}>
          {selectedOrg?.role !== "viewer" && (
            <Link
              to="/"
              className={`nav-link${isActive("/") ? " nav-link--active" : ""}`}
              onClick={handleNavClick}
            >
              {t("nav.record")}
            </Link>
          )}

          <Link
            to="/library"
            className={`nav-link${isActive("/library") ? " nav-link--active" : ""}`}
            onClick={handleNavClick}
          >
            {t("nav.library")}
          </Link>

          <Link
            to="/playlists"
            className={`nav-link${isActive("/playlists") ? " nav-link--active" : ""}`}
            onClick={handleNavClick}
          >
            {t("nav.playlists")}
          </Link>

          <Link
            to="/analytics"
            className={`nav-link${isActive("/analytics") ? " nav-link--active" : ""}`}
            onClick={handleNavClick}
          >
            {t("nav.analytics")}
          </Link>

          <Link
            to="/settings"
            className={`nav-link${isActive("/settings") ? " nav-link--active" : ""}`}
            onClick={handleNavClick}
          >
            {t("nav.settings")}
          </Link>

          {selectedOrg && (selectedOrg.role === "owner" || selectedOrg.role === "admin") && (
            <Link
              to={`/organizations/${selectedOrgId}/settings`}
              className="nav-link"
              onClick={handleNavClick}
            >
              {t("nav.workspaceSettings")}
            </Link>
          )}

          <button
            className="nav-theme-toggle"
            onClick={toggleTheme}
            aria-label={t("nav.theme")}
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

          <LanguageSelect compact />

          <button className="nav-signout" onClick={signOut}>
            {t("nav.signOut")}
          </button>
        </div>

        <button
          className="nav-hamburger"
          onClick={() => setMenuOpen(!menuOpen)}
          aria-label={t("nav.menu")}
        >
          <span />
          <span />
          <span />
        </button>
      </nav>

      <main key={selectedOrgId ?? "personal"}>{children}</main>
    </>
  );
}
