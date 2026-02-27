import { useCallback, useEffect, useRef, useState } from "react";
import { useDrawingCanvas } from "../hooks/useDrawingCanvas";
import { useCanvasCompositing } from "../hooks/useCanvasCompositing";
import { getSupportedMimeType, blobTypeFromMimeType } from "../utils/mediaFormat";

type RecordingState = "idle" | "countdown" | "recording" | "paused" | "stopped";

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
  const [countdownValue, setCountdownValue] = useState(3);
  const [webcamEnabled, setWebcamEnabled] = useState(false);
  const [captureWidth, setCaptureWidth] = useState(1920);
  const [captureHeight, setCaptureHeight] = useState(1080);
  const [previewExpanded, setPreviewExpanded] = useState(false);
  const [systemAudioEnabled, setSystemAudioEnabled] = useState(true);
  const [mediaError, setMediaError] = useState<string | null>(null);

  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const startTimeRef = useRef<number>(0);
  const timerRef = useRef<ReturnType<typeof setInterval>>(0 as unknown as ReturnType<typeof setInterval>);
  const screenStreamRef = useRef<MediaStream | null>(null);
  const pauseStartRef = useRef<number>(0);
  const totalPausedRef = useRef<number>(0);

  const countdownTimerRef = useRef<ReturnType<typeof setInterval>>(0 as unknown as ReturnType<typeof setInterval>);
  const webcamStreamRef = useRef<MediaStream | null>(null);
  const webcamRecorderRef = useRef<MediaRecorder | null>(null);
  const webcamChunksRef = useRef<Blob[]>([]);
  const webcamBlobPromiseRef = useRef<Promise<Blob> | null>(null);
  const mimeTypeRef = useRef("");
  const webcamVideoCallbackRef = useCallback((node: HTMLVideoElement | null) => {
    if (node && webcamStreamRef.current) {
      node.srcObject = webcamStreamRef.current;
    }
  }, []);

  // Drawing and compositing refs
  const drawingCanvasRef = useRef<HTMLCanvasElement | null>(null);
  const compositingCanvasRef = useRef<HTMLCanvasElement | null>(null);
  const screenVideoRef = useRef<HTMLVideoElement | null>(null);

  const {
    drawMode,
    drawColor,
    lineWidth,
    toggleDrawMode,
    setDrawColor,
    setLineWidth,
    clearCanvas,
    handlePointerDown,
    handlePointerMove,
    handlePointerUp,
    handlePointerLeave,
  } = useDrawingCanvas({ canvasRef: drawingCanvasRef, captureWidth, captureHeight });

  const { startCompositing, stopCompositing } =
    useCanvasCompositing({
      compositingCanvasRef,
      screenVideoRef,
      drawingCanvasRef,
    });

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
    const hasActiveRecorder = mediaRecorderRef.current && mediaRecorderRef.current.state !== "inactive";

    if (hasActiveRecorder) {
      if (mediaRecorderRef.current!.state === "paused") {
        totalPausedRef.current += Date.now() - pauseStartRef.current;
      }
      mediaRecorderRef.current!.stop();
    }
    if (webcamRecorderRef.current && webcamRecorderRef.current.state !== "inactive") {
      if (webcamRecorderRef.current.state === "paused") {
        webcamRecorderRef.current.resume();
      }
      webcamRecorderRef.current.stop();
    }
    clearInterval(timerRef.current);
    stopCompositing();
    if (screenVideoRef.current) {
      screenVideoRef.current.srcObject = null;
    }
    // When recording screenStream directly, we must NOT stop the stream tracks
    // until after MediaRecorder fires its async onstop event and produces the
    // final data. Stream cleanup happens in the recorder's onstop handler.
    // Only clean up immediately if there's no active recorder (e.g. abort paths).
    if (!hasActiveRecorder) {
      stopAllStreams();
    }
    setRecordingState("stopped");
  }, [stopAllStreams, stopCompositing]);

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

  const beginRecording = useCallback(() => {
    clearInterval(countdownTimerRef.current);
    if (mediaRecorderRef.current) {
      // No timeslice — Chrome's MP4 MediaRecorder may produce empty fragments
      // with start(timeslice) on getDisplayMedia() streams. All data is buffered
      // internally and flushed as a single blob on stop().
      mediaRecorderRef.current.start();
    }
    if (webcamRecorderRef.current) {
      webcamRecorderRef.current.start(1000);
    }
    startTimeRef.current = Date.now();
    setDuration(0);
    startTimer();
    setRecordingState("recording");
  }, [startTimer]);

  const abortCountdown = useCallback(() => {
    clearInterval(countdownTimerRef.current);
    stopCompositing();
    if (screenVideoRef.current) {
      screenVideoRef.current.srcObject = null;
    }
    stopAllStreams();
    mediaRecorderRef.current = null;
    webcamRecorderRef.current = null;
    webcamBlobPromiseRef.current = null;
    setRecordingState("idle");
    setCountdownValue(3);
  }, [stopAllStreams, stopCompositing]);

  async function toggleWebcam() {
    setMediaError(null);
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
      setWebcamEnabled(true);
    } catch (err) {
      console.error("Webcam access failed", err);
      setMediaError("Could not access your camera. Please allow camera access and try again.");
    }
  }

  async function startRecording() {
    setMediaError(null);
    try {
      const displayMediaOptions: DisplayMediaStreamOptions & Record<string, unknown> = {
        video: true,
        audio: systemAudioEnabled,
      };
      if (systemAudioEnabled) {
        displayMediaOptions.systemAudio = "include";
        displayMediaOptions.suppressLocalAudioPlayback = true;
      }
      const screenStream = await navigator.mediaDevices.getDisplayMedia(displayMediaOptions);
      screenStreamRef.current = screenStream;

      // Play screen stream on preview video first
      if (screenVideoRef.current) {
        screenVideoRef.current.srcObject = screenStream;
        await screenVideoRef.current.play();
      }

      // Get actual video frame dimensions (not constrained settings)
      const width = screenVideoRef.current?.videoWidth || 1920;
      const height = screenVideoRef.current?.videoHeight || 1080;
      setCaptureWidth(width);
      setCaptureHeight(height);

      // Set canvas dimensions to match actual video frames
      if (compositingCanvasRef.current) {
        compositingCanvasRef.current.width = width;
        compositingCanvasRef.current.height = height;
      }
      if (drawingCanvasRef.current) {
        drawingCanvasRef.current.width = width;
        drawingCanvasRef.current.height = height;
      }

      // Start compositing loop (for visual preview only)
      startCompositing();

      // Record the original display stream directly — NOT through the canvas.
      // Canvas compositing freezes when the tab goes to the background because
      // requestAnimationFrame/setInterval are throttled and the video element
      // stops decoding frames. The raw getDisplayMedia stream keeps capturing
      // regardless of tab visibility.
      const mimeType = getSupportedMimeType();
      mimeTypeRef.current = mimeType;

      const recorder = new MediaRecorder(screenStream, {
        mimeType,
      });
      mediaRecorderRef.current = recorder;
      chunksRef.current = [];
      pauseStartRef.current = 0;
      totalPausedRef.current = 0;

      // Set up webcam recorder if webcam is enabled (but don't start yet).
      // Always use WebM for webcam — it's only used temporarily for server-side compositing,
      // and WebM is more reliable for video-only MediaRecorder streams across browsers.
      webcamBlobPromiseRef.current = null;
      if (webcamEnabled && webcamStreamRef.current) {
        const webcamMimeType = MediaRecorder.isTypeSupported("video/webm;codecs=vp9")
          ? "video/webm;codecs=vp9"
          : "video/webm";
        const webcamRecorder = new MediaRecorder(webcamStreamRef.current, {
          mimeType: webcamMimeType,
        });
        webcamRecorderRef.current = webcamRecorder;
        webcamChunksRef.current = [];

        webcamRecorder.ondataavailable = (event) => {
          if (event.data.size > 0) {
            webcamChunksRef.current.push(event.data);
          }
        };

        webcamBlobPromiseRef.current = new Promise<Blob>((resolve) => {
          webcamRecorder.onstop = () => {
            resolve(new Blob(webcamChunksRef.current, { type: "video/webm" }));
          };
        });
      }

      const handleDataAvailable = (event: BlobEvent) => {
        if (event.data.size > 0) {
          chunksRef.current.push(event.data);
        }
      };

      const handleStop = async () => {
        const blob = new Blob(chunksRef.current, { type: blobTypeFromMimeType(mimeTypeRef.current) });
        const elapsed = elapsedSeconds();

        let webcamBlob: Blob | undefined;
        if (webcamBlobPromiseRef.current) {
          const timeout = new Promise<undefined>((resolve) => {
            setTimeout(() => {
              console.warn("Webcam blob promise timed out after 10s");
              resolve(undefined);
            }, 10_000);
          });
          webcamBlob = await Promise.race([webcamBlobPromiseRef.current, timeout]);
        }

        stopAllStreams();
        onRecordingComplete(blob, elapsed, webcamBlob);
      };

      // Track whether the encoder failed so the original onstop is skipped
      // when a fallback recorder takes over.
      let encoderFailed = false;

      recorder.ondataavailable = handleDataAvailable;

      recorder.onerror = () => {
        // Chrome's H.264 encoder fails for high-resolution display captures
        // (e.g., Retina screens exceeding encoder limits). Fall back to WebM.
        if (!mimeTypeRef.current.startsWith("video/mp4")) return;
        encoderFailed = true;

        const webmMimeType = MediaRecorder.isTypeSupported("video/webm;codecs=vp9,opus")
          ? "video/webm;codecs=vp9,opus"
          : "video/webm";
        mimeTypeRef.current = webmMimeType;
        chunksRef.current = [];

        const fallback = new MediaRecorder(screenStream, { mimeType: webmMimeType });
        mediaRecorderRef.current = fallback;
        fallback.ondataavailable = handleDataAvailable;
        fallback.onstop = handleStop;
        fallback.start();
      };

      recorder.onstop = async () => {
        if (encoderFailed) return;
        await handleStop();
      };

      screenStream.getVideoTracks()[0].addEventListener("ended", () => {
        if (mediaRecorderRef.current && mediaRecorderRef.current.state !== "inactive") {
          stopRecording();
        } else {
          abortCountdown();
        }
      });

      setCountdownValue(3);
      setRecordingState("countdown");
    } catch (err) {
      console.error("Screen capture failed", err);
      setMediaError("Screen recording was blocked or failed. Please allow screen capture and try again.");
      stopAllStreams();
    }
  }

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
      stopCompositing();
      stopAllStreams();
    };
  }, [stopAllStreams, stopCompositing]);


  useEffect(() => {
    if (previewExpanded) {
      document.documentElement.style.overflowX = "hidden";
      return () => { document.documentElement.style.overflowX = ""; };
    }
  }, [previewExpanded]);

  const isIdle = recordingState === "idle";
  const isCountdown = recordingState === "countdown";
  const isPaused = recordingState === "paused";
  const isActive = !isIdle && recordingState !== "stopped";
  const isRecording = recordingState === "recording" || isPaused;
  const remaining = maxDurationSeconds > 0 ? maxDurationSeconds - duration : null;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16, alignItems: "center" }}>
      {/* Screen preview with drawing overlay — hidden in idle, visible during recording */}
      <div style={{
        position: "relative",
        display: isActive ? "block" : "none",
        ...(previewExpanded
          ? { width: "100vw", maxWidth: "none" }
          : { width: "100%", maxWidth: 960 }),
      }}>
        <video
          ref={screenVideoRef}
          autoPlay
          muted
          playsInline
          data-testid="screen-preview"
          style={{
            width: "100%",
            borderRadius: previewExpanded ? 0 : 8,
            background: "#000",
            display: "block",
          }}
        />
        <canvas
          ref={drawingCanvasRef}
          data-testid="drawing-canvas"
          onPointerDown={handlePointerDown}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
          onPointerLeave={handlePointerLeave}
          style={{
            position: "absolute",
            top: 0,
            left: 0,
            width: "100%",
            height: "100%",
            cursor: drawMode ? "crosshair" : "default",
            touchAction: "none",
            pointerEvents: drawMode ? "auto" : "none",
          }}
        />
        <button
          onClick={() => setPreviewExpanded((prev) => !prev)}
          aria-label={previewExpanded ? "Collapse preview" : "Expand preview"}
          data-testid="expand-preview"
          style={{
            position: "absolute",
            top: 8,
            right: 8,
            zIndex: 10,
            width: 32,
            height: 32,
            borderRadius: 6,
            border: "none",
            background: "rgba(0, 0, 0, 0.6)",
            color: "#fff",
            cursor: "pointer",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: 16,
            padding: 0,
          }}
          title={previewExpanded ? "Collapse" : "Expand"}
        >
          {previewExpanded ? "\u2199" : "\u2197"}
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

      {/* Hidden compositing canvas — always mounted so ref is available */}
      <canvas
        ref={compositingCanvasRef}
        data-testid="compositing-canvas"
        style={{ display: "none" }}
      />

      {/* Idle UI */}
      {isIdle && (
        <>
          {mediaError && (
            <div
              role="alert"
              style={{
                background: "rgba(239, 68, 68, 0.1)",
                border: "1px solid var(--color-error)",
                borderRadius: 8,
                padding: "12px 16px",
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: 12,
                width: "100%",
                maxWidth: 480,
              }}
            >
              <span style={{ color: "var(--color-error)", fontSize: 13 }}>{mediaError}</span>
              <button
                onClick={() => setMediaError(null)}
                aria-label="Dismiss error"
                style={{
                  background: "transparent",
                  color: "var(--color-error)",
                  border: "none",
                  fontSize: 16,
                  cursor: "pointer",
                  padding: "2px 6px",
                  lineHeight: 1,
                }}
              >
                &times;
              </button>
            </div>
          )}
          {maxDurationSeconds > 0 && (
            <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: 0 }}>
              Maximum recording length: {formatDuration(maxDurationSeconds)}
            </p>
          )}
          <div style={{ display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap", justifyContent: "center" }}>
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
              onClick={() => setSystemAudioEnabled((prev) => !prev)}
              aria-label={systemAudioEnabled ? "Disable system audio" : "Enable system audio"}
              style={{
                background: systemAudioEnabled ? "var(--color-accent)" : "transparent",
                color: systemAudioEnabled ? "var(--color-text)" : "var(--color-text-secondary)",
                border: systemAudioEnabled ? "none" : "1px solid var(--color-border)",
                borderRadius: 8,
                padding: "14px 24px",
                fontSize: 14,
                fontWeight: 600,
              }}
            >
              {systemAudioEnabled ? "Audio On" : "Audio Off"}
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
        </>
      )}

      {/* Recording controls — always above preview */}
      {isRecording && (
        <div role="status" aria-live="polite" style={{ display: "flex", alignItems: "center", gap: 12, flexWrap: "wrap", justifyContent: "center", order: -1 }}>
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

          <button
            onClick={toggleDrawMode}
            aria-label={drawMode ? "Disable drawing" : "Enable drawing"}
            data-testid="draw-toggle"
            style={{
              background: drawMode ? "var(--color-accent)" : "transparent",
              color: drawMode ? "var(--color-text)" : "var(--color-text-secondary)",
              border: drawMode ? "none" : "1px solid var(--color-border)",
              borderRadius: 8,
              padding: "10px 24px",
              fontSize: 14,
              fontWeight: 600,
            }}
          >
            Draw
          </button>

          {drawMode && (
            <input
              type="color"
              value={drawColor}
              onChange={(e) => setDrawColor(e.target.value)}
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
              onClick={clearCanvas}
              aria-label="Clear drawing"
              data-testid="clear-drawing"
              style={{
                background: "transparent",
                color: "var(--color-text-secondary)",
                border: "1px solid var(--color-border)",
                borderRadius: 8,
                padding: "10px 24px",
                fontSize: 14,
                fontWeight: 600,
              }}
            >
              Clear
            </button>
          )}

          {drawMode && (
            <div style={{ display: "flex", gap: 4, alignItems: "center" }} data-testid="thickness-selector">
              {[2, 4, 8].map((w) => (
                <button
                  key={w}
                  onClick={() => setLineWidth(w)}
                  aria-label={`Line width ${w}`}
                  style={{
                    width: 28,
                    height: 28,
                    borderRadius: "50%",
                    border: lineWidth === w ? "2px solid var(--color-accent)" : "1px solid var(--color-border)",
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
      )}

      {webcamEnabled && (
        <video
          ref={webcamVideoCallbackRef}
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
