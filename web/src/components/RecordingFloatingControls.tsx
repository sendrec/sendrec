import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { formatDuration } from "../utils/format";

interface RecordingFloatingControlsProps {
  webcamStream: MediaStream | null;
  webcamEnabled: boolean;
  duration: number;
  isPaused: boolean;
  remaining: number | null;
  onPause: () => void;
  onResume: () => void;
  onStop: () => void;
  onUnavailable?: () => void;
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
  webcamStream,
  webcamEnabled,
  duration,
  isPaused,
  remaining,
  onPause,
  onResume,
  onStop,
  onUnavailable,
}: RecordingFloatingControlsProps) {
  const [portalRoot, setPortalRoot] = useState<HTMLElement | null>(null);
  const onStopRef = useRef(onStop);

  useEffect(() => {
    onStopRef.current = onStop;
  }, [onStop]);

  useEffect(() => {
    let cancelled = false;
    let activePipWindow: Window | null = null;
    let handlePageHide: (() => void) | null = null;

    async function openPictureInPictureWindow() {
      activePipWindow = await requestPictureInPictureWindow();
      if (!activePipWindow) {
        onUnavailable?.();
        return;
      }

      if (cancelled) {
        activePipWindow.close();
        return;
      }

      cloneStylesheetsInto(activePipWindow.document);

      const root = activePipWindow.document.createElement("div");
      root.id = "root";
      activePipWindow.document.body.appendChild(root);

      handlePageHide = () => onStopRef.current();
      activePipWindow.addEventListener("pagehide", handlePageHide);
      setPortalRoot(root);
    }

    void openPictureInPictureWindow();

    return () => {
      cancelled = true;
      if (activePipWindow && handlePageHide) {
        activePipWindow.removeEventListener("pagehide", handlePageHide);
      }
      activePipWindow?.close();
    };
  }, [onUnavailable]);

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

async function requestPictureInPictureWindow(): Promise<Window | null> {
  const documentPictureInPicture = window.documentPictureInPicture;
  if (!documentPictureInPicture) return null;

  try {
    return await documentPictureInPicture.requestWindow({
      width: 280,
      height: 220,
    });
  } catch {
    return null;
  }
}

function cloneStylesheetsInto(pipDocument: Document) {
  document
    .querySelectorAll<HTMLLinkElement>('link[rel="stylesheet"]')
    .forEach((link) => {
      pipDocument.head.appendChild(link.cloneNode(true));
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
