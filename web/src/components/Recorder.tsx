import { useCallback, useEffect, useRef, useState } from "react";
import { useDrawingCanvas } from "../hooks/useDrawingCanvas";
import { useCanvasCompositing } from "../hooks/useCanvasCompositing";
import {
  useRecordingLifecycle,
  type RecordingCommand,
} from "../hooks/useRecordingLifecycle";
import { getSupportedMimeType, blobTypeFromMimeType } from "../utils/mediaFormat";
import { formatDuration } from "../utils/format";
import { MIN_RECORDING_BYTES, MIN_RECORDING_SECONDS } from "../utils/recordingLimits";
import { useI18n } from "../i18n/I18nContext";

interface RecorderProps {
  onRecordingComplete: (blob: Blob, duration: number, webcamBlob?: Blob, cameraPosition?: string) => void;
  onRecordingError?: (message: string) => void;
  maxDurationSeconds?: number;
}

export function Recorder({ onRecordingComplete, onRecordingError, maxDurationSeconds = 0 }: RecorderProps) {
  const { t } = useI18n();
  const [webcamEnabled, setWebcamEnabled] = useState(false);
  const [cameraPosition, setCameraPosition] = useState(() => {
    if (typeof window === "undefined") return "bottom-right";
    return localStorage.getItem("recording-camera-position") || "bottom-right";
  });
  useEffect(() => {
    localStorage.setItem("recording-camera-position", cameraPosition);
  }, [cameraPosition]);

  const [captureWidth, setCaptureWidth] = useState(1920);
  const [captureHeight, setCaptureHeight] = useState(1080);
  const [previewExpanded, setPreviewExpanded] = useState(false);
  const [systemAudioEnabled, setSystemAudioEnabled] = useState(() => localStorage.getItem("recording-audio") !== "false");
  const [mediaError, setMediaError] = useState<string | null>(null);
  const countdownEnabled = useRef(localStorage.getItem("recording-countdown") !== "false");

  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const screenStreamRef = useRef<MediaStream | null>(null);
  const micStreamRef = useRef<MediaStream | null>(null);
  const audioContextRef = useRef<AudioContext | null>(null);

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

  const stopMicStream = useCallback(() => {
    if (micStreamRef.current) {
      micStreamRef.current.getTracks().forEach((track) => track.stop());
      micStreamRef.current = null;
    }
    if (audioContextRef.current) {
      audioContextRef.current.close();
      audioContextRef.current = null;
    }
  }, []);

  const stopAllStreams = useCallback(() => {
    if (screenStreamRef.current) {
      screenStreamRef.current.getTracks().forEach((track) => track.stop());
      screenStreamRef.current = null;
    }
    stopMicStream();
    stopWebcamStream();
  }, [stopMicStream, stopWebcamStream]);

  const performRecordingCommand = useCallback((command: RecordingCommand) => {
    const recorder = mediaRecorderRef.current;

    if (command === "start") {
      // No timeslice: Chrome's MP4 MediaRecorder may produce empty fragments
      // with start(timeslice) on getDisplayMedia() streams.
      recorder?.start();
      webcamRecorderRef.current?.start(1000);
      return;
    }

    if (command === "pause") {
      if (recorder?.state !== "recording") return false;
      recorder.pause();
      if (webcamRecorderRef.current?.state === "recording") {
        webcamRecorderRef.current.pause();
      }
      return;
    }

    if (command === "resume") {
      if (recorder?.state !== "paused") return false;
      recorder.resume();
      if (webcamRecorderRef.current?.state === "paused") {
        webcamRecorderRef.current.resume();
      }
      return;
    }

    const hasActiveRecorder = mediaRecorderRef.current && mediaRecorderRef.current.state !== "inactive";

    if (hasActiveRecorder) {
      mediaRecorderRef.current!.stop();
    }
    if (webcamRecorderRef.current && webcamRecorderRef.current.state !== "inactive") {
      if (webcamRecorderRef.current.state === "paused") {
        webcamRecorderRef.current.resume();
      }
      webcamRecorderRef.current.stop();
    }
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
  }, [stopAllStreams, stopCompositing]);

  const recording = useRecordingLifecycle({
    maxDurationSeconds,
    perform: performRecordingCommand,
  });
  const { dispatch, elapsedSeconds } = recording;

  function stopRecording() {
    dispatch({ type: "stop" });
  }

  function pauseRecording() {
    dispatch({ type: "pause" });
  }

  function resumeRecording() {
    dispatch({ type: "resume" });
  }

  const abortCountdown = useCallback(() => {
    dispatch({ type: "cancel-countdown" });
    stopCompositing();
    if (screenVideoRef.current) {
      screenVideoRef.current.srcObject = null;
    }
    stopAllStreams();
    mediaRecorderRef.current = null;
    webcamRecorderRef.current = null;
    webcamBlobPromiseRef.current = null;
  }, [dispatch, stopAllStreams, stopCompositing]);

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
      console.error("Kamerazugriff fehlgeschlagen", err);
      setMediaError("Auf die Kamera konnte nicht zugegriffen werden. Bitte erlaube den Kamerazugriff und versuche es erneut.");
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

      // Capture microphone audio separately — getDisplayMedia only provides
      // system/tab audio, never microphone input. MediaRecorder only records
      // one audio track, so we use AudioContext to mix system + mic audio
      // into a single track.
      let recordingStream: MediaStream = screenStream;
      if (systemAudioEnabled) {
        try {
          const micStream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false });
          micStreamRef.current = micStream;

          const audioContext = new AudioContext();
          audioContextRef.current = audioContext;
          const destination = audioContext.createMediaStreamDestination();

          // Connect system audio (if present) to the mixer
          if (screenStream.getAudioTracks().length > 0) {
            audioContext.createMediaStreamSource(screenStream).connect(destination);
          }

          // Connect microphone to the mixer
          audioContext.createMediaStreamSource(micStream).connect(destination);

          // Build recording stream: screen video + mixed audio
          recordingStream = new MediaStream([
            ...screenStream.getVideoTracks(),
            ...destination.stream.getAudioTracks(),
          ]);
        } catch (micErr) {
          console.warn("Mikrofonzugriff verweigert – Aufnahme ohne Mikrofonton", micErr);
        }
      }

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

      // Record the combined stream (screen video + system audio + mic audio)
      // directly — NOT through the canvas. Canvas compositing freezes when the
      // tab goes to the background because requestAnimationFrame/setInterval are
      // throttled. The raw streams keep capturing regardless of tab visibility.
      const mimeType = getSupportedMimeType();
      mimeTypeRef.current = mimeType;

      const recorder = new MediaRecorder(recordingStream, {
        mimeType,
      });
      mediaRecorderRef.current = recorder;
      chunksRef.current = [];

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
              console.warn("Kamera-Verarbeitung nach 10 Sekunden abgebrochen");
              resolve(undefined);
            }, 10_000);
          });
          webcamBlob = await Promise.race([webcamBlobPromiseRef.current, timeout]);
        }

        stopAllStreams();

        if (elapsed < MIN_RECORDING_SECONDS || blob.size < MIN_RECORDING_BYTES) {
          onRecordingError?.("Die Aufnahme ist zu kurz. Bitte nimm mindestens 1 Sekunde auf.");
          return;
        }

        onRecordingComplete(blob, elapsed, webcamBlob, cameraPosition);
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

        const fallback = new MediaRecorder(recordingStream, { mimeType: webmMimeType });
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
          dispatch({ type: "stop" });
        } else {
          abortCountdown();
        }
      });

      dispatch({
        type: "request-start",
        countdown: countdownEnabled.current,
      });
    } catch (err) {
      console.error("Bildschirmaufnahme fehlgeschlagen", err);
      setMediaError("Die Bildschirmaufnahme wurde blockiert oder ist fehlgeschlagen. Bitte erlaube die Bildschirmfreigabe und versuche es erneut.");
      stopAllStreams();
    }
  }

  useEffect(() => {
    return () => {
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

  const {
    state,
    elapsed: duration,
    countdown: countdownValue,
    remaining,
  } = recording.snapshot;
  const isIdle = state === "idle";
  const isCountdown = state === "countdown";
  const isPaused = state === "paused";
  const isActive = state !== "idle" && state !== "stopped";
  const isRecording = state === "recording" || isPaused;

  return (
    <div className="recorder-container">
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
          aria-label={previewExpanded ? "Vorschau einklappen" : "Vorschau ausklappen"}
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
            onClick={() => dispatch({ type: "start-now" })}
          >
            <div className="countdown-number">{countdownValue}</div>
            <div className="countdown-hint">Klicken, um sofort zu starten</div>
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
                aria-label="Fehler schließen"
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
            <p className="max-duration-label">
              {t("record.maxDuration", { duration: formatDuration(maxDurationSeconds) })}
            </p>
          )}
          <div className="record-controls">
            <button
              onClick={toggleWebcam}
              aria-label={webcamEnabled ? t("record.cameraDisable") : t("record.cameraEnable")}
              className={`btn-secondary${webcamEnabled ? " btn-secondary--active" : ""}`}
            >
              {webcamEnabled ? t("record.cameraOn") : t("record.cameraOff")}
            </button>
        {webcamEnabled && (
          <div
            className="camera-position-controls"
            style={{ display: "flex", gap: 6, alignItems: "center" }}
          >
            {[
              ["top-left", "↖"],
              ["top-right", "↗"],
              ["bottom-left", "↙"],
              ["bottom-right", "↘"],
            ].map(([position, symbol]) => (
              <button
                key={position}
                type="button"
                onClick={() => setCameraPosition(position)}
                className={`btn-secondary${cameraPosition === position ? " btn-secondary--active" : ""}`}
                aria-label={`Kameraposition ${position}`}
                title={`Kameraposition ${position}`}
                style={{ minWidth: 38, padding: "8px 10px" }}
              >
                {symbol}
              </button>
            ))}
          </div>
        )}

            <button
              onClick={() => setSystemAudioEnabled((prev) => !prev)}
              aria-label={systemAudioEnabled ? t("record.audioDisable") : t("record.audioEnable")}
              className={`btn-secondary${systemAudioEnabled ? " btn-secondary--active" : ""}`}
            >
              {systemAudioEnabled ? t("record.audioOn") : t("record.audioOff")}
            </button>
            <button
              onClick={startRecording}
              aria-label={t("record.start")}
              className="btn-record"
            >
              {t("record.start")}
            </button>
          </div>
        </>
      )}

      {/* Recording controls — always above preview */}
      {isRecording && (
        <div className="recording-header" role="status" aria-live="polite">
          <div className={`recording-indicator ${isPaused ? "recording-indicator--paused" : "recording-indicator--active"}`}>
            <div className={`recording-dot ${isPaused ? "recording-dot--paused" : "recording-dot--active"}`} />
            {formatDuration(duration)}
            {isPaused && <span className="recording-remaining">({t("record.paused")})</span>}
            {!isPaused && remaining !== null && (
              <span className="recording-remaining">({t("record.remaining", { duration: formatDuration(remaining) })})</span>
            )}
          </div>

          <button
            onClick={toggleDrawMode}
            aria-label={drawMode ? "Zeichnen ausschalten" : "Zeichnen einschalten"}
            data-testid="draw-toggle"
            className={`btn-draw${drawMode ? " btn-draw--active" : ""}`}
          >
            {t("record.draw")}
          </button>

          {drawMode && (
            <input
              type="color"
              value={drawColor}
              onChange={(e) => setDrawColor(e.target.value)}
              aria-label="Zeichenfarbe"
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
              aria-label="Zeichnung löschen"
              data-testid="clear-drawing"
              className="btn-pause"
            >
              {t("record.clear")}
            </button>
          )}

          {drawMode && (
            <div style={{ display: "flex", gap: 4, alignItems: "center" }} data-testid="thickness-selector">
              {[2, 4, 8].map((w) => (
                <button
                  key={w}
                  onClick={() => setLineWidth(w)}
                  aria-label={`Linienstärke ${w}`}
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
            <button onClick={resumeRecording} aria-label="Aufnahme fortsetzen" className="btn-resume">
              {t("record.resume")}
            </button>
          ) : (
            <button onClick={pauseRecording} aria-label="Aufnahme pausieren" className="btn-pause">
              {t("record.pause")}
            </button>
          )}

          <button onClick={stopRecording} aria-label="Aufnahme stoppen" className="btn-stop">
            {t("record.stop")}
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
