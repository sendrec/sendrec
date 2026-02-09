import { useCallback, useEffect, useRef, useState } from "react";

type RecordingState = "idle" | "recording" | "paused" | "stopped";

interface RecorderProps {
  onRecordingComplete: (blob: Blob, duration: number) => void;
  maxDurationSeconds?: number;
}

function formatDuration(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  const remaining = seconds % 60;
  return `${minutes}:${String(remaining).padStart(2, "0")}`;
}

export function Recorder({ onRecordingComplete, maxDurationSeconds = 0 }: RecorderProps) {
  const [recordingState, setRecordingState] = useState<RecordingState>("idle");
  const [duration, setDuration] = useState(0);

  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const startTimeRef = useRef<number>(0);
  const timerRef = useRef<ReturnType<typeof setInterval>>(0 as unknown as ReturnType<typeof setInterval>);
  const screenStreamRef = useRef<MediaStream | null>(null);
  const pauseStartRef = useRef<number>(0);
  const totalPausedRef = useRef<number>(0);

  const stopAllStreams = useCallback(() => {
    if (screenStreamRef.current) {
      screenStreamRef.current.getTracks().forEach((track) => track.stop());
      screenStreamRef.current = null;
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
    stopAllStreams();
    setRecordingState("stopped");
  }, [stopAllStreams]);

  const pauseRecording = useCallback(() => {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state === "recording") {
      mediaRecorderRef.current.pause();
      pauseStartRef.current = Date.now();
      clearInterval(timerRef.current);
      setRecordingState("paused");
    }
  }, []);

  const resumeRecording = useCallback(() => {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state === "paused") {
      totalPausedRef.current += Date.now() - pauseStartRef.current;
      mediaRecorderRef.current.resume();
      startTimer();
      setRecordingState("recording");
    }
  }, [startTimer]);

  async function startRecording() {
    try {
      const screenStream = await navigator.mediaDevices.getDisplayMedia({
        video: { width: 1920, height: 1080 },
        audio: true,
      });
      screenStreamRef.current = screenStream;

      // Record the screen stream directly — no canvas intermediate
      const recorder = new MediaRecorder(screenStream, {
        mimeType: "video/webm;codecs=vp9,opus",
      });
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
        const blob = new Blob(chunksRef.current, { type: "video/webm" });
        const elapsed = elapsedSeconds();
        onRecordingComplete(blob, elapsed);
      };

      screenStream.getVideoTracks()[0].addEventListener("ended", () => {
        stopRecording();
      });

      startTimeRef.current = Date.now();
      setDuration(0);
      startTimer();

      recorder.start(1000);
      setRecordingState("recording");
    } catch (err) {
      console.error("Screen capture failed", err);
      alert("Screen recording was blocked or failed. Please allow screen capture and try again.");
      stopAllStreams();
    }
  }

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
      stopAllStreams();
    };
  }, [stopAllStreams]);

  if (recordingState === "idle") {
    return (
      <div style={{ display: "flex", flexDirection: "column", gap: 16, alignItems: "center" }}>
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
          }}
        >
          Start Recording
        </button>
      </div>
    );
  }

  const isPaused = recordingState === "paused";
  const remaining = maxDurationSeconds > 0 ? maxDurationSeconds - duration : null;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16, alignItems: "center" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
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
          }}
        >
          Stop Recording
        </button>
      </div>

    </div>
  );
}
