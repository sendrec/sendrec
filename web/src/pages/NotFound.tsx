import { Link } from "react-router-dom";
import { useI18n } from "../i18n/I18nContext";

export function NotFound() {
  const { t } = useI18n();
  return (
    <div className="error-page-content">
      <main className="error-block">
        <div className="error-illustration" aria-hidden="true">
          <svg
            width="64"
            height="48"
            viewBox="0 0 64 48"
            fill="none"
          >
            <g opacity="0.35" stroke="var(--color-text-secondary)" strokeWidth="2.5" strokeLinecap="square">
              <polyline points="4,20 4,4 20,4" />
              <polyline points="44,4 60,4 60,20" />
              <polyline points="4,30 4,44 20,44" />
              <polyline points="44,44 60,44 60,30" />
            </g>
            <g opacity="0.45" stroke="var(--color-text-secondary)" strokeWidth="2" strokeLinecap="round">
              <line x1="26" y1="18" x2="38" y2="30" />
              <line x1="38" y1="18" x2="26" y2="30" />
            </g>
          </svg>
        </div>

        <p className="error-code" aria-hidden="true">404</p>
        <h1 className="error-heading">{t("notFound.title")}</h1>
        <p className="error-subtext">
          {t("notFound.text")}
        </p>

        <div className="error-btn-group">
          <Link to="/library" className="error-btn error-btn--primary">
            {t("record.library")}
          </Link>
          <Link to="/" className="error-btn error-btn--secondary">
            {t("notFound.record")}
          </Link>
        </div>
      </main>
    </div>
  );
}
