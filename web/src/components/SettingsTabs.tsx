import { Link, useLocation } from "react-router-dom";
import type { Organization } from "../hooks/useOrganization";

interface Tab {
  to: string;
  label: string;
  icon: string | null;
}

// One Settings page with a tab per scope: your account, always — security and
// sign-in live there, so they stay reachable from inside a workspace — and the
// selected workspace for its owners and admins. Each tab is its own route, so
// existing links keep working. #262.
//
// The page passes in the workspace it already loaded, so the tab bar cannot
// disagree with the page it sits on.
export function SettingsTabs({ workspace }: { workspace: Organization | null }) {
  const pathname = useLocation().pathname.replace(/\/+$/, "");

  const tabs: Tab[] = [{ to: "/settings", label: "Account", icon: null }];
  if (workspace && (workspace.role === "owner" || workspace.role === "admin")) {
    tabs.push({ to: `/organizations/${workspace.id}/settings`, label: workspace.name, icon: workspace.icon ?? null });
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
