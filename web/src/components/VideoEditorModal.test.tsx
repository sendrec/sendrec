import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { VideoEditorModal } from "./VideoEditorModal";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

describe("VideoEditorModal multi-source preview", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    vi.spyOn(HTMLMediaElement.prototype, "load").mockImplementation(() => undefined);
    vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => undefined);
    vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue(undefined);
    mockApiFetch.mockResolvedValueOnce({ downloadUrl: "https://media.example/original.mp4" });
  });

  async function renderWithInsertedVideo() {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce([
      {
        id: "inserted",
        title: "Inserted source",
        status: "ready",
        duration: 30,
      },
    ]);

    const { container } = render(
      <VideoEditorModal
        videoId="original"
        duration={120}
        onClose={vi.fn()}
      />,
    );

    await waitFor(() => expect(container.querySelector("video")).not.toBeNull());
    await user.click(screen.getByRole("button", { name: /Video einfügen/ }));
    await user.click(await screen.findByRole("button", { name: /Inserted source/ }));
    mockApiFetch.mockResolvedValueOnce({ downloadUrl: "https://media.example/inserted.mp4" });
    await user.click(screen.getByRole("button", { name: "Hier einfügen" }));

    const video = container.querySelector("video")!;
    Object.defineProperty(video, "duration", { configurable: true, value: 30 });
    await waitFor(() => expect(video.src).toBe("https://media.example/inserted.mp4"));
    fireEvent.loadedMetadata(video);

    const timeline = screen.getByTestId("video-editor-timeline");
    vi.spyOn(timeline, "getBoundingClientRect").mockReturnValue({
      x: 0,
      y: 0,
      left: 0,
      top: 0,
      right: 1000,
      bottom: 64,
      width: 1000,
      height: 64,
      toJSON: () => ({}),
    });

    return { container, timeline };
  }

  it("loads the clicked clip source at its corresponding source time", async () => {
    const { container } = await renderWithInsertedVideo();

    fireEvent.click(screen.getByText("Eingefügt: Inserted source"), { clientX: 100 });

    const video = container.querySelector("video")!;

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/inserted/download");
      expect(video.src).toBe("https://media.example/inserted.mp4");
      expect(video.currentTime).toBeCloseTo(15, 3);
    });
  });

  it("continues with the next clip when the active clip ends", async () => {
    const { container } = await renderWithInsertedVideo();

    fireEvent.click(screen.getByText("Eingefügt: Inserted source"), { clientX: 100 });
    const video = container.querySelector("video")!;
    await waitFor(() => expect(video.currentTime).toBeCloseTo(15, 3));

    Object.defineProperty(video, "paused", { configurable: true, value: false });
    video.currentTime = 30;
    fireEvent.timeUpdate(video);
    Object.defineProperty(video, "duration", { configurable: true, value: 120 });
    await waitFor(() => expect(video.src).toBe("https://media.example/original.mp4"));
    fireEvent.loadedMetadata(video);

    await waitFor(() => expect(video.currentTime).toBe(0));
  });

  it("ignores stale metadata callbacks during rapid source changes", async () => {
    const { container } = await renderWithInsertedVideo();
    const video = container.querySelector("video")!;

    fireEvent.click(screen.getByText("Clip 2"), { clientX: 500 });
    fireEvent.click(screen.getByText("Eingefügt: Inserted source"), { clientX: 100 });

    Object.defineProperty(video, "duration", { configurable: true, value: 30 });
    fireEvent.loadedMetadata(video);

    await waitFor(() => {
      expect(video.src).toBe("https://media.example/inserted.mp4");
      expect(video.currentTime).toBeCloseTo(15, 3);
    });
  });

  it("shows source loading errors in the existing error area", async () => {
    const { container } = await renderWithInsertedVideo();
    mockApiFetch.mockResolvedValueOnce([
      {
        id: "broken",
        title: "Broken source",
        status: "ready",
        duration: 20,
      },
    ]);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /Video einfügen/ }));
    await user.click(await screen.findByRole("button", { name: /Broken source/ }));
    mockApiFetch.mockRejectedValueOnce(new Error("Quelle ist nicht erreichbar"));
    await user.click(screen.getByRole("button", { name: "Hier einfügen" }));

    expect(container.querySelector("video")).not.toBeNull();

    expect(await screen.findByText("Quelle ist nicht erreichbar")).toBeInTheDocument();
  });
});
