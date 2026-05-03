import { formatDuration } from "../utils/format";

interface RecordingHeaderProps {
  duration: number;
  isPaused: boolean;
  remaining: number | null;
  drawMode: boolean;
  drawColor: string;
  lineWidth: number;
  onPause: () => void;
  onResume: () => void;
  onStop: () => void;
  onToggleDraw: () => void;
  onClearCanvas: () => void;
  onSetDrawColor: (color: string) => void;
  onSetLineWidth: (width: number) => void;
}

export function RecordingHeader({
  duration,
  isPaused,
  remaining,
  drawMode,
  drawColor,
  lineWidth,
  onPause,
  onResume,
  onStop,
  onToggleDraw,
  onClearCanvas,
  onSetDrawColor,
  onSetLineWidth,
}: RecordingHeaderProps) {
  return (
    <div className="recording-header" role="status" aria-live="polite">
      <div
        className={`recording-indicator ${
          isPaused ? "recording-indicator--paused" : "recording-indicator--active"
        }`}
      >
        <div
          className={`recording-dot ${
            isPaused ? "recording-dot--paused" : "recording-dot--active"
          }`}
        />
        {formatDuration(duration)}
        {isPaused && <span className="recording-remaining">(Paused)</span>}
        {!isPaused && remaining !== null && (
          <span className="recording-remaining">
            ({formatDuration(remaining)} remaining)
          </span>
        )}
      </div>

      <button
        onClick={onToggleDraw}
        aria-label={drawMode ? "Disable drawing" : "Enable drawing"}
        data-testid="draw-toggle"
        className={`btn-draw${drawMode ? " btn-draw--active" : ""}`}
      >
        Draw
      </button>

      {drawMode && (
        <input
          type="color"
          value={drawColor}
          onChange={(e) => onSetDrawColor(e.target.value)}
          aria-label="Drawing color"
          data-testid="color-picker"
          style={{
            width: 36,
            height: 36,
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 2,
            background: "transparent",
            cursor: "pointer",
          }}
        />
      )}

      {drawMode && (
        <button
          onClick={onClearCanvas}
          aria-label="Clear drawing"
          data-testid="clear-drawing"
          className="btn-pause"
        >
          Clear
        </button>
      )}

      {drawMode && (
        <div
          style={{ display: "flex", gap: 4, alignItems: "center" }}
          data-testid="thickness-selector"
        >
          {[2, 4, 8].map((w) => (
            <button
              key={w}
              onClick={() => onSetLineWidth(w)}
              aria-label={`Line width ${w}`}
              style={{
                width: 28,
                height: 28,
                borderRadius: "50%",
                border:
                  lineWidth === w
                    ? "2px solid var(--color-accent)"
                    : "1px solid var(--color-border)",
                background: "transparent",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                cursor: "pointer",
                padding: 0,
              }}
            >
              <div
                style={{
                  width: w + 2,
                  height: w + 2,
                  borderRadius: "50%",
                  background: "var(--color-text)",
                }}
              />
            </button>
          ))}
        </div>
      )}

      {isPaused ? (
        <button
          onClick={onResume}
          aria-label="Resume recording"
          className="btn-resume"
        >
          Resume
        </button>
      ) : (
        <button
          onClick={onPause}
          aria-label="Pause recording"
          className="btn-pause"
        >
          Pause
        </button>
      )}

      <button
        onClick={onStop}
        aria-label="Stop recording"
        className="btn-stop"
      >
        Stop Recording
      </button>
    </div>
  );
}
