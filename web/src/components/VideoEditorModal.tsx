import { useEffect, useRef, useState } from "react";
import { apiFetch } from "../api/client";
import { formatDuration } from "../utils/format";
import type { Video } from "../types/video";

interface EditorClip {
  id: string;
  sourceVideoId: string;
  start: number;
  end: number;
}

interface VideoEditorModalProps {
  videoId: string;
  duration: number;
  onClose: () => void;
  onTrimStarted?: () => void;
}

export function VideoEditorModal({
  videoId,
  duration,
  onClose,
  onTrimStarted,
}: VideoEditorModalProps) {
  const [videoUrl, setVideoUrl] = useState<string | null>(null);
  const [currentTime, setCurrentTime] = useState(0);
  const [showInsertPicker, setShowInsertPicker] = useState(false);
  const [libraryVideos, setLibraryVideos] = useState<Video[]>([]);
  const [loadingLibrary, setLoadingLibrary] = useState(false);
  const [selectedInsertVideo, setSelectedInsertVideo] = useState<Video | null>(null);
  const [trimStart, setTrimStart] = useState(0);
  const [trimEnd, setTrimEnd] = useState(duration);
  const [trimming, setTrimming] = useState(false);
  const [selectedClipId, setSelectedClipId] = useState<string | null>(null);
  const [clipHistory, setClipHistory] = useState<EditorClip[][]>([]);
  const [clips, setClips] = useState<EditorClip[]>([
    {
      id: "clip-1",
      sourceVideoId: videoId,
      start: 0,
      end: duration,
    },
  ]);
  const [error, setError] = useState<string | null>(null);

  const videoRef = useRef<HTMLVideoElement>(null);
  const nextClipIdRef = useRef(2);
  const timelineRef = useRef<HTMLDivElement>(null);
  const draggingTrimRef = useRef<"start" | "end" | null>(null);

  useEffect(() => {
    apiFetch<{ downloadUrl: string }>(`/api/videos/${videoId}/download`)
      .then((res) => {
        if (res?.downloadUrl) {
          setVideoUrl(res.downloadUrl);
        } else {
          setError("Video konnte nicht geladen werden.");
        }
      })
      .catch(() => setError("Video konnte nicht geladen werden."));
  }, [videoId]);

  useEffect(() => {
    setTrimEnd(duration);
    setClips([
      {
        id: "clip-1",
        sourceVideoId: videoId,
        start: 0,
        end: duration,
      },
    ]);
    nextClipIdRef.current = 2;
    setSelectedClipId(null);
    setClipHistory([]);
  }, [duration, videoId]);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  const timelineDuration = clips.reduce(
    (sum, clip) => sum + Math.max(0, clip.end - clip.start),
    0,
  );

  function sourceTimeToTimelineTime(sourceTime: number) {
    let offset = 0;

    for (const clip of clips) {
      const clipDuration = clip.end - clip.start;

      if (sourceTime < clip.start) {
        return offset;
      }

      if (sourceTime <= clip.end) {
        return offset + (sourceTime - clip.start);
      }

      offset += clipDuration;
    }

    return offset;
  }

  function timelineTimeToSourceTime(timelineTime: number) {
    let remaining = Math.max(0, timelineTime);

    for (const clip of clips) {
      const clipDuration = clip.end - clip.start;

      if (remaining <= clipDuration) {
        return clip.start + remaining;
      }

      remaining -= clipDuration;
    }

    return clips.length > 0
      ? clips[clips.length - 1].end
      : 0;
  }

  function timeFromClientX(clientX: number) {
    const timeline = timelineRef.current;
    if (!timeline || !duration) return 0;

    const rect = timeline.getBoundingClientRect();
    const x = Math.max(0, Math.min(clientX - rect.left, rect.width));
    return (x / rect.width) * duration;
  }

  function handleTrimPointerDown(handle: "start" | "end") {
    return (e: React.MouseEvent | React.TouchEvent) => {
      e.preventDefault();
      e.stopPropagation();
      draggingTrimRef.current = handle;

      const fixedStart = trimStart;
      const fixedEnd = trimEnd;
      const minimumGap = 1;

      function onMove(ev: MouseEvent | TouchEvent) {
        const point = "touches" in ev ? ev.touches[0] : ev;
        if (!point) return;

        const rawTime = timeFromClientX(point.clientX);

        let nextTime = rawTime;

        if (handle === "start") {
          nextTime = Math.max(
            0,
            Math.min(rawTime, fixedEnd - minimumGap)
          );
          setTrimStart(nextTime);
        } else {
          nextTime = Math.min(
            duration,
            Math.max(rawTime, fixedStart + minimumGap)
          );
          setTrimEnd(nextTime);
        }

        setCurrentTime(nextTime);

        if (videoRef.current) {
          videoRef.current.currentTime = nextTime;
        }
      }

      function onUp() {
        draggingTrimRef.current = null;
        document.removeEventListener("mousemove", onMove);
        document.removeEventListener("mouseup", onUp);
        document.removeEventListener("touchmove", onMove);
        document.removeEventListener("touchend", onUp);
      }

      document.addEventListener("mousemove", onMove);
      document.addEventListener("mouseup", onUp);
      document.addEventListener("touchmove", onMove, { passive: false });
      document.addEventListener("touchend", onUp);
    };
  }

  function handleTimelineClick(e: React.MouseEvent<HTMLDivElement>) {
    if (!timelineDuration) return;

    const rect = e.currentTarget.getBoundingClientRect();
    const x = Math.max(0, Math.min(e.clientX - rect.left, rect.width));

    const timelineTime =
      (x / rect.width) * timelineDuration;

    const nextTime =
      timelineTimeToSourceTime(timelineTime);

    setCurrentTime(nextTime);

    if (videoRef.current) {
      videoRef.current.currentTime = nextTime;
    }
  }

  async function handleOpenInsertPicker() {
    setShowInsertPicker(true);
    setLoadingLibrary(true);
    setError(null);

    try {
      const videos = await apiFetch<Video[]>("/api/videos");

      setLibraryVideos(
        (videos ?? []).filter(
          (video) =>
            video.id !== videoId &&
            video.status === "ready" &&
            video.duration > 0,
        ),
      );
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Videobibliothek konnte nicht geladen werden.",
      );
    } finally {
      setLoadingLibrary(false);
    }
  }

  function rememberClipState() {
    setClipHistory((history) => [
      ...history.slice(-49),
      clips.map((clip) => ({ ...clip })),
    ]);
  }

  function handleUndo() {
    if (clipHistory.length === 0) return;

    const previousClips =
      clipHistory[clipHistory.length - 1];

    setClips(previousClips);
    setClipHistory((history) => history.slice(0, -1));
    setSelectedClipId(null);
    setError(null);
  }

  function handleSplit() {
    const minimumDistance = 0.1;

    const clipIndex = clips.findIndex(
      (clip) =>
        currentTime > clip.start + minimumDistance &&
        currentTime < clip.end - minimumDistance,
    );

    if (clipIndex === -1) {
      setError(
        "Zum Teilen muss der Abspielkopf innerhalb eines Clips stehen.",
      );
      return;
    }

    rememberClipState();

    setClips((previousClips) => {
      const index = previousClips.findIndex(
        (clip) =>
          currentTime > clip.start + minimumDistance &&
          currentTime < clip.end - minimumDistance,
      );

      if (index === -1) return previousClips;

      const clip = previousClips[index];

      const leftClip: EditorClip = {
        ...clip,
        id: `clip-${nextClipIdRef.current++}`,
        end: currentTime,
      };

      const rightClip: EditorClip = {
        ...clip,
        id: `clip-${nextClipIdRef.current++}`,
        start: currentTime,
      };

      return [
        ...previousClips.slice(0, index),
        leftClip,
        rightClip,
        ...previousClips.slice(index + 1),
      ];
    });

    setSelectedClipId(null);
    setError(null);
  }

  function handleDeleteSelectedClip() {
    if (!selectedClipId) {
      setError("Bitte zuerst einen Clip auswählen.");
      return;
    }

    if (clips.length <= 1) {
      setError("Der letzte verbleibende Clip kann nicht gelöscht werden.");
      return;
    }

    const selectedClip = clips.find(
      (clip) => clip.id === selectedClipId,
    );

    if (!selectedClip) return;

    rememberClipState();

    setClips((previousClips) =>
      previousClips.filter(
        (clip) => clip.id !== selectedClipId,
      ),
    );

    setSelectedClipId(null);
    setError(null);
  }

  function handleResetTrim() {
    setTrimStart(0);
    setTrimEnd(duration);
    setCurrentTime(0);

    if (videoRef.current) {
      videoRef.current.currentTime = 0;
    }
  }

  async function handleApplyTrim() {
    if (trimEnd - trimStart < 1) {
      setError("Der verbleibende Bereich muss mindestens 1 Sekunde lang sein.");
      return;
    }

    const confirmed = window.confirm(
      "Das aktuelle Video wird durch die getrimmte Version ersetzt. Möchtest du fortfahren?"
    );

    if (!confirmed) return;

    setTrimming(true);
    setError(null);

    try {
      await apiFetch(`/api/videos/${videoId}/trim`, {
        method: "POST",
        body: JSON.stringify({
          startSeconds: trimStart,
          endSeconds: trimEnd,
        }),
      });

      onTrimStarted?.();
      onClose();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Trimmen fehlgeschlagen."
      );
      setTrimming(false);
    }
  }

  const timelineCurrentTime =
    sourceTimeToTimelineTime(currentTime);

  const playheadPct =
    timelineDuration > 0
      ? Math.max(
          0,
          Math.min(
            100,
            (timelineCurrentTime / timelineDuration) * 100,
          ),
        )
      : 0;

  const trimStartPct =
    duration > 0 ? (trimStart / duration) * 100 : 0;

  const trimEndPct =
    duration > 0 ? (trimEnd / duration) * 100 : 100;

  let timelineOffset = 0;

  const clipLayout = clips.map((clip) => {
    const clipDuration = Math.max(0, clip.end - clip.start);
    const timelineStart = timelineOffset;

    timelineOffset += clipDuration;

    return {
      clip,
      clipDuration,
      timelineStart,
    };
  });

  const hasDeletedTime =
    timelineDuration < duration - 0.001;

  return (
    <div
      style={{
        position: "fixed",
        inset: 0,
        background: "var(--color-overlay)",
        zIndex: 1000,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: 24,
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="video-editor-title"
        style={{
          width: "min(1000px, 96vw)",
          maxHeight: "94vh",
          overflowY: "auto",
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: 12,
          padding: 24,
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            marginBottom: 16,
          }}
        >
          <h2
            id="video-editor-title"
            style={{
              margin: 0,
              fontSize: 20,
              color: "var(--color-text)",
            }}
          >
            Video bearbeiten
          </h2>

          <button
            type="button"
            onClick={onClose}
            style={{
              background: "transparent",
              border: "1px solid var(--color-border)",
              borderRadius: 8,
              padding: "6px 12px",
              color: "var(--color-text-secondary)",
              cursor: "pointer",
            }}
          >
            Schließen
          </button>
        </div>

        {error && (
          <div
            style={{
              color: "var(--color-error)",
              marginBottom: 16,
            }}
          >
            {error}
          </div>
        )}

        {videoUrl && (
          <video
            ref={videoRef}
            src={videoUrl}
            controls
            onTimeUpdate={(e) =>
              setCurrentTime(e.currentTarget.currentTime)
            }
            style={{
              width: "100%",
              maxHeight: 480,
              background: "#000",
              borderRadius: 8,
              marginBottom: 24,
            }}
          />
        )}

        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 12,
            marginBottom: 16,
          }}
        >
          <button
            type="button"
            style={{
              border: "none",
              borderRadius: 8,
              padding: "8px 14px",
              background: "#0F172A",
              color: "#FFFFFF",
              fontWeight: 600,
              cursor: "default",
            }}
          >
            ✂ Trimmen
          </button>

          <button
            type="button"
            onClick={handleSplit}
            style={{
              border: "1px solid var(--color-border)",
              borderRadius: 8,
              padding: "8px 14px",
              background: "#0F172A",
              color: "#FFFFFF",
              fontWeight: 600,
              cursor: "pointer",
            }}
          >
            Teilen
          </button>

          {selectedClipId && (
            <button
              type="button"
              onClick={handleDeleteSelectedClip}
              disabled={clips.length <= 1}
              style={{
                border: "1px solid #B42318",
                borderRadius: 8,
                padding: "8px 14px",
                background: "#FFFFFF",
                color: "#B42318",
                fontWeight: 600,
                cursor:
                  clips.length <= 1 ? "default" : "pointer",
                opacity: clips.length <= 1 ? 0.5 : 1,
              }}
            >
              Clip löschen
            </button>
          )}

          <button
            type="button"
            onClick={handleOpenInsertPicker}
            style={{
              border: "1px solid var(--color-border)",
              borderRadius: 8,
              padding: "8px 14px",
              background: "#0F172A",
              color: "#FFFFFF",
              fontWeight: 600,
              cursor: "pointer",
            }}
          >
            + Video einfügen
          </button>

          <button
            type="button"
            onClick={handleUndo}
            disabled={clipHistory.length === 0}
            style={{
              border: "1px solid var(--color-border)",
              borderRadius: 8,
              padding: "8px 14px",
              background: "#FFFFFF",
              color: "#0F172A",
              fontWeight: 600,
              cursor:
                clipHistory.length === 0
                  ? "default"
                  : "pointer",
              opacity:
                clipHistory.length === 0 ? 0.45 : 1,
            }}
          >
            ↶ Rückgängig
          </button>

          <span
            style={{
              fontSize: 13,
              color: "var(--color-text-secondary)",
            }}
          >
            Abspielkopf setzen und mit „Teilen“ einen neuen Clip erzeugen
          </span>
        </div>

        {showInsertPicker && (
          <div
            style={{
              border: "1px solid var(--color-border)",
              borderRadius: 10,
              padding: 14,
              marginBottom: 18,
              background: "var(--color-surface)",
            }}
          >
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                marginBottom: 12,
              }}
            >
              <strong style={{ color: "var(--color-text)" }}>
                Video aus Bibliothek auswählen
              </strong>

              <button
                type="button"
                onClick={() => setShowInsertPicker(false)}
                style={{
                  border: "none",
                  background: "transparent",
                  color: "var(--color-text-secondary)",
                  cursor: "pointer",
                  fontSize: 18,
                }}
              >
                ×
              </button>
            </div>

            {loadingLibrary ? (
              <div style={{ color: "var(--color-text-secondary)" }}>
                Bibliothek wird geladen...
              </div>
            ) : libraryVideos.length === 0 ? (
              <div style={{ color: "var(--color-text-secondary)" }}>
                Keine weiteren fertigen Videos gefunden.
              </div>
            ) : (
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns:
                    "repeat(auto-fill, minmax(220px, 1fr))",
                  gap: 10,
                }}
              >
                {libraryVideos.map((video) => (
                  <button
                    key={video.id}
                    type="button"
                    onClick={() => {
                      setSelectedInsertVideo(video);
                      setShowInsertPicker(false);
                    }}
                    style={{
                      textAlign: "left",
                      border:
                        selectedInsertVideo?.id === video.id
                          ? "2px solid #E6467A"
                          : "1px solid var(--color-border)",
                      borderRadius: 8,
                      padding: 10,
                      background: "#FFFFFF",
                      cursor: "pointer",
                    }}
                  >
                    <div
                      style={{
                        fontWeight: 600,
                        color: "#0F172A",
                        marginBottom: 4,
                      }}
                    >
                      {video.title || "Unbenanntes Video"}
                    </div>

                    <div
                      style={{
                        fontSize: 12,
                        color: "var(--color-text-secondary)",
                      }}
                    >
                      {formatDuration(video.duration)}
                    </div>
                  </button>
                ))}
              </div>
            )}
          </div>
        )}

        {selectedInsertVideo && (
          <div
            style={{
              marginBottom: 14,
              padding: "8px 12px",
              borderRadius: 8,
              background: "#F8FAFC",
              color: "#0F172A",
              fontSize: 13,
            }}
          >
            Zum Einfügen ausgewählt:{" "}
            <strong>
              {selectedInsertVideo.title || "Unbenanntes Video"}
            </strong>{" "}
            ({formatDuration(selectedInsertVideo.duration)})
          </div>
        )}

        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            fontSize: 13,
            color: "var(--color-text-secondary)",
            marginBottom: 8,
          }}
        >
          <span>{formatDuration(timelineCurrentTime)}</span>
          <span>{formatDuration(timelineDuration)}</span>
        </div>

        <div
          ref={timelineRef}
          onClick={handleTimelineClick}
          style={{
            position: "relative",
            height: 64,
            borderRadius: 8,
            background: "var(--color-border)",
            cursor: "pointer",
            overflow: "hidden",
            userSelect: "none",
          }}
        >
          {clipLayout.map(
            ({ clip, clipDuration, timelineStart }, index) => {
            const left =
              timelineDuration > 0
                ? (timelineStart / timelineDuration) * 100
                : 0;

            const width =
              timelineDuration > 0
                ? (clipDuration / timelineDuration) * 100
                : 0;

            return (
              <div
                key={clip.id}
                onClick={(e) => {
                  e.stopPropagation();
                  setSelectedClipId(clip.id);
                }}
                style={{
                  position: "absolute",
                  top: 10,
                  bottom: 10,
                  left: `${left}%`,
                  width: `${width}%`,
                  background: "#1E293B",
                  border: "1px solid rgba(255,255,255,0.35)",
                  outline:
                    selectedClipId === clip.id
                      ? "3px solid #E6467A"
                      : "none",
                  outlineOffset: "-3px",
                  boxSizing: "border-box",
                  display: "flex",
                  alignItems: "center",
                  padding: "0 12px",
                  color: "#fff",
                  fontSize: 13,
                  fontWeight: 600,
                  overflow: "hidden",
                  whiteSpace: "nowrap",
                  cursor: "pointer",
                  zIndex: selectedClipId === clip.id ? 2 : 1,
                }}
              >
                Clip {index + 1}
              </div>
            );
          })}

          <div
            style={{
              position: "absolute",
              top: 0,
              bottom: 0,
              left: 0,
              width: `${trimStartPct}%`,
              background: "rgba(15, 23, 42, 0.6)",
              pointerEvents: "none",
            }}
          />

          <div
            style={{
              position: "absolute",
              top: 0,
              bottom: 0,
              left: `${trimEndPct}%`,
              right: 0,
              background: "rgba(15, 23, 42, 0.6)",
              pointerEvents: "none",
            }}
          />

          <div
            onMouseDown={handleTrimPointerDown("start")}
            onTouchStart={handleTrimPointerDown("start")}
            title="Trim-Anfang"
            style={{
              position: "absolute",
              top: 4,
              bottom: 4,
              left: `${trimStartPct}%`,
              width: 14,
              transform: "translateX(-7px)",
              background: "#fff",
              border: "2px solid #E6467A",
              borderRadius: 5,
              cursor: "ew-resize",
              zIndex: 4,
              touchAction: "none",
            }}
          />

          <div
            onMouseDown={handleTrimPointerDown("end")}
            onTouchStart={handleTrimPointerDown("end")}
            title="Trim-Ende"
            style={{
              position: "absolute",
              top: 4,
              bottom: 4,
              left: `${trimEndPct}%`,
              width: 14,
              transform: "translateX(-7px)",
              background: "#fff",
              border: "2px solid #E6467A",
              borderRadius: 5,
              cursor: "ew-resize",
              zIndex: 4,
              touchAction: "none",
            }}
          />

          <div
            style={{
              position: "absolute",
              top: 0,
              bottom: 0,
              left: `${playheadPct}%`,
              width: 2,
              background: "#E6467A",
              transform: "translateX(-1px)",
              pointerEvents: "none",
            }}
          />

          <div
            style={{
              position: "absolute",
              top: 0,
              left: `${playheadPct}%`,
              width: 10,
              height: 10,
              borderRadius: "50%",
              background: "#E6467A",
              transform: "translate(-5px, -2px)",
              pointerEvents: "none",
            }}
          />
        </div>

        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            gap: 16,
            marginTop: 10,
            fontSize: 12,
            color: "var(--color-text-secondary)",
          }}
        >
          <span>Anfang: {formatDuration(trimStart)}</span>
          <span>
            Auswahl: {formatDuration(Math.max(0, trimEnd - trimStart))}
          </span>
          <span>Ende: {formatDuration(trimEnd)}</span>
        </div>

        <div
          style={{
            display: "flex",
            justifyContent: "flex-end",
            gap: 10,
            marginTop: 18,
          }}
        >
          <button
            type="button"
            onClick={handleResetTrim}
            disabled={trimming}
            style={{
              background: "transparent",
              color: "var(--color-text-secondary)",
              border: "1px solid var(--color-border)",
              borderRadius: 8,
              padding: "9px 16px",
              fontWeight: 600,
              cursor: trimming ? "default" : "pointer",
            }}
          >
            Zurücksetzen
          </button>

          <button
            type="button"
            onClick={handleApplyTrim}
            disabled={
              trimming ||
              hasDeletedTime ||
              (trimStart <= 0.001 && trimEnd >= duration - 0.001)
            }
            style={{
              background: "#0F172A",
              color: "#FFFFFF",
              border: "none",
              borderRadius: 8,
              padding: "9px 18px",
              fontWeight: 600,
              cursor: trimming ? "default" : "pointer",
              opacity:
                trimming ||
                hasDeletedTime ||
                (trimStart <= 0.001 && trimEnd >= duration - 0.001)
                  ? 0.6
                  : 1,
            }}
          >
            {trimming ? "Wird getrimmt..." : "Trimmen anwenden"}
          </button>
        </div>
      </div>
    </div>
  );
}
