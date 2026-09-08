import { type FormEvent, useState } from "react";
import { apiFetch } from "../../api/client";
import { useTheme } from "../../hooks/useTheme";
import { useUnsavedChanges } from "../../hooks/useUnsavedChanges";
import { TRANSCRIPTION_LANGUAGES } from "../../constants/languages";
import { UserProfile } from "./types";
import { useI18n } from "../../i18n/I18nContext";
import { LanguageSelect } from "../../components/LanguageSelect";

interface ProfileSectionProps {
  profile: UserProfile;
  transcriptionEnabled: boolean;
  noiseReductionEnabled: boolean;
  initialTranscriptionLanguage: string;
  initialNoiseReduction: boolean;
  initialRetentionDays: number;
}

export function ProfileSection({
  profile,
  transcriptionEnabled,
  noiseReductionEnabled,
  initialTranscriptionLanguage,
  initialNoiseReduction,
  initialRetentionDays,
}: ProfileSectionProps) {
  const { theme, setTheme } = useTheme();
  const { t } = useI18n();
  const [name, setName] = useState(profile.name);
  const [nameMessage, setNameMessage] = useState("");
  const [nameError, setNameError] = useState("");
  const [savingName, setSavingName] = useState(false);
  const [transcriptionLanguage, setTranscriptionLanguage] = useState(initialTranscriptionLanguage);
  const [noiseReduction, setNoiseReduction] = useState(initialNoiseReduction);
  const [retentionDays, setRetentionDays] = useState(initialRetentionDays);
  const [currentProfile, setCurrentProfile] = useState(profile);

  const nameIsDirty = name !== currentProfile.name;
  useUnsavedChanges(nameIsDirty);

  async function handleNameSubmit(event: FormEvent) {
    event.preventDefault();
    setNameError("");
    setNameMessage("");

    if (!name.trim()) {
      setNameError(t("settings.nameRequired"));
      return;
    }

    setSavingName(true);
    try {
      await apiFetch("/api/user", {
        method: "PATCH",
        body: JSON.stringify({ name: name.trim() }),
      });
      setNameMessage(t("settings.nameUpdated"));
      setCurrentProfile((prev) => ({ ...prev, name: name.trim() }));
    } catch (err) {
      setNameError(err instanceof Error ? err.message : t("settings.nameUpdateFailed"));
    } finally {
      setSavingName(false);
    }
  }

  async function handleTranscriptionLanguageChange(value: string) {
    const previous = transcriptionLanguage;
    setTranscriptionLanguage(value);
    try {
      await apiFetch("/api/user", {
        method: "PATCH",
        body: JSON.stringify({ transcriptionLanguage: value }),
      });
    } catch {
      setTranscriptionLanguage(previous);
    }
  }

  async function handleNoiseReductionChange(enabled: boolean) {
    const previous = noiseReduction;
    setNoiseReduction(enabled);
    try {
      await apiFetch("/api/user", {
        method: "PATCH",
        body: JSON.stringify({ noiseReduction: enabled }),
      });
    } catch {
      setNoiseReduction(previous);
    }
  }

  async function handleRetentionDaysChange(value: number) {
    const previous = retentionDays;
    setRetentionDays(value);
    try {
      await apiFetch("/api/user", {
        method: "PATCH",
        body: JSON.stringify({ retentionDays: value }),
      });
    } catch {
      setRetentionDays(previous);
    }
  }

  return (
    <>
      <form
        onSubmit={handleNameSubmit}
        className="card settings-section"
      >
        <h2>{t("settings.profile")}</h2>

        <div className="form-field">
          <label className="form-label" htmlFor="profile-email">{t("auth.email")}</label>
          <input
            id="profile-email"
            type="email"
            className="form-input"
            value={profile.email}
            disabled
          />
        </div>

        <div className="form-field">
          <label className="form-label" htmlFor="profile-name">{t("auth.name")}</label>
          <input
            id="profile-name"
            type="text"
            className="form-input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        </div>

        {nameError && (
          <p className="status-message status-message--error">{nameError}</p>
        )}
        {nameMessage && (
          <p className="status-message status-message--success">{nameMessage}</p>
        )}

        <div className="btn-row">
          <button
            type="submit"
            className="btn btn--primary"
            disabled={savingName || name.trim() === currentProfile.name}
          >
            {savingName ? t("settings.saving") : t("settings.saveName")}
          </button>
        </div>
      </form>

      <div className="card settings-section">
        <h2>{t("settings.appearance")}</h2>
        <p className="card-description">
          {t("settings.appearanceDescription")}
        </p>

        <fieldset className="btn-row" style={{ border: "none", padding: 0, margin: 0 }}>
          <legend className="sr-only">{t("settings.appearance")}</legend>
          {(["dark", "light", "system"] as const).map((option) => {
            const labels: Record<string, string> = { dark: t("settings.theme.dark"), light: t("settings.theme.light"), system: t("settings.theme.system") };
            const selected = theme === option;
            return (
              <label
                key={option}
                className={`theme-option${selected ? " theme-option--active" : ""}`}
              >
                <input
                  type="radio"
                  name="theme"
                  value={option}
                  checked={selected}
                  onChange={() => setTheme(option)}
                  className="sr-only"
                  aria-label={labels[option]}
                />
                {labels[option]}
              </label>
            );
          })}
        </fieldset>
      </div>

      <div className="card settings-section">
        <h2>{t("language.label")}</h2>
        <p className="card-description">{t("language.description")}</p>
        <LanguageSelect />
        <p className="form-hint" style={{ marginTop: 8 }}>{t("language.browser")}</p>
      </div>

      <RecordingDefaults />

      {(transcriptionEnabled || noiseReductionEnabled) && (
        <div className="card settings-section">
          <h2>{t("settings.audio")}</h2>
          <p className="card-description">
            {t("settings.audioDescription")}
          </p>
          {transcriptionEnabled && (
            <div className="form-field">
              <label className="form-label" htmlFor="transcription-language">{t("settings.transcriptionLanguage")}</label>
              <select
                id="transcription-language"
                className="form-input"
                value={transcriptionLanguage}
                onChange={(e) => handleTranscriptionLanguageChange(e.target.value)}
              >
                {TRANSCRIPTION_LANGUAGES.map((lang) => (
                  <option key={lang.code} value={lang.code}>{lang.name}</option>
                ))}
              </select>
            </div>
          )}
          {noiseReductionEnabled && (
            <div className="form-field">
              <label className="form-label" htmlFor="noise-reduction">{t("settings.noiseReduction")}</label>
              <select
                id="noise-reduction"
                className="form-input"
                value={noiseReduction ? "on" : "off"}
                onChange={(e) => handleNoiseReductionChange(e.target.value === "on")}
              >
                <option value="on">{t("settings.noiseOn")}</option>
                <option value="off">{t("settings.disabled")}</option>
              </select>
            </div>
          )}
        </div>
      )}

      <div className="card settings-section">
        <h2>{t("settings.retention")}</h2>
        <p className="card-description">
          {t("settings.retentionDescription")}
        </p>
        <div className="form-field">
          <label className="form-label" htmlFor="retention-days">{t("settings.deleteAfter")}</label>
          <select
            id="retention-days"
            className="form-input"
            value={retentionDays}
            onChange={(e) => handleRetentionDaysChange(Number(e.target.value))}
          >
            <option value={0}>{t("common.off")}</option>
            <option value={30}>{t("settings.days", { days: 30 })}</option>
            <option value={60}>{t("settings.days", { days: 60 })}</option>
            <option value={90}>{t("settings.days", { days: 90 })}</option>
            <option value={180}>{t("settings.days", { days: 180 })}</option>
            <option value={365}>{t("settings.days", { days: 365 })}</option>
          </select>
        </div>
      </div>
    </>
  );
}

