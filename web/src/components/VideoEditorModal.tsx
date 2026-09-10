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

interface EditorCoverOverlay {
  id: string;
  x: number;
  y: number;
  width: number;
  height: number;
  start: number;
  end: number;
}

interface StoredEditorState {
  timeline: {
    version: number;
    clips: Array<{
      id: string;
      sourceId: string;
      sourceStart: number;
      sourceEnd: number;
      duration: number;
    }>;
  };
  renderStatus: "none" | "processing" | "ready" | "failed";
  renderError: string | null;
  renderedVideoId: string | null;
}

interface VideoEditorModalProps {
  videoId: string;
  duration: number;
  onClose: () => void;
  onTrimStarted?: () => void;
}

const TIMELINE_ZOOM_LEVELS = [1, 2, 5, 10] as const;
const TIMELINE_TICK_STEPS = [0.1, 0.5, 1, 2, 5, 10, 15, 30, 60, 120, 300, 600];

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
  const [rendering, setRendering] = useState(false);
  const [renderStatus, setRenderStatus] = useState<StoredEditorState["renderStatus"]>("none");
  const [renderedVideoId, setRenderedVideoId] = useState<string | null>(null);
  const [selectedClipId, setSelectedClipId] = useState<string | null>(null);
  const [clipHistory, setClipHistory] = useState<EditorClip[][]>([]);
  const [coverOverlays, setCoverOverlays] = useState<EditorCoverOverlay[]>([]);
  const [selectedCoverOverlayId, setSelectedCoverOverlayId] = useState<string | null>(null);
  const [copiedCoverOverlay, setCopiedCoverOverlay] = useState<EditorCoverOverlay | null>(null);
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
  const pendingRestoredPreviewRef = useRef<EditorClip | null>(null);

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
    const restoredClip = pendingRestoredPreviewRef.current;
    if (restoredClip) {
      pendingRestoredPreviewRef.current = null;
      void switchPreviewSource(
        restoredClip.sourceVideoId,
        restoredClip.start,
        restoredClip.id,
        false,
      );
    }
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

  useEffect(() => {
    let cancelled = false;

    async function loadEditorState() {
      try {
        const state = await apiFetch<StoredEditorState>(`/api/videos/${videoId}/editor`);
        if (cancelled || !state) return;

        setRenderStatus(state.renderStatus);
        setRendering(state.renderStatus === "processing");
        setRenderedVideoId(state.renderedVideoId);
        if (state.renderStatus === "failed" && state.renderError) {
          setError(state.renderError);
        }

        if (state.timeline?.version === 1 && state.timeline.clips.length > 0) {
          const restoredClips = state.timeline.clips.map((clip) => ({
            id: clip.id,
            sourceVideoId: clip.sourceId,
            start: clip.sourceStart,
            end: clip.sourceEnd,
          }));
          setClips(restoredClips);
          activeClipIdRef.current = restoredClips[0].id;
          activeSourceVideoIdRef.current = restoredClips[0].sourceVideoId;
          const maxClipNumber = restoredClips.reduce((max, clip) => {
            const match = /^clip-(\d+)$/.exec(clip.id);
            return match ? Math.max(max, Number(match[1])) : max;
          }, 0);
          nextClipIdRef.current = maxClipNumber + 1;
          setTimelinePlayheadTime(0);
          setCurrentTime(restoredClips[0].start);
          if (videoRef.current) {
            void switchPreviewSource(
              restoredClips[0].sourceVideoId,
              restoredClips[0].start,
              restoredClips[0].id,
              false,
            );
          } else {
            pendingRestoredPreviewRef.current = restoredClips[0];
          }
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Editorstand konnte nicht geladen werden.");
        }
      }
    }

    void loadEditorState();
    return () => {
      cancelled = true;
    };
  }, [videoId]);

  useEffect(() => {
    if (!rendering) return;
    const interval = window.setInterval(async () => {
      try {
        const state = await apiFetch<StoredEditorState>(`/api/videos/${videoId}/editor`);
        if (!state) return;
        setRenderStatus(state.renderStatus);
        setRenderedVideoId(state.renderedVideoId);
        if (state.renderStatus !== "processing") {
          setRendering(false);
          if (state.renderStatus === "failed") {
            setError(state.renderError || "Rendern fehlgeschlagen.");
          }
        }
      } catch (err) {
        setRendering(false);
        setError(err instanceof Error ? err.message : "Renderstatus konnte nicht geladen werden.");
      }
    }, 2000);
    return () => window.clearInterval(interval);
  }, [rendering, videoId]);

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

  const selectedCoverOverlay =
    coverOverlays.find((overlay) => overlay.id === selectedCoverOverlayId) ?? null;

  const visibleTimelineDuration =
    timelineDuration / Math.max(1, timelineZoom);
  const desiredTimelineTickStep = visibleTimelineDuration / 10;
  const timelineTickStep =
    TIMELINE_TICK_STEPS.find((step) => step >= desiredTimelineTickStep) ?? 600;

  function formatTimelineTick(seconds: number) {
    const minutes = Math.floor(seconds / 60);
    const secondsInMinute = seconds % 60;

    if (timelineTickStep >= 1) {
      return `${minutes}:${String(Math.floor(secondsInMinute)).padStart(2, "0")}`;
    }

    const decimals = 1;
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

  useEffect(() => {
    function handleTimelineArrowKeys(e: KeyboardEvent) {
      const target = e.target as HTMLElement | null;

      if (
        target?.tagName === "INPUT" ||
        target?.tagName === "TEXTAREA" ||
        target?.tagName === "SELECT" ||
        target?.isContentEditable
      ) {
        return;
      }

      if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
      if (!timelineDuration) return;

      e.preventDefault();

      const delta = e.key === "ArrowLeft" ? -0.1 : 0.1;
      const nextTimelineTime = Math.max(
        0,
        Math.min(timelinePlayheadTime + delta, timelineDuration),
      );

      const position = timelineTimeToClipPosition(nextTimelineTime);
      if (!position) return;

      videoRef.current?.pause();
      setTimelinePlayheadTime(nextTimelineTime);
      setSelectedClipId(position.clip.id);
      setError(null);
      setCurrentTime(position.sourceTime);

      void switchPreviewSource(
        position.clip.sourceVideoId,
        position.sourceTime,
        position.clip.id,
      );
    }

    document.addEventListener("keydown", handleTimelineArrowKeys);
    return () =>
      document.removeEventListener("keydown", handleTimelineArrowKeys);
  }, [
    timelinePlayheadTime,
    timelineDuration,
    clips,
    timelineTimeToClipPosition,
    switchPreviewSource,
  ]);

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

  function handleCoverOverlayPointerDown(
    e: React.PointerEvent<HTMLDivElement>,
    overlayId: string,
  ) {
    e.preventDefault();
    e.stopPropagation();
    setSelectedCoverOverlayId(overlayId);

    const overlay = coverOverlays.find((item) => item.id === overlayId);
    const container = e.currentTarget.parentElement;

    if (!overlay || !container) return;

    const rect = container.getBoundingClientRect();
    const startClientX = e.clientX;
    const startClientY = e.clientY;
    const startX = overlay.x;
    const startY = overlay.y;
    const overlayWidth = overlay.width;
    const overlayHeight = overlay.height;

    function onMove(ev: PointerEvent) {
      const deltaX = ((ev.clientX - startClientX) / rect.width) * 100;
      const deltaY = ((ev.clientY - startClientY) / rect.height) * 100;

      const nextX = Math.max(
        0,
        Math.min(100 - overlayWidth, startX + deltaX),
      );
      const nextY = Math.max(
        0,
        Math.min(100 - overlayHeight, startY + deltaY),
      );

      setCoverOverlays((previous) =>
        previous.map((item) =>
          item.id === overlayId
            ? { ...item, x: nextX, y: nextY }
            : item,
        ),
      );
    }

    function onUp() {
      document.removeEventListener("pointermove", onMove);
      document.removeEventListener("pointerup", onUp);
    }

    document.addEventListener("pointermove", onMove);
    document.addEventListener("pointerup", onUp);
  }

  function handleCoverOverlayResizePointerDown(
    e: React.PointerEvent<HTMLDivElement>,
    overlayId: string,
  ) {
    e.preventDefault();
    e.stopPropagation();

    const overlay = coverOverlays.find((item) => item.id === overlayId);
    const container = e.currentTarget.parentElement?.parentElement;

    if (!overlay || !container) return;

    const rect = container.getBoundingClientRect();
    const startClientX = e.clientX;
    const startClientY = e.clientY;
    const startWidth = overlay.width;
    const startHeight = overlay.height;
    const overlayX = overlay.x;
    const overlayY = overlay.y;

    function onMove(ev: PointerEvent) {
      const deltaX = ((ev.clientX - startClientX) / rect.width) * 100;
      const deltaY = ((ev.clientY - startClientY) / rect.height) * 100;

      const nextWidth = Math.max(
        5,
        Math.min(100 - overlayX, startWidth + deltaX),
      );
      const nextHeight = Math.max(
        5,
        Math.min(100 - overlayY, startHeight + deltaY),
      );

      setCoverOverlays((previous) =>
        previous.map((item) =>
          item.id === overlayId
            ? { ...item, width: nextWidth, height: nextHeight }
            : item,
        ),
      );
    }

    function onUp() {
      document.removeEventListener("pointermove", onMove);
      document.removeEventListener("pointerup", onUp);
    }

    document.addEventListener("pointermove", onMove);
    document.addEventListener("pointerup", onUp);
  }

  function handleCoverOverlayTimelineMovePointerDown(
    e: React.PointerEvent<HTMLDivElement>,
    overlayId: string,
  ) {
    e.preventDefault();
    e.stopPropagation();
    setSelectedCoverOverlayId(overlayId);

    const overlay = coverOverlays.find((item) => item.id === overlayId);
    const track = e.currentTarget.parentElement;

    if (!overlay || !track || timelineDuration <= 0) return;

    const rect = track.getBoundingClientRect();
    const startClientX = e.clientX;
    const originalStart = overlay.start;
    const overlayDuration = overlay.end - overlay.start;

    function onMove(ev: PointerEvent) {
      const deltaTime =
        ((ev.clientX - startClientX) / rect.width) * timelineDuration;

      const start = Math.max(
        0,
        Math.min(
          timelineDuration - overlayDuration,
          originalStart + deltaTime,
        ),
      );

      const end = start + overlayDuration;

      setCoverOverlays((previous) =>
        previous.map((item) =>
          item.id === overlayId
            ? { ...item, start, end }
            : item,
        ),
      );
    }

    function onUp() {
      document.removeEventListener("pointermove", onMove);
      document.removeEventListener("pointerup", onUp);
    }

    document.addEventListener("pointermove", onMove);
    document.addEventListener("pointerup", onUp);
  }

  function handleCoverOverlayTimelineStartPointerDown(
    e: React.PointerEvent<HTMLDivElement>,
    overlayId: string,
  ) {
    e.preventDefault();
    e.stopPropagation();
    setSelectedCoverOverlayId(overlayId);

    const overlay = coverOverlays.find((item) => item.id === overlayId);
    const track = e.currentTarget.parentElement?.parentElement;

    if (!overlay || !track || timelineDuration <= 0) return;

    const rect = track.getBoundingClientRect();
    const minimumGap = 0.1;
    const overlayEnd = overlay.end;

    function onMove(ev: PointerEvent) {
      const x = Math.max(
        0,
        Math.min(ev.clientX - rect.left, rect.width),
      );

      const rawTime = (x / rect.width) * timelineDuration;
      const start = Math.max(
        0,
        Math.min(rawTime, overlayEnd - minimumGap),
      );

      setCoverOverlays((previous) =>
        previous.map((item) =>
          item.id === overlayId
            ? { ...item, start }
            : item,
        ),
      );
    }

    function onUp() {
      document.removeEventListener("pointermove", onMove);
      document.removeEventListener("pointerup", onUp);
    }

    document.addEventListener("pointermove", onMove);
    document.addEventListener("pointerup", onUp);
  }

  function handleCoverOverlayTimelineEndPointerDown(
    e: React.PointerEvent<HTMLDivElement>,
    overlayId: string,
  ) {
    e.preventDefault();
    e.stopPropagation();
    setSelectedCoverOverlayId(overlayId);

    const overlay = coverOverlays.find((item) => item.id === overlayId);
    const track = e.currentTarget.parentElement?.parentElement;

    if (!overlay || !track || timelineDuration <= 0) return;

    const rect = track.getBoundingClientRect();
    const minimumGap = 0.1;
    const overlayStart = overlay.start;

    function onMove(ev: PointerEvent) {
      const x = Math.max(
        0,
        Math.min(ev.clientX - rect.left, rect.width),
      );

      const rawTime = (x / rect.width) * timelineDuration;
      const end = Math.min(
        timelineDuration,
        Math.max(rawTime, overlayStart + minimumGap),
      );

      setCoverOverlays((previous) =>
        previous.map((item) =>
          item.id === overlayId
            ? { ...item, end }
            : item,
        ),
      );
    }

    function onUp() {
      document.removeEventListener("pointermove", onMove);
      document.removeEventListener("pointerup", onUp);
    }

    document.addEventListener("pointermove", onMove);
    document.addEventListener("pointerup", onUp);
  }

  function handleCopyCoverOverlay() {
    if (!selectedCoverOverlay) {
      setError("Bitte zuerst eine Abdeckung auswählen.");
      return;
    }

    setCopiedCoverOverlay({ ...selectedCoverOverlay });
    setError(null);
  }

  function handlePasteCoverOverlay() {
    if (!copiedCoverOverlay || timelineDuration <= 0) {
      setError("Es ist keine Abdeckung zum Einfügen kopiert.");
      return;
    }

    const duration = copiedCoverOverlay.end - copiedCoverOverlay.start;
    const start = Math.min(
      timelinePlayheadTime,
      Math.max(0, timelineDuration - 0.1),
    );
    const end = Math.min(timelineDuration, start + duration);

    const pastedOverlay: EditorCoverOverlay = {
      ...copiedCoverOverlay,
      id: `cover-${Date.now()}`,
      start,
      end,
    };

    setCoverOverlays((previous) => [...previous, pastedOverlay]);
    setSelectedCoverOverlayId(pastedOverlay.id);
    setError(null);
  }

  function handleAddCoverOverlay() {
    if (timelineDuration <= 0) return;

    const start = Math.min(
      timelinePlayheadTime,
      Math.max(0, timelineDuration - 0.1),
    );
    const end = Math.min(timelineDuration, start + 5);

    const overlay: EditorCoverOverlay = {
      id: `cover-${Date.now()}`,
      x: 30,
      y: 30,
      width: 40,
      height: 20,
      start,
      end,
    };

    setCoverOverlays((previous) => [...previous, overlay]);
    setSelectedCoverOverlayId(overlay.id);
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

  async function handleRenderTimeline() {
    if (clips.length === 0 || timelineDuration < 1) {
      setError("Die Timeline muss mindestens eine Sekunde lang sein.");
      return;
    }

    setRendering(true);
    setRenderStatus("processing");
    setRenderedVideoId(null);
    setError(null);
    try {
      await apiFetch(`/api/videos/${videoId}/editor/render`, {
        method: "POST",
        body: JSON.stringify({
          version: 1,
          clips: clips.map((clip) => ({
            id: clip.id,
            sourceId: clip.sourceVideoId,
            sourceStart: clip.start,
            sourceEnd: clip.end,
            duration: clip.end - clip.start,
          })),
          overlays: coverOverlays.map((overlay) => ({
            id: overlay.id,
            x: overlay.x,
            y: overlay.y,
            width: overlay.width,
            height: overlay.height,
            start: overlay.start,
            end: overlay.end,
          })),
        }),
      });
    } catch (err) {
      setRendering(false);
      setRenderStatus("failed");
      setError(err instanceof Error ? err.message : "Rendern konnte nicht gestartet werden.");
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
        padding: 16,
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
        width: "calc(100vw - 32px)",
        height: "calc(100vh - 32px)",
        minWidth: 720,
        minHeight: 520,
        maxWidth: "calc(100vw - 32px)",
        maxHeight: "calc(100vh - 32px)",
        resize: "both",
        boxSizing: "border-box",
        position: "relative",
          overflowY: "auto",
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: 12,
          padding: 20,
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
          <div style={{ position: "relative", width: "100%" }}>
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
                maxHeight: "min(48vh, 560px)",
                background: "#000",
                borderRadius: 8,
                marginBottom: 16,
              }}
            />

            {coverOverlays
              .filter(
                (overlay) =>
                  timelinePlayheadTime >= overlay.start &&
                  timelinePlayheadTime <= overlay.end,
              )
              .map((overlay) => (
                <div
                  key={overlay.id}
                  onPointerDown={(e) =>
                    handleCoverOverlayPointerDown(e, overlay.id)
                  }
                  style={{
                    position: "absolute",
                    left: `${overlay.x}%`,
                    top: `${overlay.y}%`,
                    width: `${overlay.width}%`,
                    height: `${overlay.height}%`,
                    background: "#000",
                    zIndex: 2,
                    pointerEvents: "auto",
                    cursor: "move",
                    touchAction: "none",
                  }}
                >
                  <div
                    onPointerDown={(e) =>
                      handleCoverOverlayResizePointerDown(e, overlay.id)
                    }
                    style={{
                      position: "absolute",
                      right: -7,
                      bottom: -7,
                      width: 14,
                      height: 14,
                      borderRadius: 3,
                      background: "#FC2667",
                      border: "2px solid #FFFFFF",
                      boxSizing: "border-box",
                      cursor: "nwse-resize",
                      touchAction: "none",
                    }}
                  />
                </div>
              ))}
          </div>
        )}

        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 12,
            marginBottom: 12,
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

          <button
            type="button"
            onClick={handleAddCoverOverlay}
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
            + Abdeckung
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

      {selectedCoverOverlay && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 12,
            marginBottom: 12,
            padding: "10px 12px",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
          }}
        >
          <strong>Abdeckung:</strong>

          <label>
            Start{" "}
            <input
              type="number"
              min={0}
              max={Math.max(0, selectedCoverOverlay.end - 0.1)}
              step={0.1}
              value={selectedCoverOverlay.start}
              onChange={(e) => {
                const value = Number(e.target.value);
                if (!Number.isFinite(value)) return;

                const start = Math.max(
                  0,
                  Math.min(value, selectedCoverOverlay.end - 0.1),
                );

                setCoverOverlays((previous) =>
                  previous.map((overlay) =>
                    overlay.id === selectedCoverOverlay.id
                      ? { ...overlay, start }
                      : overlay,
                  ),
                );
              }}
              style={{ width: 80, marginLeft: 6 }}
            />
            {" s"}
          </label>

          <label>
            Ende{" "}
            <input
              type="number"
              min={selectedCoverOverlay.start + 0.1}
              max={timelineDuration}
              step={0.1}
              value={selectedCoverOverlay.end}
              onChange={(e) => {
                const value = Number(e.target.value);
                if (!Number.isFinite(value)) return;

                const end = Math.min(
                  timelineDuration,
                  Math.max(value, selectedCoverOverlay.start + 0.1),
                );

                setCoverOverlays((previous) =>
                  previous.map((overlay) =>
                    overlay.id === selectedCoverOverlay.id
                      ? { ...overlay, end }
                      : overlay,
                  ),
                );
              }}
              style={{ width: 80, marginLeft: 6 }}
            />
            {" s"}
          </label>

          <button
            type="button"
            onClick={handleCopyCoverOverlay}
            style={{
              border: "1px solid var(--color-border)",
              borderRadius: 7,
              padding: "6px 10px",
              background: "#FFFFFF",
              color: "#0F172A",
              fontWeight: 600,
              cursor: "pointer",
            }}
          >
            Kopieren
          </button>

          <button
            type="button"
            onClick={handlePasteCoverOverlay}
            disabled={!copiedCoverOverlay}
            style={{
              border: "1px solid var(--color-border)",
              borderRadius: 7,
              padding: "6px 10px",
              background: "#FFFFFF",
              color: "#0F172A",
              fontWeight: 600,
              cursor: copiedCoverOverlay ? "pointer" : "default",
              opacity: copiedCoverOverlay ? 1 : 0.45,
            }}
          >
            Einfügen
          </button>
        </div>
      )}

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
                  cursor: "grab",
                  touchAction: "none",
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
            onClick={() => {
              const index = TIMELINE_ZOOM_LEVELS.indexOf(
                timelineZoom as (typeof TIMELINE_ZOOM_LEVELS)[number],
              );
              setTimelineZoom(TIMELINE_ZOOM_LEVELS[Math.max(0, index - 1)]);
            }}
            disabled={timelineZoom <= 1}
          >
            −
          </button>

          <input
            type="range"
            min="0"
            max={TIMELINE_ZOOM_LEVELS.length - 1}
            step="1"
            value={TIMELINE_ZOOM_LEVELS.indexOf(
              timelineZoom as (typeof TIMELINE_ZOOM_LEVELS)[number],
            )}
            onChange={(e) =>
              setTimelineZoom(TIMELINE_ZOOM_LEVELS[Number(e.currentTarget.value)])
            }
            aria-label="Timeline-Zoom"
            style={{ width: 190 }}
          />

          <button
            type="button"
            onClick={() => {
              const index = TIMELINE_ZOOM_LEVELS.indexOf(
                timelineZoom as (typeof TIMELINE_ZOOM_LEVELS)[number],
              );
              setTimelineZoom(
                TIMELINE_ZOOM_LEVELS[Math.min(TIMELINE_ZOOM_LEVELS.length - 1, index + 1)],
              );
            }}
            disabled={
              timelineZoom >= TIMELINE_ZOOM_LEVELS[TIMELINE_ZOOM_LEVELS.length - 1]
            }
          >
            +
          </button>

          <span
            style={{
              fontSize: 12,
              color: "var(--color-text-secondary)",
            }}
          >
            {timelineZoom === 1 ? "1.0" : timelineZoom}×
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
          data-testid="video-editor-timeline-scroll"
          style={{
            overflowX: "auto",
            overflowY: "hidden",
            paddingBottom: 6,
          }}
        >
        <div
          data-testid="video-editor-timeline-ruler"
          data-tick-step={timelineTickStep}
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
          data-testid="video-editor-overlay-track"
          style={{
            position: "relative",
            height: 38,
            width: `${timelineZoom * 100}%`,
            minWidth: "100%",
            marginBottom: 4,
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            background: "#F8FAFC",
            overflow: "hidden",
          }}
        >
          {coverOverlays.length === 0 && (
            <span
              style={{
                position: "absolute",
                left: 10,
                top: 9,
                fontSize: 12,
                color: "var(--color-text-secondary)",
              }}
            >
              Abdeckungen
            </span>
          )}

          {coverOverlays.map((overlay, index) => {
            const left =
              timelineDuration > 0
                ? (overlay.start / timelineDuration) * 100
                : 0;

            const width =
              timelineDuration > 0
                ? ((overlay.end - overlay.start) / timelineDuration) * 100
                : 0;

            const selected = overlay.id === selectedCoverOverlayId;

            return (
              <div
                key={overlay.id}
                onClick={(e) => {
                  e.stopPropagation();
                  setSelectedCoverOverlayId(overlay.id);
                }}
                onPointerDown={(e) =>
                  handleCoverOverlayTimelineMovePointerDown(e, overlay.id)
                }
                style={{
                  position: "absolute",
                  top: 4,
                  bottom: 4,
                  left: `${left}%`,
                  width: `${width}%`,
                  minWidth: 4,
                  borderRadius: 5,
                  background: selected ? "#FC2667" : "#F7C2D2",
                  border: "1px solid #FC2667",
                  color: selected ? "#FFFFFF" : "#0F172A",
                  fontSize: 11,
                  fontWeight: 600,
                  padding: "5px 7px",
                  boxSizing: "border-box",
                  overflow: "hidden",
                  whiteSpace: "nowrap",
                  cursor: "pointer",
                }}
              >
                Abdeckung {index + 1}

                <div
                  onPointerDown={(e) =>
                    handleCoverOverlayTimelineStartPointerDown(e, overlay.id)
                  }
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    bottom: 0,
                    width: 12,
                    borderRight: "2px solid #FFFFFF",
                    background: "rgba(255,255,255,0.22)",
                    cursor: "ew-resize",
                    touchAction: "none",
                  }}
                  title="Start der Abdeckung ziehen"
                />

                <div
                  onPointerDown={(e) =>
                    handleCoverOverlayTimelineEndPointerDown(e, overlay.id)
                  }
                  style={{
                    position: "absolute",
                    top: 0,
                    right: 0,
                    bottom: 0,
                    width: 12,
                    borderLeft: "2px solid #FFFFFF",
                    background: "rgba(255,255,255,0.22)",
                    cursor: "ew-resize",
                    touchAction: "none",
                  }}
                  title="Ende der Abdeckung ziehen"
                />
              </div>
            );
          })}
        </div>

          <div
            ref={timelineRef}
            data-testid="video-editor-timeline"
            data-zoom={timelineZoom}
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

          <button
            type="button"
            onClick={handleRenderTimeline}
            disabled={rendering || clips.length === 0 || timelineDuration < 1}
            style={{
              background: "#E6467A",
              color: "#FFFFFF",
              border: "none",
              borderRadius: 8,
              padding: "9px 18px",
              fontWeight: 600,
              cursor: rendering ? "default" : "pointer",
              opacity: rendering ? 0.6 : 1,
            }}
          >
            {rendering ? "Video wird gerendert..." : "Als neues Video rendern"}
          </button>
        </div>
        {renderStatus === "ready" && renderedVideoId && (
          <div style={{ marginTop: 12, textAlign: "center", color: "var(--color-text)" }}>
            Render abgeschlossen. <a href={`/videos/${renderedVideoId}`}>Bearbeitetes Video öffnen</a>
          </div>
        )}
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
