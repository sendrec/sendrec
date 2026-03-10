import { useCallback, useEffect, useRef, useState } from "react";
import { useRecording, MIN_RECORDING_SECONDS, MIN_RECORDING_BYTES } from "../hooks/useRecording";
import { getSupportedMimeType, blobTypeFromMimeType } from "../utils/mediaFormat";
import { formatDuration } from "../utils/format";

interface CameraRecorderProps {
  onRecordingComplete: (blob: Blob, duration: number) => void;
  onRecordingError?: (message: string) => void;
  maxDurationSeconds?: number;
}

export function CameraRecorder({ onRecordingComplete, onRecordingError, maxDurationSeconds = 0 }: CameraRecorderProps) {
  const countdownEnabled = useRef(localStorage.getItem("recording-countdown") !== "false");
  const [facingMode, setFacingMode] = useState<"user" | "environment">("user");
  const [cameraError, setCameraError] = useState<string | null>(null);

  const videoRef = useRef<HTMLVideoElement>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const mimeTypeRef = useRef("");

  const stopStream = useCallback(() => {
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((track) => track.stop());
      streamRef.current = null;
    }
  }, []);

  // Stable refs for callbacks passed to useRecording to avoid stale closure issues
  const stopRecordingRef = useRef<() => void>(() => {});
  const beginRecordingRef = useRef<() => void>(() => {});

  const recording = useRecording(
    maxDurationSeconds,
    () => beginRecordingRef.current(),
    () => stopRecordingRef.current(),
  );

  const stopRecording = useCallback(() => {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state !== "inactive") {
      if (mediaRecorderRef.current.state === "paused") {
        recording.totalPausedRef.current += Date.now() - recording.pauseStartRef.current;
      }
      mediaRecorderRef.current.stop();
    }
    recording.stopTimer();
    recording.setState("stopped");
  }, [recording]);

  const beginRecording = useCallback(() => {
    clearInterval(recording.countdownTimerRef.current);
    if (mediaRecorderRef.current) {
      mediaRecorderRef.current.start(1000);
    }
    recording.startTimeRef.current = Date.now();
    recording.setState("recording");
    recording.startTimer();
  }, [recording]);

  // Keep stable callback refs up to date
  stopRecordingRef.current = stopRecording;
  beginRecordingRef.current = beginRecording;

  useEffect(() => {
    async function startPreview() {
      setCameraError(null);
      stopStream();
      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode, width: { ideal: 1280 }, height: { ideal: 720 } },
          audio: true,
        });
        streamRef.current = stream;
        if (videoRef.current) {
          videoRef.current.srcObject = stream;
          videoRef.current.play().catch(() => {});
        }
      } catch {
        setCameraError("Could not access your camera. Please allow camera access and try again.");
      }
    }
    startPreview();
  }, [facingMode, stopStream]);

  const stopTimerRef = useRef(recording.stopTimer);
  stopTimerRef.current = recording.stopTimer;

  useEffect(() => {
    return () => {
      stopTimerRef.current();
      stopStream();
    };
  }, [stopStream]);

  function flipCamera() {
    setFacingMode((prev) => (prev === "user" ? "environment" : "user"));
  }

  function startRecording() {
    if (!streamRef.current) return;

    const mimeType = getSupportedMimeType();
    mimeTypeRef.current = mimeType;

    const recorder = new MediaRecorder(streamRef.current, { mimeType });
    mediaRecorderRef.current = recorder;
    chunksRef.current = [];
    recording.pauseStartRef.current = 0;
    recording.totalPausedRef.current = 0;

    recorder.ondataavailable = (event) => {
      if (event.data.size > 0) {
        chunksRef.current.push(event.data);
      }
    };

    recorder.onstop = () => {
      const blob = new Blob(chunksRef.current, { type: blobTypeFromMimeType(mimeType) });
      const elapsed = recording.elapsedSeconds();

      if (elapsed < MIN_RECORDING_SECONDS || blob.size < MIN_RECORDING_BYTES) {
        onRecordingError?.("Recording too short. Please record for at least 1 second.");
        return;
      }

      onRecordingComplete(blob, elapsed);
    };

    if (countdownEnabled.current) {
      recording.setCountdown(3);
      recording.setState("countdown");
    } else {
      beginRecording();
    }
  }

  function pauseRecording() {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state === "recording") {
      mediaRecorderRef.current.pause();
      recording.pauseStartRef.current = Date.now();
      recording.stopTimer();
      recording.setState("paused");
    }
  }

  function resumeRecording() {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state === "paused") {
      recording.totalPausedRef.current += Date.now() - recording.pauseStartRef.current;
      mediaRecorderRef.current.resume();
      recording.startTimer();
      recording.setState("recording");
    }
  }

  const { elapsed: duration, countdown: countdownValue,
    isIdle, isCountdown, isPaused, isActive, isRecording, remaining } = recording;

  if (cameraError) {
    return (
      <div style={{ textAlign: "center" }}>
        <p style={{ color: "var(--color-error)", fontSize: 14, marginBottom: 16 }}>
          {cameraError}
        </p>
      </div>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16, alignItems: "center", width: "100%" }}>
      <div style={{ position: "relative", width: "100%", maxWidth: 480 }}>
        <video
          ref={videoRef}
          autoPlay
          muted
          playsInline
          data-testid="camera-preview"
          style={{
            width: "100%",
            borderRadius: 12,
            background: "#000",
            display: "block",
            transform: facingMode === "user" ? "scaleX(-1)" : "none",
          }}
        />
        <button
          onClick={flipCamera}
          disabled={isActive}
          aria-label="Flip camera"
          style={{
            position: "absolute",
            top: 8,
            right: 8,
            width: 40,
            height: 40,
            borderRadius: 20,
            border: "none",
            background: "rgba(0, 0, 0, 0.5)",
            color: "#fff",
            cursor: isActive ? "default" : "pointer",
            opacity: isActive ? 0.4 : 1,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: 18,
            padding: 0,
          }}
        >
          &#x21C4;
        </button>
        {isCountdown && (
          <div
            className="countdown-overlay"
            data-testid="countdown-overlay"
            onClick={beginRecording}
          >
            <div className="countdown-number">{countdownValue}</div>
            <div className="countdown-hint">Click to start now</div>
          </div>
        )}
      </div>

      {isIdle && (
        <>
          {maxDurationSeconds > 0 && (
            <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: 0 }}>
              Maximum recording length: {formatDuration(maxDurationSeconds)}
            </p>
          )}
          <button
            onClick={startRecording}
            aria-label="Start recording"
            style={{
              background: "var(--color-accent)",
              color: "var(--color-text)",
              borderRadius: 8,
              padding: "14px 32px",
              fontSize: 16,
              fontWeight: 600,
              minHeight: 44,
            }}
          >
            Start Recording
          </button>
        </>
      )}

      {isRecording && (
        <div style={{ display: "flex", alignItems: "center", gap: 12, flexWrap: "wrap", justifyContent: "center" }}>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 8,
              color: isPaused ? "var(--color-text-secondary)" : "var(--color-error)",
              fontWeight: 600,
              fontSize: 14,
            }}
          >
            <div
              style={{
                width: 10,
                height: 10,
                borderRadius: "50%",
                background: isPaused ? "var(--color-text-secondary)" : "var(--color-error)",
                animation: isPaused ? "none" : "pulse 1.5s infinite",
              }}
            />
            {formatDuration(duration)}
            {isPaused && (
              <span style={{ fontWeight: 400 }}>
                (Paused)
              </span>
            )}
            {!isPaused && remaining !== null && (
              <span style={{ color: "var(--color-text-secondary)", fontWeight: 400 }}>
                ({formatDuration(remaining)} remaining)
              </span>
            )}
          </div>

          {isPaused ? (
            <button
              onClick={resumeRecording}
              aria-label="Resume recording"
              style={{
                background: "var(--color-accent)",
                color: "var(--color-text)",
                borderRadius: 8,
                padding: "10px 24px",
                fontSize: 14,
                fontWeight: 600,
                minHeight: 44,
              }}
            >
              Resume
            </button>
          ) : (
            <button
              onClick={pauseRecording}
              aria-label="Pause recording"
              style={{
                background: "transparent",
                color: "var(--color-text)",
                border: "1px solid var(--color-border)",
                borderRadius: 8,
                padding: "10px 24px",
                fontSize: 14,
                fontWeight: 600,
                minHeight: 44,
              }}
            >
              Pause
            </button>
          )}

          <button
            onClick={stopRecording}
            aria-label="Stop recording"
            style={{
              background: "var(--color-error)",
              color: "var(--color-text)",
              borderRadius: 8,
              padding: "10px 24px",
              fontSize: 14,
              fontWeight: 600,
              minHeight: 44,
            }}
          >
            Stop Recording
          </button>
        </div>
      )}
    </div>
  );
}
