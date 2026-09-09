import { useI18n } from "../i18n/I18nContext";
import type { UiLanguage } from "../i18n/translations";

interface LanguageSelectProps {
  compact?: boolean;
}

export function LanguageSelect({ compact = false }: LanguageSelectProps) {
  const { language, languages, setLanguage, t } = useI18n();

  return (
    <label className={compact ? "language-select language-select--compact" : "language-select"}>
      {!compact && <span className="form-label">{t("language.label")}</span>}
      <select
        className={compact ? "language-select-control language-select-control--compact" : "form-input language-select-control"}
        value={language}
        onChange={(event) => setLanguage(event.target.value as UiLanguage)}
        aria-label={t("language.label")}
      >
        {languages.map((item) => (
          <option key={item.code} value={item.code}>
            {compact ? item.shortLabel : item.label}
          </option>
        ))}
      </select>
    </label>
  );
}