type RecordingMode = "camera" | "screen" | "screen-camera";

function RecordingDefaults() {
  const [mode, setModeState] = useState<RecordingMode>(() => {
    const stored = localStorage.getItem("recording-mode");
    if (stored === "camera" || stored === "screen" || stored === "screen-camera") return stored;
    return "screen";
  });
  const [countdown, setCountdownState] = useState(() => localStorage.getItem("recording-countdown") !== "false");
  const [systemAudio, setSystemAudioState] = useState(() => localStorage.getItem("recording-audio") !== "false");

  function setMode(m: RecordingMode) {
    setModeState(m);
    localStorage.setItem("recording-mode", m);
  }
  function setCountdown(v: boolean) {
    setCountdownState(v);
    localStorage.setItem("recording-countdown", String(v));
  }
  function setSystemAudio(v: boolean) {
    setSystemAudioState(v);
    localStorage.setItem("recording-audio", String(v));
  }

  const modes: { value: RecordingMode; label: string }[] = [
    { value: "camera", label: "Kamera" },
    { value: "screen", label: "Bildschirm" },
    { value: "screen-camera", label: "Bildschirm + Kamera" },
  ];

  return (
    <div className="card settings-section">
      <h2>Aufnahme-Standards</h2>
      <p className="card-description">
        Lege deinen bevorzugten Aufnahmemodus und die Optionen fest.
      </p>

      <div className="form-field">
        <label className="form-label">Standard-Aufnahmemodus</label>
        <fieldset className="btn-row" style={{ border: "none", padding: 0, margin: 0 }}>
          <legend className="sr-only">Aufnahmemodus</legend>
          {modes.map((m) => (
            <label
              key={m.value}
              className={`theme-option${mode === m.value ? " theme-option--active" : ""}`}
            >
              <input
                type="radio"
                name="recording-mode"
                value={m.value}
                checked={mode === m.value}
                onChange={() => setMode(m.value)}
                className="sr-only"
              />
              {m.label}
            </label>
          ))}
        </fieldset>
      </div>

      <div className="form-field" style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}>
        <label className="form-label" style={{ margin: 0 }}>Countdown</label>
        <button
          type="button"
          className={`toggle-track${countdown ? " active" : ""}`}
          onClick={() => setCountdown(!countdown)}
          role="switch"
          aria-checked={countdown}
        >
          <span className="toggle-thumb" />
        </button>
      </div>

      <div className="form-field" style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}>
        <label className="form-label" style={{ margin: 0 }}>Systemaudio aufnehmen</label>
        <button
          type="button"
          className={`toggle-track${systemAudio ? " active" : ""}`}
          onClick={() => setSystemAudio(!systemAudio)}
          role="switch"
          aria-checked={systemAudio}
        >
          <span className="toggle-thumb" />
        </button>
      </div>
    </div>
  );
}
