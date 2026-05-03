import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { formatDuration } from "../utils/format";

interface RecordingFloatingControlsProps {
  pipWindow: Window;
  webcamStream: MediaStream | null;
  webcamEnabled: boolean;
  duration: number;
  isPaused: boolean;
  remaining: number | null;
  onPause: () => void;
  onResume: () => void;
  onStop: () => void;
}

interface FloatingPanelProps {
  webcamStream: MediaStream | null;
  webcamEnabled: boolean;
  duration: number;
  isPaused: boolean;
  remaining: number | null;
  onPause: () => void;
  onResume: () => void;
  onStop: () => void;
}

export function RecordingFloatingControls({
  pipWindow,
  webcamStream,
  webcamEnabled,
  duration,
  isPaused,
  remaining,
  onPause,
  onResume,
  onStop,
}: RecordingFloatingControlsProps) {
  const [portalRoot, setPortalRoot] = useState<HTMLElement | null>(null);
  const onStopRef = useRef(onStop);

  useEffect(() => {
    onStopRef.current = onStop;
  }, [onStop]);

  useEffect(() => {
    cloneStylesheetsInto(pipWindow.document);

    const root = pipWindow.document.createElement("div");
    root.id = "root";
    pipWindow.document.body.appendChild(root);

    const handlePageHide = () => onStopRef.current();
    pipWindow.addEventListener("pagehide", handlePageHide);
    setPortalRoot(root);

    return () => {
      pipWindow.removeEventListener("pagehide", handlePageHide);
    };
  }, [pipWindow]);

  if (!portalRoot) return null;

  return createPortal(
    <FloatingPanel
      webcamStream={webcamStream}
      webcamEnabled={webcamEnabled}
      duration={duration}
      isPaused={isPaused}
      remaining={remaining}
      onPause={onPause}
      onResume={onResume}
      onStop={onStop}
    />,
    portalRoot,
  );
}

function cloneStylesheetsInto(pipDocument: Document) {
  document
    .querySelectorAll<HTMLLinkElement | HTMLStyleElement>(
      'link[rel="stylesheet"], style',
    )
    .forEach((styleNode) => {
      pipDocument.head.appendChild(styleNode.cloneNode(true));
    });
}

function FloatingPanel({
  webcamStream,
  webcamEnabled,
  duration,
  isPaused,
  remaining,
  onPause,
  onResume,
  onStop,
}: FloatingPanelProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null);

  useEffect(() => {
    if (!videoRef.current) return;
    videoRef.current.srcObject = webcamEnabled ? webcamStream : null;
  }, [webcamEnabled, webcamStream]);

  return (
    <div className="recording-floating-panel" role="status" aria-live="polite">
      {webcamEnabled && (
        <video
          ref={videoRef}
          autoPlay
          muted
          playsInline
          className="recording-floating-panel__video"
        />
      )}

      <div className="recording-floating-panel__timer">
        <span
          className={`recording-dot ${
            isPaused ? "recording-dot--paused" : "recording-dot--active"
          }`}
        />
        <span>{formatDuration(duration)}</span>
        {isPaused && <span className="recording-remaining">(Paused)</span>}
        {!isPaused && remaining !== null && (
          <span className="recording-remaining">
            ({formatDuration(remaining)} remaining)
          </span>
        )}
      </div>

      <div className="recording-floating-panel__actions">
        {isPaused ? (
          <button
            type="button"
            onClick={onResume}
            aria-label="Resume recording"
            className="btn-resume"
          >
            Resume
          </button>
        ) : (
          <button
            type="button"
            onClick={onPause}
            aria-label="Pause recording"
            className="btn-pause"
          >
            Pause
          </button>
        )}
        <button
          type="button"
          onClick={onStop}
          aria-label="Stop recording"
          className="btn-stop"
        >
          Stop
        </button>
      </div>
    </div>
  );
}
