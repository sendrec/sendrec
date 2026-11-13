export type RecordingMode = "camera" | "screen" | "screen-camera";

export const RECORDING_MODES: { value: RecordingMode; label: string }[] = [
  { value: "camera", label: "Camera" },
  { value: "screen", label: "Screen" },
  { value: "screen-camera", label: "Screen + Camera" },
];

const STORAGE_KEY = "recording-mode";

export function readRecordingMode(): RecordingMode {
  const stored = localStorage.getItem(STORAGE_KEY);
  return RECORDING_MODES.some((m) => m.value === stored) ? (stored as RecordingMode) : "screen";
}

export function saveRecordingMode(mode: RecordingMode) {
  localStorage.setItem(STORAGE_KEY, mode);
}

interface RecordingModePickerProps {
  value: RecordingMode;
  onChange: (mode: RecordingMode) => void;
  modes?: RecordingMode[];
}

export function RecordingModePicker({ value, onChange, modes }: RecordingModePickerProps) {
  const options = modes ? RECORDING_MODES.filter((m) => modes.includes(m.value)) : RECORDING_MODES;
  return (
    <fieldset className="btn-row" style={{ border: "none", padding: 0, margin: 0 }}>
      <legend className="sr-only">Recording mode</legend>
      {options.map((m) => (
        <label
          key={m.value}
          className={`theme-option${value === m.value ? " theme-option--active" : ""}`}
        >
          <input
            type="radio"
            name="recording-mode"
            value={m.value}
            checked={value === m.value}
            onChange={() => onChange(m.value)}
            className="sr-only"
          />
          {m.label}
        </label>
      ))}
    </fieldset>
  );
}
