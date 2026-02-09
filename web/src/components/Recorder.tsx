import { useCallback, useEffect, useRef, useState } from "react";

type RecordingState = "idle" | "recording" | "paused" | "stopped";

interface RecorderProps {
  onRecordingComplete: (blob: Blob, duration: number, webcamBlob?: Blob) => void;
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
  const [webcamEnabled, setWebcamEnabled] = useState(false);

  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const startTimeRef = useRef<number>(0);
  const timerRef = useRef<ReturnType<typeof setInterval>>(0 as unknown as ReturnType<typeof setInterval>);
  const screenStreamRef = useRef<MediaStream | null>(null);
  const pauseStartRef = useRef<number>(0);
  const totalPausedRef = useRef<number>(0);

  const webcamStreamRef = useRef<MediaStream | null>(null);
  const webcamRecorderRef = useRef<MediaRecorder | null>(null);
  const webcamChunksRef = useRef<Blob[]>([]);
  const webcamVideoRef = useRef<HTMLVideoElement | null>(null);

  const stopWebcamStream = useCallback(() => {
    if (webcamStreamRef.current) {
      webcamStreamRef.current.getTracks().forEach((track) => track.stop());
      webcamStreamRef.current = null;
    }
  }, []);

  const stopAllStreams = useCallback(() => {
    if (screenStreamRef.current) {
      screenStreamRef.current.getTracks().forEach((track) => track.stop());
      screenStreamRef.current = null;
    }
    stopWebcamStream();
  }, [stopWebcamStream]);

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
    if (webcamRecorderRef.current && webcamRecorderRef.current.state !== "inactive") {
      if (webcamRecorderRef.current.state === "paused") {
        webcamRecorderRef.current.resume();
      }
      webcamRecorderRef.current.stop();
    }
    clearInterval(timerRef.current);
    stopAllStreams();
    setRecordingState("stopped");
  }, [stopAllStreams]);

  const pauseRecording = useCallback(() => {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state === "recording") {
      mediaRecorderRef.current.pause();
      if (webcamRecorderRef.current && webcamRecorderRef.current.state === "recording") {
        webcamRecorderRef.current.pause();
      }
      pauseStartRef.current = Date.now();
      clearInterval(timerRef.current);
      setRecordingState("paused");
    }
  }, []);

  const resumeRecording = useCallback(() => {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state === "paused") {
      totalPausedRef.current += Date.now() - pauseStartRef.current;
      mediaRecorderRef.current.resume();
      if (webcamRecorderRef.current && webcamRecorderRef.current.state === "paused") {
        webcamRecorderRef.current.resume();
      }
      startTimer();
      setRecordingState("recording");
    }
  }, [startTimer]);

  async function toggleWebcam() {
    if (webcamEnabled) {
      stopWebcamStream();
      setWebcamEnabled(false);
      return;
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        video: { width: 320, height: 240, facingMode: "user" },
        audio: false,
      });
      webcamStreamRef.current = stream;
      if (webcamVideoRef.current) {
        webcamVideoRef.current.srcObject = stream;
      }
      setWebcamEnabled(true);
    } catch (err) {
      console.error("Webcam access failed", err);
      alert("Could not access your camera. Please allow camera access and try again.");
    }
  }

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

      // Set up webcam recorder if webcam is enabled
      let webcamBlobPromise: Promise<Blob> | null = null;
      if (webcamEnabled && webcamStreamRef.current) {
        const webcamRecorder = new MediaRecorder(webcamStreamRef.current, {
          mimeType: "video/webm;codecs=vp9",
        });
        webcamRecorderRef.current = webcamRecorder;
        webcamChunksRef.current = [];

        webcamRecorder.ondataavailable = (event) => {
          if (event.data.size > 0) {
            webcamChunksRef.current.push(event.data);
          }
        };

        webcamBlobPromise = new Promise<Blob>((resolve) => {
          webcamRecorder.onstop = () => {
            resolve(new Blob(webcamChunksRef.current, { type: "video/webm" }));
          };
        });

        webcamRecorder.start(1000);
      }

      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          chunksRef.current.push(event.data);
        }
      };

      recorder.onstop = async () => {
        const blob = new Blob(chunksRef.current, { type: "video/webm" });
        const elapsed = elapsedSeconds();
        const webcamBlob = webcamBlobPromise ? await webcamBlobPromise : undefined;
        onRecordingComplete(blob, elapsed, webcamBlob);
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

  // Attach webcam stream to video element when ref becomes available
  useEffect(() => {
    if (webcamVideoRef.current && webcamStreamRef.current) {
      webcamVideoRef.current.srcObject = webcamStreamRef.current;
    }
  }, [webcamEnabled]);

  if (recordingState === "idle") {
    return (
      <div style={{ display: "flex", flexDirection: "column", gap: 16, alignItems: "center" }}>
        {maxDurationSeconds > 0 && (
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: 0 }}>
            Maximum recording length: {formatDuration(maxDurationSeconds)}
          </p>
        )}
        <div style={{ display: "flex", gap: 12, alignItems: "center" }}>
          <button
            onClick={toggleWebcam}
            aria-label={webcamEnabled ? "Disable camera" : "Enable camera"}
            style={{
              background: webcamEnabled ? "var(--color-accent)" : "transparent",
              color: webcamEnabled ? "var(--color-text)" : "var(--color-text-secondary)",
              border: webcamEnabled ? "none" : "1px solid var(--color-border)",
              borderRadius: 8,
              padding: "14px 24px",
              fontSize: 14,
              fontWeight: 600,
            }}
          >
            {webcamEnabled ? "Camera On" : "Camera Off"}
          </button>
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
        {webcamEnabled && (
          <div style={{ position: "relative", marginTop: 8 }}>
            <video
              ref={webcamVideoRef}
              autoPlay
              muted
              playsInline
              className="pip-preview"
              style={{ width: 160, height: 120 }}
            />
          </div>
        )}
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

      {webcamEnabled && (
        <video
          ref={webcamVideoRef}
          autoPlay
          muted
          playsInline
          className="pip-preview"
          style={{ width: 160, height: 120 }}
        />
      )}
    </div>
  );
}
