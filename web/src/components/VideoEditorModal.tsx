import { useEffect, useRef, useState } from "react";
import { apiFetch } from "../api/client";
import { formatDuration } from "../utils/format";
import type { Video } from "../types/video";

interface EditorClip {
  id: string;
  sourceVideoId: string;
  sourceTitle?: string;
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
  const [timelinePlayheadTime, setTimelinePlayheadTime] = useState(0);
  const [timelineZoom, setTimelineZoom] = useState(1);
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
  const videoUrlsRef = useRef<Record<string, string>>({});
  const activeSourceVideoIdRef = useRef(videoId);
  const activeClipIdRef = useRef("clip-1");
  const sourceSwitchGenerationRef = useRef(0);
  const sourceTransitionPendingRef = useRef(false);
  const cancelPendingSourceLoadRef = useRef<(() => void) | null>(null);

  async function loadVideoUrl(sourceVideoId: string) {
    if (videoUrlsRef.current[sourceVideoId]) {
      return videoUrlsRef.current[sourceVideoId];
    }

    const res = await apiFetch<{ downloadUrl: string }>(`/api/videos/${sourceVideoId}/download`);
    if (!res?.downloadUrl) {
      throw new Error("Video konnte nicht geladen werden.");
    }

    videoUrlsRef.current[sourceVideoId] = res.downloadUrl;
    return res.downloadUrl;
  }

  async function switchPreviewSource(
    sourceVideoId: string,
    sourceTime: number,
    clipId?: string,
    resumePlayback?: boolean,
  ) {
    const video = videoRef.current;
    if (!video) return;

    const generation = ++sourceSwitchGenerationRef.current;
    cancelPendingSourceLoadRef.current?.();
    cancelPendingSourceLoadRef.current = null;
    const shouldResume = resumePlayback ?? !video.paused;

    try {
      const url = await loadVideoUrl(sourceVideoId);
      if (generation !== sourceSwitchGenerationRef.current) return;

      if (clipId) activeClipIdRef.current = clipId;

      if (activeSourceVideoIdRef.current === sourceVideoId && video.src === url) {
        video.currentTime = sourceTime;
        if (shouldResume) await video.play();
        sourceTransitionPendingRef.current = false;
        return;
      }

      video.pause();
      activeSourceVideoIdRef.current = sourceVideoId;
      setVideoUrl(url);

      await new Promise<void>((resolve, reject) => {
        let settled = false;
        const handleLoadedMetadata = () => {
          settled = true;
          cleanup();
          if (generation !== sourceSwitchGenerationRef.current) {
            resolve();
            return;
          }
          video.currentTime = Math.max(0, Math.min(sourceTime, video.duration || sourceTime));
          resolve();
        };
        const handleError = () => {
          settled = true;
          cleanup();
          reject(new Error("Video konnte nicht geladen werden."));
        };
        const cleanup = () => {
          video.removeEventListener("loadedmetadata", handleLoadedMetadata);
          video.removeEventListener("error", handleError);
          if (cancelPendingSourceLoadRef.current === cancel) {
            cancelPendingSourceLoadRef.current = null;
          }
        };
        const cancel = () => {
          if (settled) return;
          settled = true;
          cleanup();
          reject(new Error("Quellenwechsel wurde ersetzt."));
        };

        cancelPendingSourceLoadRef.current = cancel;

        video.addEventListener("loadedmetadata", handleLoadedMetadata);
        video.addEventListener("error", handleError);
        video.src = url;
        video.load();
      });

      if (generation !== sourceSwitchGenerationRef.current) return;
      if (shouldResume) await video.play();
      sourceTransitionPendingRef.current = false;
      setError(null);
    } catch (err) {
      if (generation !== sourceSwitchGenerationRef.current) return;
      sourceTransitionPendingRef.current = false;
      setError(
        err instanceof Error ? err.message : "Video konnte nicht geladen werden.",
      );
    }
  }



