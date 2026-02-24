import { useCallback, useEffect, useRef, useState } from "react";

type RecordingState = "idle" | "countdown" | "recording" | "paused" | "stopped";

interface CameraRecorderProps {
  onRecordingComplete: (blob: Blob, duration: number) => void;
  maxDurationSeconds?: number;
}

function formatDuration(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  const remaining = seconds % 60;
  return `${minutes}:${String(remaining).padStart(2, "0")}`;
}

function getSupportedMimeType(): string {
  if (typeof MediaRecorder === "undefined") return "video/mp4";
  // Prefer MP4 for universal playback compatibility (Safari can record WebM but not play it back)
  if (MediaRecorder.isTypeSupported("video/mp4")) return "video/mp4";
  if (MediaRecorder.isTypeSupported("video/webm;codecs=vp9,opus")) return "video/webm;codecs=vp9,opus";
  return "video/mp4";
}

export function CameraRecorder({ onRecordingComplete, maxDurationSeconds = 0 }: CameraRecorderProps) {
  const [recordingState, setRecordingState] = useState<RecordingState>("idle");
  const [duration, setDuration] = useState(0);
  const [countdownValue, setCountdownValue] = useState(3);
  const [facingMode, setFacingMode] = useState<"user" | "environment">("user");
  const [cameraError, setCameraError] = useState<string | null>(null);

  const videoRef = useRef<HTMLVideoElement>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const startTimeRef = useRef(0);
  const timerRef = useRef<ReturnType<typeof setInterval>>(0 as unknown as ReturnType<typeof setInterval>);
  const countdownTimerRef = useRef<ReturnType<typeof setInterval>>(0 as unknown as ReturnType<typeof setInterval>);
  const pauseStartRef = useRef(0);
  const totalPausedRef = useRef(0);
  const mimeTypeRef = useRef("");

  const stopStream = useCallback(() => {
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((track) => track.stop());
      streamRef.current = null;
    }
  }, []);

  const elapsedSeconds = useCallback(() => {
    return Math.floor((Date.now() - startTimeRef.current - totalPausedRef.current) / 1000);
  }, []);

  const startTimer = useCallback(() => {
    timerRef.current = setInterval(() => {
      setDuration(elapsedSeconds());
    }, 1000);
  }, [elapsedSeconds]);

  const stopRecording = useCallback(() => {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state !== "inactive") {
      if (mediaRecorderRef.current.state === "paused") {
        totalPausedRef.current += Date.now() - pauseStartRef.current;
      }
      mediaRecorderRef.current.stop();
    }
    clearInterval(timerRef.current);
    setRecordingState("stopped");
  }, []);

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
      } catch (err) {
        const detail = err instanceof DOMException ? `${err.name}: ${err.message}` : String(err);
        setCameraError(`Could not access your camera: ${detail}`);
      }
    }
    startPreview();
  }, [facingMode, stopStream]);

  const beginRecording = useCallback(() => {
    clearInterval(countdownTimerRef.current);
    if (mediaRecorderRef.current) {
      mediaRecorderRef.current.start(1000);
    }
    startTimeRef.current = Date.now();
    setDuration(0);
    startTimer();
    setRecordingState("recording");
  }, [startTimer]);

  useEffect(() => {
    if (recordingState !== "countdown") return;
    countdownTimerRef.current = setInterval(() => {
      setCountdownValue((prev) => {
        if (prev <= 1) {
          beginRecording();
          return 3;
        }
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(countdownTimerRef.current);
  }, [recordingState, beginRecording]);

  useEffect(() => {
    if (
      recordingState === "recording" &&
      maxDurationSeconds > 0 &&
      duration >= maxDurationSeconds
    ) {
      stopRecording();
    }
  }, [duration, maxDurationSeconds, recordingState, stopRecording]);

  useEffect(() => {
    return () => {
      clearInterval(timerRef.current);
      clearInterval(countdownTimerRef.current);
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
    pauseStartRef.current = 0;
    totalPausedRef.current = 0;

    recorder.ondataavailable = (event) => {
      if (event.data.size > 0) {
        chunksRef.current.push(event.data);
      }
    };

    recorder.onstop = () => {
      const blobType = mimeType.startsWith("video/webm") ? "video/webm" : "video/mp4";
      const blob = new Blob(chunksRef.current, { type: blobType });
      const elapsed = elapsedSeconds();
      onRecordingComplete(blob, elapsed);
    };

    setCountdownValue(3);
    setRecordingState("countdown");
  }

  function pauseRecording() {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state === "recording") {
      mediaRecorderRef.current.pause();
      pauseStartRef.current = Date.now();
      clearInterval(timerRef.current);
      setRecordingState("paused");
    }
  }

  function resumeRecording() {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state === "paused") {
      totalPausedRef.current += Date.now() - pauseStartRef.current;
      mediaRecorderRef.current.resume();
      startTimer();
      setRecordingState("recording");
    }
  }

  const isIdle = recordingState === "idle";
  const isCountdown = recordingState === "countdown";
  const isPaused = recordingState === "paused";
  const isActive = !isIdle && recordingState !== "stopped";
  const isRecording = recordingState === "recording" || isPaused;
  const remaining = maxDurationSeconds > 0 ? maxDurationSeconds - duration : null;

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
