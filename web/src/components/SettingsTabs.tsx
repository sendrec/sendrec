import { Link, useLocation } from "react-router-dom";
import { useOrganization } from "../hooks/useOrganization";

interface Tab {
  to: string;
  label: string;
  icon: string | null;
}

// One Settings page with a tab per scope: your account, always — security and
// sign-in live there, so they stay reachable from inside a workspace — and the
// selected workspace for its owners and admins. Each tab is its own route, so
// existing links keep working. #262.
export function SettingsTabs() {
  const { selectedOrg } = useOrganization();
  const { pathname } = useLocation();

  const tabs: Tab[] = [{ to: "/settings", label: "Account", icon: null }];
  if (selectedOrg && (selectedOrg.role === "owner" || selectedOrg.role === "admin")) {
    tabs.push({ to: `/organizations/${selectedOrg.id}/settings`, label: selectedOrg.name, icon: selectedOrg.icon ?? null });
  }

  return (
    <nav className="settings-tabs" aria-label="Settings">
      {tabs.map((tab) => {
        const current = pathname === tab.to;
        return (
          <Link
            key={tab.to}
            to={tab.to}
            className={`settings-tab${current ? " settings-tab--active" : ""}`}
            aria-current={current ? "page" : undefined}
          >
            {tab.icon && <span aria-hidden="true">{tab.icon}</span>}
            {tab.label}
          </Link>
        );
      })}
    </nav>
  );
}
