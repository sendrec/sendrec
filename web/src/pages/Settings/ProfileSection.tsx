import { type FormEvent, useState } from "react";
import { apiFetch } from "../../api/client";
import { useTheme } from "../../hooks/useTheme";
import { useUnsavedChanges } from "../../hooks/useUnsavedChanges";
import { TRANSCRIPTION_LANGUAGES } from "../../constants/languages";
import { UserProfile } from "./types";

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
      setNameError("Name is required");
      return;
    }

    setSavingName(true);
    try {
      await apiFetch("/api/user", {
        method: "PATCH",
        body: JSON.stringify({ name: name.trim() }),
      });
      setNameMessage("Name updated");
      setCurrentProfile((prev) => ({ ...prev, name: name.trim() }));
    } catch (err) {
      setNameError(err instanceof Error ? err.message : "Failed to update name");
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
        <h2>Profile</h2>

        <div className="form-field">
          <label className="form-label" htmlFor="profile-email">Email</label>
          <input
            id="profile-email"
            type="email"
            className="form-input"
            value={profile.email}
            disabled
          />
        </div>

        <div className="form-field">
          <label className="form-label" htmlFor="profile-name">Name</label>
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
            {savingName ? "Saving..." : "Save name"}
          </button>
        </div>
      </form>

      <div className="card settings-section">
        <h2>Appearance</h2>
        <p className="card-description">
          Choose how SendRec looks to you.
        </p>

        <fieldset className="btn-row" style={{ border: "none", padding: 0, margin: 0 }}>
          <legend className="sr-only">Theme preference</legend>
          {(["dark", "light", "system"] as const).map((option) => {
            const labels: Record<string, string> = { dark: "Dark", light: "Light", system: "System" };
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

      <RecordingDefaults />

      {(transcriptionEnabled || noiseReductionEnabled) && (
        <div className="card settings-section">
          <h2>Audio</h2>
          <p className="card-description">
            Configure transcription and audio processing for new recordings.
          </p>
          {transcriptionEnabled && (
            <div className="form-field">
              <label className="form-label" htmlFor="transcription-language">Default transcription language</label>
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
              <label className="form-label" htmlFor="noise-reduction">Noise reduction</label>
              <select
                id="noise-reduction"
                className="form-input"
                value={noiseReduction ? "on" : "off"}
                onChange={(e) => handleNoiseReductionChange(e.target.value === "on")}
              >
                <option value="on">Enabled — reduce background noise in new recordings</option>
                <option value="off">Disabled</option>
              </select>
            </div>
          )}
        </div>
      )}

      <div className="card settings-section">
        <h2>Data Retention</h2>
        <p className="card-description">
          Automatically delete videos after a set number of days. Pinned videos are excluded.
        </p>
        <div className="form-field">
          <label className="form-label" htmlFor="retention-days">Auto-delete after</label>
          <select
            id="retention-days"
            className="form-input"
            value={retentionDays}
            onChange={(e) => handleRetentionDaysChange(Number(e.target.value))}
          >
            <option value={0}>Off</option>
            <option value={30}>30 days</option>
            <option value={60}>60 days</option>
            <option value={90}>90 days</option>
            <option value={180}>180 days</option>
            <option value={365}>365 days</option>
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
    { value: "camera", label: "Camera" },
    { value: "screen", label: "Screen" },
    { value: "screen-camera", label: "Screen + Camera" },
  ];

  return (
    <div className="card settings-section">
      <h2>Recording Defaults</h2>
      <p className="card-description">
        Set your preferred recording mode and options.
      </p>

      <div className="form-field">
        <label className="form-label">Default recording mode</label>
        <fieldset className="btn-row" style={{ border: "none", padding: 0, margin: 0 }}>
          <legend className="sr-only">Recording mode</legend>
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
        <label className="form-label" style={{ margin: 0 }}>Countdown timer</label>
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
        <label className="form-label" style={{ margin: 0 }}>System audio capture</label>
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
