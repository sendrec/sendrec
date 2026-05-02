import { afterEach, describe, expect, it, vi } from "vitest";
import { render, waitFor, within } from "@testing-library/react";
import { RecordingFloatingControls } from "./RecordingFloatingControls";

class MockMediaStream {
  getTracks = vi.fn().mockReturnValue([]);
}

globalThis.MediaStream = MockMediaStream as unknown as typeof MediaStream;

function createPictureInPictureWindow() {
  const pipDocument = document.implementation.createHTMLDocument("pip");
  const eventTarget = new EventTarget();
  const close = vi.fn();
  const pipWindow = {
    document: pipDocument,
    close,
    addEventListener: eventTarget.addEventListener.bind(eventTarget),
    removeEventListener: eventTarget.removeEventListener.bind(eventTarget),
    dispatchEvent: eventTarget.dispatchEvent.bind(eventTarget),
  } as unknown as Window;

  return { pipWindow, close };
}

const baseProps = {
  webcamStream: new MediaStream(),
  webcamEnabled: true,
  duration: 75,
  isPaused: false,
  remaining: null as number | null,
  onPause: vi.fn(),
  onResume: vi.fn(),
  onStop: vi.fn(),
};

describe("RecordingFloatingControls", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("renders timer, camera preview, Pause, and Stop inside the PiP window", async () => {
    const { pipWindow } = createPictureInPictureWindow();

    render(<RecordingFloatingControls {...baseProps} pipWindow={pipWindow} />);

    await waitFor(() => {
      expect(pipWindow.document.body.querySelector("#root")).not.toBeNull();
    });
    const pipBody = within(pipWindow.document.body);
    expect(pipBody.getByText("1:15")).toBeTruthy();
    expect(
      pipWindow.document.body.querySelector('[aria-label="Pause recording"]'),
    ).not.toBeNull();
    expect(
      pipWindow.document.body.querySelector('[aria-label="Stop recording"]'),
    ).not.toBeNull();
    expect(pipWindow.document.body.querySelector("video")).not.toBeNull();
  });

  it("invokes onStop when Stop is clicked in the PiP window", async () => {
    const { pipWindow } = createPictureInPictureWindow();
    const onStop = vi.fn();

    render(<RecordingFloatingControls {...baseProps} pipWindow={pipWindow} onStop={onStop} />);

    await waitFor(() => {
      expect(pipWindow.document.body.querySelector("button")).not.toBeNull();
    });
    const stopButton = pipWindow.document.body.querySelector(
      '[aria-label="Stop recording"]',
    ) as HTMLButtonElement;
    stopButton.click();

    expect(onStop).toHaveBeenCalledTimes(1);
  });

  it("invokes onStop when the PiP window fires pagehide", async () => {
    const { pipWindow } = createPictureInPictureWindow();
    const onStop = vi.fn();

    render(<RecordingFloatingControls {...baseProps} pipWindow={pipWindow} onStop={onStop} />);

    await waitFor(() => {
      expect(pipWindow.document.body.querySelector("#root")).not.toBeNull();
    });
    pipWindow.dispatchEvent(new Event("pagehide"));

    expect(onStop).toHaveBeenCalledTimes(1);
  });

  it("closes the PiP window on unmount", async () => {
    const { pipWindow, close } = createPictureInPictureWindow();

    const { unmount } = render(<RecordingFloatingControls {...baseProps} pipWindow={pipWindow} />);

    await waitFor(() => {
      expect(pipWindow.document.body.querySelector("#root")).not.toBeNull();
    });
    unmount();

    expect(close).toHaveBeenCalledTimes(1);
  });
});
