import { useCallback, useRef, useState } from "react";

export function useDocumentPictureInPictureWindow(supported: boolean) {
  const [pipWindow, setPipWindow] = useState<Window | null>(null);
  const pipWindowRef = useRef<Window | null>(null);
  const pendingWindowRef = useRef<Promise<Window | null> | null>(null);
  const requestIdRef = useRef(0);

  const close = useCallback(() => {
    requestIdRef.current += 1;
    pendingWindowRef.current = null;
    pipWindowRef.current?.close();
    pipWindowRef.current = null;
    setPipWindow(null);
  }, []);

  const open = useCallback(() => {
    if (!supported) return Promise.resolve(null);
    const dpip = window.documentPictureInPicture;
    if (!dpip) return Promise.resolve(null);

    const requestId = requestIdRef.current + 1;
    requestIdRef.current = requestId;
    const pendingWindow = dpip
      .requestWindow({ width: 280, height: 220 })
      .then((nextWindow) => {
        if (requestIdRef.current !== requestId) {
          nextWindow.close();
          return null;
        }
        pipWindowRef.current = nextWindow;
        setPipWindow(nextWindow);
        return nextWindow;
      })
      .catch(() => {
        if (requestIdRef.current === requestId) {
          pipWindowRef.current = null;
          setPipWindow(null);
        }
        return null;
      });
    pendingWindowRef.current = pendingWindow;
    return pendingWindow;
  }, [supported]);

  const waitForOpen = useCallback(async () => {
    if (pendingWindowRef.current) {
      await pendingWindowRef.current;
    }
  }, []);

  return { pipWindow, open, close, waitForOpen };
}