  useEffect(() => {
    let cancelled = false;
    apiFetch<{ downloadUrl: string }>(`/api/videos/${videoId}/download`)
      .then((res) => {
        if (cancelled) return;
        if (res?.downloadUrl) {
          setVideoUrl(res.downloadUrl);
          videoUrlsRef.current[videoId] = res.downloadUrl;
        } else {
          setError("Video konnte nicht geladen werden.");
        }
      })
      .catch(() => {
        if (!cancelled) setError("Video konnte nicht geladen werden.");
      });
    return () => {
      cancelled = true;
    };
  }, [videoId]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video || !videoUrl || video.src === videoUrl) return;
    video.src = videoUrl;
    video.load();
  }, [videoUrl]);

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
    setTimelinePlayheadTime(0);
    activeSourceVideoIdRef.current = videoId;
    activeClipIdRef.current = "clip-1";
    sourceSwitchGenerationRef.current += 1;
    sourceTransitionPendingRef.current = false;
  }, [duration, videoId]);

  useEffect(() => () => {
    sourceSwitchGenerationRef.current += 1;
    cancelPendingSourceLoadRef.current?.();
  }, []);

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

  const timelineTickSteps = [0.25, 0.5, 1, 2, 5, 10, 15, 30, 60, 120, 300, 600];
  const visibleTimelineDuration =
    timelineDuration / Math.max(1, timelineZoom);
  const desiredTimelineTickStep = visibleTimelineDuration / 10;
  const timelineTickStep =
    timelineTickSteps.find((step) => step >= desiredTimelineTickStep) ?? 600;

  function formatTimelineTick(seconds: number) {
    const minutes = Math.floor(seconds / 60);
    const secondsInMinute = seconds % 60;

    if (timelineTickStep >= 1) {
      return `${minutes}:${String(Math.floor(secondsInMinute)).padStart(2, "0")}`;
    }

    const decimals = timelineTickStep <= 0.25 ? 2 : 1;
    const formattedSeconds = secondsInMinute.toFixed(decimals);
    return `${minutes}:${formattedSeconds.padStart(3 + decimals, "0")}`;
  }


  function timelineTimeToClipPosition(timelineTime: number) {
    if (clips.length === 0) return null;

    const clampedTime = Math.max(
      0,
      Math.min(timelineTime, timelineDuration),
    );

    let offset = 0;

    for (let index = 0; index < clips.length; index += 1) {
      const clip = clips[index];
      const clipDuration = clip.end - clip.start;
      const clipTimelineEnd = offset + clipDuration;

      if (
        clampedTime <= clipTimelineEnd ||
        index === clips.length - 1
      ) {
        const insideClip = Math.max(
          0,
          Math.min(clampedTime - offset, clipDuration),
        );

        return {
          clip,
          index,
          timelineStart: offset,
          sourceTime: clip.start + insideClip,
        };
      }

      offset = clipTimelineEnd;
    }

    return null;
  }

  function timelineStartForClip(clipId: string) {
    let offset = 0;
    for (const clip of clips) {
      if (clip.id === clipId) return offset;
      offset += clip.end - clip.start;
    }
    return null;
  }

  function sourceClipAtTime(sourceVideoId: string, sourceTime: number) {
    return clips.find(
      (clip) =>
        clip.sourceVideoId === sourceVideoId &&
        sourceTime >= clip.start - 0.001 &&
        sourceTime <= clip.end + 0.001,
    );
  }

  function advancePreviewToNextClip() {
    if (sourceTransitionPendingRef.current) return;
    const currentIndex = clips.findIndex((clip) => clip.id === activeClipIdRef.current);
    if (currentIndex < 0) return;
    if (currentIndex >= clips.length - 1) {
      videoRef.current?.pause();
      setTimelinePlayheadTime(timelineDuration);
      return;
    }

    const nextClip = clips[currentIndex + 1];
    const nextTimelineStart = timelineStartForClip(nextClip.id);
    if (nextTimelineStart === null) return;

    sourceTransitionPendingRef.current = true;
    setTimelinePlayheadTime(nextTimelineStart);
    setCurrentTime(nextClip.start);
    setSelectedClipId(nextClip.id);
    void switchPreviewSource(
      nextClip.sourceVideoId,
      nextClip.start,
      nextClip.id,
      true,
    );
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

        if (videoRef.current && activeSourceVideoIdRef.current === videoId) {
          videoRef.current.currentTime = nextTime;
        } else {
          void switchPreviewSource(
            videoId,
            nextTime,
            sourceClipAtTime(videoId, nextTime)?.id,
          );
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

    const position =
      timelineTimeToClipPosition(timelineTime);

    if (!position) return;

    setTimelinePlayheadTime(timelineTime);
    setSelectedClipId(position.clip.id);
    setError(null);
    setCurrentTime(position.sourceTime);
    void switchPreviewSource(
      position.clip.sourceVideoId,
      position.sourceTime,
      position.clip.id,
    );
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

  function handleInsertSelectedVideo() {
    if (!selectedInsertVideo) {
      setError("Bitte zuerst ein Video auswählen.");
      return;
    }

    const insertAt = Math.max(
      0,
      Math.min(timelinePlayheadTime, timelineDuration),
    );

    const position =
      timelineTimeToClipPosition(insertAt);

    const insertedId =
      `clip-${nextClipIdRef.current++}`;

    const insertedClip: EditorClip = {
      id: insertedId,
      sourceVideoId: selectedInsertVideo.id,
      sourceTitle:
        selectedInsertVideo.title || "Unbenanntes Video",
      start: 0,
      end: selectedInsertVideo.duration,
    };

    rememberClipState();

    setClips((previousClips) => {
      if (!position) {
        return [...previousClips, insertedClip];
      }

      const { clip, index, sourceTime, timelineStart } =
        position;

      const clipDuration = clip.end - clip.start;
      const distanceFromStart =
        insertAt - timelineStart;
      const distanceFromEnd =
        clipDuration - distanceFromStart;

      // Genau am Anfang eines Clips
      if (distanceFromStart <= 0.001) {
        return [
          ...previousClips.slice(0, index),
          insertedClip,
          ...previousClips.slice(index),
        ];
      }

      // Genau am Ende eines Clips
      if (distanceFromEnd <= 0.001) {
        return [
          ...previousClips.slice(0, index + 1),
          insertedClip,
          ...previousClips.slice(index + 1),
        ];
      }

      // Mitten im Clip: automatisch teilen
      const leftClip: EditorClip = {
        ...clip,
        id: `clip-${nextClipIdRef.current++}`,
        end: sourceTime,
      };

      const rightClip: EditorClip = {
        ...clip,
        id: `clip-${nextClipIdRef.current++}`,
        start: sourceTime,
      };

      return [
        ...previousClips.slice(0, index),
        leftClip,
        insertedClip,
        rightClip,
        ...previousClips.slice(index + 1),
      ];
    });

    setTimelinePlayheadTime(
      insertAt + selectedInsertVideo.duration,
    );
    setSelectedClipId(insertedId);
    void switchPreviewSource(
      insertedClip.sourceVideoId,
      insertedClip.end,
      insertedId,
      false,
    );
    setSelectedInsertVideo(null);
    setError(null);
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
    const firstClip = previousClips[0];
    if (firstClip) {
      setTimelinePlayheadTime(0);
      setCurrentTime(firstClip.start);
      void switchPreviewSource(
        firstClip.sourceVideoId,
        firstClip.start,
        firstClip.id,
        false,
      );
    }
    setError(null);
  }

  function handleSplit() {
    const minimumDistance = 0.1;

    const position =
      timelineTimeToClipPosition(timelinePlayheadTime);

    if (!position) {
      setError("Zum Teilen muss der Abspielkopf innerhalb eines Clips stehen.");
      return;
    }

    const { clip, index, sourceTime } = position;

    if (
      sourceTime <= clip.start + minimumDistance ||
      sourceTime >= clip.end - minimumDistance
    ) {
      setError(
        "Zum Teilen muss der Abspielkopf innerhalb eines Clips stehen.",
      );
      return;
    }

    rememberClipState();

    const leftClip: EditorClip = {
      ...clip,
      id: `clip-${nextClipIdRef.current++}`,
      end: sourceTime,
    };

    const rightClip: EditorClip = {
      ...clip,
      id: `clip-${nextClipIdRef.current++}`,
      start: sourceTime,
    };

    setClips((previousClips) => [
      ...previousClips.slice(0, index),
      leftClip,
      rightClip,
      ...previousClips.slice(index + 1),
    ]);

    if (activeClipIdRef.current === clip.id) {
      activeClipIdRef.current = rightClip.id;
    }

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

    if (activeClipIdRef.current === selectedClipId) {
      const remainingClips = clips.filter((clip) => clip.id !== selectedClipId);
      const nextClip = remainingClips[0];
      if (nextClip) {
        setTimelinePlayheadTime(0);
        setCurrentTime(nextClip.start);
        void switchPreviewSource(
          nextClip.sourceVideoId,
          nextClip.start,
          nextClip.id,
          false,
        );
      }
    }

    setSelectedClipId(null);
    setError(null);
  }

  function handleResetTrim() {
    setTrimStart(0);
    setTrimEnd(duration);
    setCurrentTime(0);
    setTimelinePlayheadTime(0);

    void switchPreviewSource(
      videoId,
      0,
      sourceClipAtTime(videoId, 0)?.id,
      false,
    );
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
    timelinePlayheadTime;

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
    timelineDuration < duration - 0.001 ||
    clips.some((clip) => clip.sourceVideoId !== videoId);

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
        width: "calc(100vw - 48px)",
        height: "calc(100vh - 48px)",
        minWidth: 720,
        minHeight: 520,
        maxWidth: "calc(100vw - 48px)",
        maxHeight: "calc(100vh - 48px)",
        resize: "both",
        boxSizing: "border-box",
        position: "relative",
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
            controls
            onTimeUpdate={(e) => {
              const sourceTime = e.currentTarget.currentTime;
              const activeClip = clips.find(
                (clip) => clip.id === activeClipIdRef.current,
              );

              if (!activeClip) return;
              if (activeClip.sourceVideoId !== activeSourceVideoIdRef.current) return;

              const clipTimelineStart = timelineStartForClip(activeClip.id);
              if (clipTimelineStart === null) return;

              setCurrentTime(sourceTime);
              setTimelinePlayheadTime(
                Math.max(
                  clipTimelineStart,
                  Math.min(
                    clipTimelineStart + (activeClip.end - activeClip.start),
                    clipTimelineStart + (sourceTime - activeClip.start),
                  ),
                ),
              );

              if (
                !e.currentTarget.paused &&
                sourceTime >= activeClip.end - 0.05
              ) {
                advancePreviewToNextClip();
              }
            }}
            onEnded={advancePreviewToNextClip}
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
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 12,
            }}
          >
            <span>
              Zum Einfügen ausgewählt:{" "}
              <strong>
                {selectedInsertVideo.title || "Unbenanntes Video"}
              </strong>{" "}
              ({formatDuration(selectedInsertVideo.duration)})
            </span>

            <button
              type="button"
              onClick={handleInsertSelectedVideo}
              style={{
                border: "none",
                borderRadius: 8,
                padding: "8px 14px",
                background: "#0F172A",
                color: "#FFFFFF",
                fontWeight: 600,
                cursor: "pointer",
                whiteSpace: "nowrap",
              }}
            >
              Hier einfügen
            </button>
          </div>
        )}

        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 10,
            marginBottom: 12,
          }}
        >
          <button
            type="button"
            onClick={() => setTimelineZoom(1)}
            style={{
              border: "1px solid var(--color-border)",
              borderRadius: 7,
              padding: "5px 10px",
              background: "#FFFFFF",
              color: "#0F172A",
              fontWeight: 600,
              cursor: "pointer",
            }}
          >
            Fit
          </button>

          <button
            type="button"
            onClick={() =>
              setTimelineZoom((zoom) => Math.max(1, zoom - 0.5))
            }
            disabled={timelineZoom <= 1}
          >
            −
          </button>

          <input
            type="range"
            min="1"
            max="12"
            step="0.5"
            value={timelineZoom}
            onChange={(e) =>
              setTimelineZoom(Number(e.currentTarget.value))
            }
            aria-label="Timeline-Zoom"
            style={{ width: 190 }}
          />

          <button
            type="button"
            onClick={() =>
              setTimelineZoom((zoom) => Math.min(12, zoom + 0.5))
            }
            disabled={timelineZoom >= 12}
          >
            +
          </button>

          <span
            style={{
              fontSize: 12,
              color: "var(--color-text-secondary)",
            }}
          >
            {timelineZoom.toFixed(1)}×
          </span>
        </div>

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
          style={{
            overflowX: "auto",
            overflowY: "hidden",
            paddingBottom: 6,
          }}
        >
        <div
          style={{
            position: "relative",
            height: 28,
            width: `${timelineZoom * 100}%`,
            minWidth: "100%",
            borderBottom: "1px solid var(--color-border)",
            marginBottom: 4,
          }}
        >
          {Array.from(
            { length: Math.floor(timelineDuration / timelineTickStep) + 1 },
            (_, index) => {
              const tickTime = index * timelineTickStep;
              const left =
                timelineDuration > 0 ? (tickTime / timelineDuration) * 100 : 0;

              return (
                <div
                  key={`timeline-tick-${index}`}
                  style={{
                    position: "absolute",
                    left: `${left}%`,
                    top: 0,
                    pointerEvents: "none",
                  }}
                >
                  <div
                    style={{
                      width: 1,
                      height: 8,
                      background: "var(--color-text-secondary)",
                      opacity: 0.6,
                    }}
                  />
                  <span
                    style={{
                      position: "absolute",
                      top: 9,
                      left: index === 0 ? 0 : "50%",
                      transform: index === 0 ? "none" : "translateX(-50%)",
                      fontSize: 10,
                      color: "var(--color-text-secondary)",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {formatTimelineTick(tickTime)}
                  </span>
                </div>
              );
            },
          )}
        </div>
          <div
            ref={timelineRef}
            data-testid="video-editor-timeline"
            onClick={handleTimelineClick}
            style={{
              position: "relative",
              height: 64,
              width: `${timelineZoom * 100}%`,
              minWidth: "100%",
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
                data-testid={`video-editor-clip-${clip.id}`}
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
                {clip.sourceVideoId === videoId
                  ? `Clip ${index + 1}`
                  : clip.sourceTitle
                    ? `Eingefügt: ${clip.sourceTitle}`
                    : "Eingefügtes Video"}
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
            justifyContent: "center",
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
      <div
        style={{
          position: "absolute",
          right: 18,
          bottom: 8,
          fontSize: 11,
          color: "var(--color-text-secondary)",
          pointerEvents: "none",
          userSelect: "none",
        }}
      >
        ↘ Größe ändern
      </div>
      </div>
    </div>
  );
}
