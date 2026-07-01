import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TranscriptSection } from "./TranscriptSection";
import type { Video } from "../../types/video";

const mockApiFetch = vi.fn();

vi.mock("../../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

function makeVideo(overrides: Partial<Video> = {}): Video {
  return {
    id: "video-1",
    title: "Demo",
    shareToken: "abc123defghi",
    status: "ready",
    transcriptStatus: "ready",
    summaryStatus: "none",
    documentStatus: "none",
    ...overrides,
  } as Video;
}

function createVttFile(name = "captions.vtt"): File {
  const content = "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nHello\n";
  return new File([content], name, { type: "text/vtt" });
}

function renderSection(overrides: Partial<Video> = {}) {
  return render(
    <TranscriptSection
      video={makeVideo(overrides)}
      limits={{
        transcriptionEnabled: true,
        aiEnabled: true,
      } as any}
      isViewer={false}
      transcriptSegments={[]}
      retranscribeLanguage="auto"
      onRetranscribeLanguageChange={() => {}}
      onVideoUpdate={() => {}}
      onTranscriptClear={() => {}}
      onTranscriptSegmentsUpdate={() => {}}
    />
  );
}

describe("TranscriptSection upload", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
  });

  it("renders an upload transcript control", () => {
    renderSection();
    expect(
      screen.getByRole("button", { name: /upload transcript/i })
    ).toBeInTheDocument();
  });

  it("uploads a selected .vtt file via multipart POST", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce({
      segments: [{ start: 0, end: 1, text: "Hello" }],
    });

    renderSection();

    const file = createVttFile();
    const input = screen.getByTestId("transcript-upload-input");
    await user.upload(input, file);

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        "/api/videos/video-1/transcript",
        expect.objectContaining({
          method: "POST",
          body: expect.any(FormData),
        })
      );
    });

    const [, options] = mockApiFetch.mock.calls[0];
    const formData = options.body as FormData;
    expect(formData.get("file")).toBe(file);
  });

  it("shows an error toast on upload failure", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockRejectedValueOnce(new Error("invalid WebVTT file"));

    renderSection();

    const file = createVttFile();
    const input = screen.getByTestId("transcript-upload-input");
    await user.upload(input, file);

    await waitFor(() => {
      expect(screen.getByText(/invalid WebVTT file/)).toBeInTheDocument();
    });
  });
});
