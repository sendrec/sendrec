import { useCallback, useEffect, useRef, useState } from "react";

type RecordingState = "idle" | "recording" | "stopped";

interface RecorderProps {
  onRecordingComplete: (blob: Blob, duration: number) => void;
}

function formatDuration(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  const remaining = seconds % 60;
  return `${minutes}:${String(remaining).padStart(2, "0")}`;
}

export function Recorder({ onRecordingComplete }: RecorderProps) {
  const [recordingState, setRecordingState] = useState<RecordingState>("idle");
  const [duration, setDuration] = useState(0);

  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const startTimeRef = useRef<number>(0);
  const timerRef = useRef<ReturnType<typeof setInterval>>(0 as unknown as ReturnType<typeof setInterval>);
  const screenStreamRef = useRef<MediaStream | null>(null);

  const stopAllStreams = useCallback(() => {
    if (screenStreamRef.current) {
      screenStreamRef.current.getTracks().forEach((track) => track.stop());
      screenStreamRef.current = null;
    }
  }, []);

  const stopRecording = useCallback(() => {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state !== "inactive") {
      mediaRecorderRef.current.stop();
    }
    clearInterval(timerRef.current);
    stopAllStreams();
    setRecordingState("stopped");
  }, [stopAllStreams]);

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

      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          chunksRef.current.push(event.data);
        }
      };

      recorder.onstop = () => {
        const blob = new Blob(chunksRef.current, { type: "video/webm" });
        const elapsed = Math.floor((Date.now() - startTimeRef.current) / 1000);
        onRecordingComplete(blob, elapsed);
      };

      screenStream.getVideoTracks()[0].addEventListener("ended", () => {
        stopRecording();
      });

      startTimeRef.current = Date.now();
      setDuration(0);
      timerRef.current = setInterval(() => {
        setDuration(Math.floor((Date.now() - startTimeRef.current) / 1000));
      }, 1000);

      recorder.start(1000);
      setRecordingState("recording");
    } catch (err) {
      console.error("Screen capture failed", err);
      alert("Screen recording was blocked or failed. Please allow screen capture and try again.");
      stopAllStreams();
    }
  }

  useEffect(() => {
    return () => {
      clearInterval(timerRef.current);
      stopAllStreams();
    };
  }, [stopAllStreams]);

  if (recordingState === "idle") {
    return (
      <div style={{ display: "flex", flexDirection: "column", gap: 16, alignItems: "center" }}>
        <button
          onClick={startRecording}
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

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16, alignItems: "center" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 8,
            color: "var(--color-error)",
            fontWeight: 600,
            fontSize: 14,
          }}
        >
          <div
            style={{
              width: 10,
              height: 10,
              borderRadius: "50%",
              background: "var(--color-error)",
              animation: "pulse 1.5s infinite",
            }}
          />
          {formatDuration(duration)}
        </div>

        <button
          onClick={stopRecording}
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
