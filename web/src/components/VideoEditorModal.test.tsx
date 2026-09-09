import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { VideoEditorModal } from "./VideoEditorModal";

const mockApiFetch = vi.fn();

const emptyEditorState = {
  timeline: { version: 0, clips: [] },
  renderStatus: "none",
  renderError: null,
  renderedVideoId: null,
};

let libraryVideos: unknown[];
let editorState: typeof emptyEditorState | {
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
};
let sourceLoadError: Error | null;

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

describe("VideoEditorModal multi-source preview", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    libraryVideos = [];
    editorState = emptyEditorState;
    sourceLoadError = null;
    vi.spyOn(HTMLMediaElement.prototype, "load").mockImplementation(() => undefined);
    vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => undefined);
    vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue(undefined);
    mockApiFetch.mockImplementation((path: string, options?: RequestInit) => {
      if (path === "/api/videos/original/editor" && !options) return Promise.resolve(editorState);
      if (path === "/api/videos/original/editor/render") return Promise.resolve(undefined);
      if (path === "/api/videos") return Promise.resolve(libraryVideos);
      if (path === "/api/videos/original/download") {
        return Promise.resolve({ downloadUrl: "https://media.example/original.mp4" });
      }
      if (path === "/api/videos/inserted/download") {
        return Promise.resolve({ downloadUrl: "https://media.example/inserted.mp4" });
      }
      if (path === "/api/videos/broken/download" && sourceLoadError) {
        return Promise.reject(sourceLoadError);
      }
      return Promise.reject(new Error(`Unexpected API call: ${path}`));
    });
  });

  async function renderWithInsertedVideo() {
    const user = userEvent.setup();
    libraryVideos = [
      {
        id: "inserted",
        title: "Inserted source",
        status: "ready",
        duration: 30,
      },
    ];

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
    libraryVideos = [
      {
        id: "broken",
        title: "Broken source",
        status: "ready",
        duration: 20,
      },
    ];
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /Video einfügen/ }));
    await user.click(await screen.findByRole("button", { name: /Broken source/ }));
    sourceLoadError = new Error("Quelle ist nicht erreichbar");
    await user.click(screen.getByRole("button", { name: "Hier einfügen" }));

    expect(container.querySelector("video")).not.toBeNull();

    expect(await screen.findByText("Quelle ist nicht erreichbar")).toBeInTheDocument();
  });

  it("restores a persisted multi-source timeline", async () => {
    editorState = {
      timeline: {
        version: 1,
        clips: [
          { id: "clip-7", sourceId: "original", sourceStart: 5, sourceEnd: 10, duration: 5 },
          { id: "clip-8", sourceId: "inserted", sourceStart: 12, sourceEnd: 20, duration: 8 },
        ],
      },
      renderStatus: "none",
      renderError: null,
      renderedVideoId: null,
    };

    render(<VideoEditorModal videoId="original" duration={120} onClose={vi.fn()} />);

    expect(await screen.findByTestId("video-editor-clip-clip-7")).toBeInTheDocument();
    expect(screen.getByTestId("video-editor-clip-clip-8")).toBeInTheDocument();
    expect(screen.getByText("0:13")).toBeInTheDocument();
  });

  it("sends the complete ordered timeline when rendering starts", async () => {
    const user = userEvent.setup();
    await renderWithInsertedVideo();

    await user.click(screen.getByRole("button", { name: "Als neues Video rendern" }));

    await waitFor(() => {
      const renderCall = mockApiFetch.mock.calls.find(
        ([path]) => path === "/api/videos/original/editor/render",
      );
      expect(renderCall).toBeDefined();
      const payload = JSON.parse((renderCall?.[1] as RequestInit).body as string);
      expect(payload.version).toBe(1);
      expect(payload.clips.map((clip: { sourceId: string }) => clip.sourceId)).toEqual([
        "inserted",
        "original",
      ]);
      expect(payload.clips[0]).toMatchObject({
        sourceId: "inserted",
        sourceStart: 0,
        sourceEnd: 30,
        duration: 30,
      });
    });
    expect(screen.getByRole("button", { name: "Video wird gerendert..." })).toBeDisabled();
  });
});
