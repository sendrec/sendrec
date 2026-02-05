import { useCallback, useEffect, useRef, useState } from "react";

type RecordingState = "idle" | "recording" | "stopped";
type WebcamPosition = "bottom-right" | "bottom-left" | "top-right" | "top-left";

interface RecorderProps {
  onRecordingComplete: (blob: Blob, duration: number) => void;
}

const WEBCAM_WIDTH = 200;
const WEBCAM_HEIGHT = 150;
const WEBCAM_PADDING = 16;
const WEBCAM_BORDER_COLOR = "rgba(255, 255, 255, 0.3)";

function formatDuration(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  const remaining = seconds % 60;
  return `${minutes}:${String(remaining).padStart(2, "0")}`;
}

function getWebcamCoordinates(
  position: WebcamPosition,
  canvasWidth: number,
  canvasHeight: number
): { x: number; y: number } {
  switch (position) {
    case "bottom-right":
      return {
        x: canvasWidth - WEBCAM_WIDTH - WEBCAM_PADDING,
        y: canvasHeight - WEBCAM_HEIGHT - WEBCAM_PADDING,
      };
    case "bottom-left":
      return { x: WEBCAM_PADDING, y: canvasHeight - WEBCAM_HEIGHT - WEBCAM_PADDING };
    case "top-right":
      return { x: canvasWidth - WEBCAM_WIDTH - WEBCAM_PADDING, y: WEBCAM_PADDING };
    case "top-left":
      return { x: WEBCAM_PADDING, y: WEBCAM_PADDING };
  }
}

export function Recorder({ onRecordingComplete }: RecorderProps) {
  const [recordingState, setRecordingState] = useState<RecordingState>("idle");
  const [webcamEnabled, setWebcamEnabled] = useState(false);
  const [webcamPosition, setWebcamPosition] = useState<WebcamPosition>("bottom-right");
  const [duration, setDuration] = useState(0);

  const canvasRef = useRef<HTMLCanvasElement>(null);
  const screenVideoRef = useRef<HTMLVideoElement>(null);
  const webcamVideoRef = useRef<HTMLVideoElement>(null);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const animationFrameRef = useRef<number>(0);
  const startTimeRef = useRef<number>(0);
  const timerRef = useRef<ReturnType<typeof setInterval>>(0 as unknown as ReturnType<typeof setInterval>);
  const screenStreamRef = useRef<MediaStream | null>(null);
  const webcamStreamRef = useRef<MediaStream | null>(null);
  const webcamPositionRef = useRef<WebcamPosition>(webcamPosition);
  const webcamEnabledRef = useRef<boolean>(webcamEnabled);

  webcamPositionRef.current = webcamPosition;
  webcamEnabledRef.current = webcamEnabled;

  const stopAllStreams = useCallback(() => {
    if (screenStreamRef.current) {
      screenStreamRef.current.getTracks().forEach((track) => track.stop());
      screenStreamRef.current = null;
    }
    if (webcamStreamRef.current) {
      webcamStreamRef.current.getTracks().forEach((track) => track.stop());
      webcamStreamRef.current = null;
    }
  }, []);

  const drawFrame = useCallback(() => {
    const canvas = canvasRef.current;
    const screenVideo = screenVideoRef.current;
    if (!canvas || !screenVideo) return;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    ctx.drawImage(screenVideo, 0, 0, canvas.width, canvas.height);

    const webcamVideo = webcamVideoRef.current;
    if (webcamEnabledRef.current && webcamVideo && webcamVideo.readyState >= 2) {
      const { x, y } = getWebcamCoordinates(
        webcamPositionRef.current,
        canvas.width,
        canvas.height
      );

      ctx.strokeStyle = WEBCAM_BORDER_COLOR;
      ctx.lineWidth = 2;
      ctx.strokeRect(x - 1, y - 1, WEBCAM_WIDTH + 2, WEBCAM_HEIGHT + 2);
      ctx.drawImage(webcamVideo, x, y, WEBCAM_WIDTH, WEBCAM_HEIGHT);
    }

    animationFrameRef.current = requestAnimationFrame(drawFrame);
  }, []);

  const stopRecording = useCallback(() => {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state !== "inactive") {
      mediaRecorderRef.current.stop();
    }
    cancelAnimationFrame(animationFrameRef.current);
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

      const screenVideo = screenVideoRef.current;
      if (!screenVideo) return;
      screenVideo.srcObject = screenStream;
      await screenVideo.play();

      const canvas = canvasRef.current;
      if (!canvas) return;
      canvas.width = screenVideo.videoWidth;
      canvas.height = screenVideo.videoHeight;

      if (webcamEnabled) {
        try {
          const webcamStream = await navigator.mediaDevices.getUserMedia({
            video: { width: 320, height: 240 },
            audio: false,
          });
          webcamStreamRef.current = webcamStream;

          const webcamVideo = webcamVideoRef.current;
          if (webcamVideo) {
            webcamVideo.srcObject = webcamStream;
            await webcamVideo.play();
          }
        } catch {
          // Webcam not available; continue without it
        }
      }

      drawFrame();

      const canvasStream = canvas.captureStream(30);
      const audioTracks = screenStream.getAudioTracks();
      for (const track of audioTracks) {
        canvasStream.addTrack(track);
      }

      const recorder = new MediaRecorder(canvasStream, {
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
    } catch {
      stopAllStreams();
    }
  }

  useEffect(() => {
    return () => {
      cancelAnimationFrame(animationFrameRef.current);
      clearInterval(timerRef.current);
      stopAllStreams();
    };
  }, [stopAllStreams]);

  if (recordingState === "idle") {
    return (
      <div style={{ display: "flex", flexDirection: "column", gap: 16, alignItems: "center" }}>
        <label
          style={{
            display: "flex",
            alignItems: "center",
            gap: 8,
            color: "var(--color-text-secondary)",
            fontSize: 14,
          }}
        >
          <input
            type="checkbox"
            checked={webcamEnabled}
            onChange={(e) => setWebcamEnabled(e.target.checked)}
          />
          Include webcam
        </label>

        {webcamEnabled && (
          <label
            style={{
              display: "flex",
              alignItems: "center",
              gap: 8,
              color: "var(--color-text-secondary)",
              fontSize: 14,
            }}
          >
            Webcam position:
            <select
              value={webcamPosition}
              onChange={(e) => setWebcamPosition(e.target.value as WebcamPosition)}
              style={{
                background: "var(--color-bg)",
                border: "1px solid var(--color-border)",
                borderRadius: 4,
                color: "var(--color-text)",
                padding: "4px 8px",
                fontSize: 14,
              }}
            >
              <option value="bottom-right">Bottom Right</option>
              <option value="bottom-left">Bottom Left</option>
              <option value="top-right">Top Right</option>
              <option value="top-left">Top Left</option>
            </select>
          </label>
        )}

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

        <video ref={screenVideoRef} style={{ display: "none" }} muted />
        <video ref={webcamVideoRef} style={{ display: "none" }} muted />
        <canvas ref={canvasRef} style={{ display: "none" }} />
      </div>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16, alignItems: "center" }}>
      <canvas
        ref={canvasRef}
        style={{ width: "100%", borderRadius: 8, background: "#000" }}
      />

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
          Stop
        </button>
      </div>

      <video ref={screenVideoRef} style={{ display: "none" }} muted />
      <video ref={webcamVideoRef} style={{ display: "none" }} muted />

      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.3; }
        }
      `}</style>
    </div>
  );
}
