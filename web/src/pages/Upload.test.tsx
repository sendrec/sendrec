import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Upload } from "./Upload";
import { expectNoA11yViolations } from "../test-utils/a11y";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

class MockXMLHttpRequest {
  open = vi.fn();
  setRequestHeader = vi.fn();
  status = 200;
  upload = { onprogress: null as ((e: any) => void) | null };
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  send = vi.fn().mockImplementation(() => {
    Promise.resolve().then(() => {
      if (this.onload) this.onload();
    });
  });
}

function renderUpload() {
  return render(
    <MemoryRouter>
      <Upload />
    </MemoryRouter>
  );
}

function createMockFile(name: string, size: number, type = "video/mp4"): File {
  const content = new Uint8Array(size);
  return new File([content], name, { type });
}

describe("Upload", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    globalThis.XMLHttpRequest = MockXMLHttpRequest as any;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("has no accessibility violations", async () => {
    const { container } = renderUpload();
    await expectNoA11yViolations(container);
  });

  it("renders file input and drop zone", () => {
    renderUpload();
    expect(screen.getByTestId("file-input")).toBeInTheDocument();
    expect(screen.getByText("Drag and drop your videos here")).toBeInTheDocument();
  });

  it("renders dropzone with instructions", () => {
    renderUpload();
    expect(screen.getByText("Drag and drop your videos here")).toBeInTheDocument();
    expect(screen.getByText(/or click to browse/)).toBeInTheDocument();
    expect(screen.getByText("MP4, WebM, MOV")).toBeInTheDocument();
  });

  it("does not show upload button before files are selected", () => {
    renderUpload();
    expect(screen.queryByText(/Upload \d/)).not.toBeInTheDocument();
  });

  it("auto-fills title from filename when file is selected", async () => {
    const user = userEvent.setup();
    renderUpload();

    const file = createMockFile("my-presentation.mp4", 1024);
    const input = screen.getByTestId("file-input");
    await user.upload(input, file);

    const titleInput = screen.getByLabelText("Title for my-presentation.mp4") as HTMLInputElement;
    expect(titleInput.value).toBe("my-presentation");
    expect(screen.getByText("Upload 1 video")).toBeInTheDocument();
  });

  it("shows file count after selection", async () => {
    const user = userEvent.setup();
    renderUpload();

    const files = [
      createMockFile("demo1.mp4", 5 * 1024 * 1024),
      createMockFile("demo2.mp4", 3 * 1024 * 1024),
    ];
    await user.upload(screen.getByTestId("file-input"), files);

    expect(screen.getByText("2 files selected")).toBeInTheDocument();
    expect(screen.getByText("Upload 2 videos")).toBeInTheDocument();
  });

  it("accepts files via drag and drop", () => {
    renderUpload();

    const file = createMockFile("dropped.mp4", 2048);
    const dropzone = screen.getByRole("button", { name: /drag and drop/i });

    fireEvent.drop(dropzone, {
      dataTransfer: { files: [file] },
    });

    expect(screen.getByLabelText("Title for dropped.mp4")).toHaveValue("dropped");
  });

  it("rejects unsupported file types on drop", () => {
    renderUpload();

    const file = createMockFile("movie.avi", 2048, "video/avi");
    const dropzone = screen.getByRole("button", { name: /drag and drop/i });

    fireEvent.drop(dropzone, {
      dataTransfer: { files: [file] },
    });

    expect(screen.getByText("Only MP4, WebM, and MOV files are supported")).toBeInTheDocument();
  });

  it("allows removing individual files", async () => {
    const user = userEvent.setup();
    renderUpload();

    const files = [
      createMockFile("video1.mp4", 1024),
      createMockFile("video2.mp4", 2048),
    ];
    await user.upload(screen.getByTestId("file-input"), files);

    expect(screen.getByText("2 files selected")).toBeInTheDocument();

    await user.click(screen.getByLabelText("Remove video1.mp4"));

    expect(screen.getByText("1 file selected")).toBeInTheDocument();
    expect(screen.queryByLabelText("Title for video1.mp4")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Title for video2.mp4")).toBeInTheDocument();
  });

  it("enforces maximum 10 files", async () => {
    const user = userEvent.setup();
    renderUpload();

    const files = Array.from({ length: 12 }, (_, i) =>
      createMockFile(`video${i + 1}.mp4`, 1024)
    );
    await user.upload(screen.getByTestId("file-input"), files);

    expect(screen.getByText("10 files selected")).toBeInTheDocument();
    expect(screen.getByText(/Only 10 of 12 files added/)).toBeInTheDocument();
  });

  it("uploads single file and shows share URL", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ maxVideosPerMonth: 25, videosUsedThisMonth: 0 })
      .mockResolvedValueOnce({
        id: "video-1",
        uploadUrl: "https://s3.example.com/upload?signed=xyz",
        shareToken: "abc123defghi",
      })
      .mockResolvedValueOnce(undefined);

    renderUpload();

    const file = createMockFile("demo.mp4", 2048);
    await user.upload(screen.getByTestId("file-input"), file);
    await user.click(screen.getByText("Upload 1 video"));

    await waitFor(() => {
      expect(screen.getByText("Upload complete")).toBeInTheDocument();
    });

    expect(screen.getByText(/abc123defghi/)).toBeInTheDocument();

    expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/limits");
    expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/upload", {
      method: "POST",
      body: JSON.stringify({
        title: "demo",
        fileSize: 2048,
        contentType: "video/mp4",
      }),
    });
  });

  it("uploads multiple files and shows results", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ maxVideosPerMonth: 25, videosUsedThisMonth: 0 })
      .mockResolvedValueOnce({
        id: "video-1",
        uploadUrl: "https://s3.example.com/upload1",
        shareToken: "token1",
      })
      .mockResolvedValueOnce(undefined)
      .mockResolvedValueOnce({
        id: "video-2",
        uploadUrl: "https://s3.example.com/upload2",
        shareToken: "token2",
      })
      .mockResolvedValueOnce(undefined);

    renderUpload();

    const files = [
      createMockFile("first.mp4", 1024),
      createMockFile("second.mp4", 2048),
    ];
    await user.upload(screen.getByTestId("file-input"), files);
    await user.click(screen.getByText("Upload 2 videos"));

    await waitFor(() => {
      expect(screen.getByText("2 videos uploaded")).toBeInTheDocument();
    });

    expect(screen.getByText(/token1/)).toBeInTheDocument();
    expect(screen.getByText(/token2/)).toBeInTheDocument();
  });

  it("shows error when monthly limit would be exceeded", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce({ maxVideosPerMonth: 25, videosUsedThisMonth: 23 });

    renderUpload();

    const files = [
      createMockFile("video1.mp4", 1024),
      createMockFile("video2.mp4", 2048),
      createMockFile("video3.mp4", 3072),
    ];
    await user.upload(screen.getByTestId("file-input"), files);
    await user.click(screen.getByText("Upload 3 videos"));

    await waitFor(() => {
      expect(screen.getByText("You can only upload 2 more videos this month")).toBeInTheDocument();
    });
  });

  it("shows error when monthly limit fully reached", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce({ maxVideosPerMonth: 25, videosUsedThisMonth: 25 });

    renderUpload();

    const file = createMockFile("video.mp4", 1024);
    await user.upload(screen.getByTestId("file-input"), file);
    await user.click(screen.getByText("Upload 1 video"));

    await waitFor(() => {
      expect(screen.getByText("Monthly video limit reached")).toBeInTheDocument();
    });
  });

  it("shows partial failure results", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ maxVideosPerMonth: 0, videosUsedThisMonth: 0 })
      .mockResolvedValueOnce({
        id: "video-1",
        uploadUrl: "https://s3.example.com/upload1",
        shareToken: "token1",
      })
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error("file too large"));

    renderUpload();

    const files = [
      createMockFile("good.mp4", 1024),
      createMockFile("bad.mp4", 2048),
    ];
    await user.upload(screen.getByTestId("file-input"), files);
    await user.click(screen.getByText("Upload 2 videos"));

    await waitFor(() => {
      expect(screen.getByText("1 of 2 uploaded")).toBeInTheDocument();
    });

    expect(screen.getByText(/token1/)).toBeInTheDocument();
    expect(screen.getByText(/bad\.mp4: file too large/)).toBeInTheDocument();
  });

  it("shows uploading state with file counter", async () => {
    const user = userEvent.setup();
    let resolveUpload: (value: unknown) => void;
    mockApiFetch
      .mockResolvedValueOnce({ maxVideosPerMonth: 0, videosUsedThisMonth: 0 })
      .mockReturnValueOnce(new Promise((r) => { resolveUpload = r; }));

    renderUpload();

    const file = createMockFile("demo.mp4", 2048);
    await user.upload(screen.getByTestId("file-input"), file);
    await user.click(screen.getByText("Upload 1 video"));

    expect(screen.getByText("Uploading 1 of 1...")).toBeInTheDocument();
    expect(screen.getByText("demo.mp4")).toBeInTheDocument();

    await act(async () => {
      resolveUpload!({
        id: "video-1",
        uploadUrl: "https://s3.example.com/upload",
        shareToken: "abc123defghi",
      });
    });
  });

  it("skips limits check for pro users and proceeds", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ maxVideosPerMonth: 0, videosUsedThisMonth: 0 })
      .mockResolvedValueOnce({
        id: "video-1",
        uploadUrl: "https://s3.example.com/upload",
        shareToken: "token1",
      })
      .mockResolvedValueOnce(undefined);

    renderUpload();

    const file = createMockFile("demo.mp4", 2048);
    await user.upload(screen.getByTestId("file-input"), file);
    await user.click(screen.getByText("Upload 1 video"));

    await waitFor(() => {
      expect(screen.getByText("Upload complete")).toBeInTheDocument();
    });
  });

  it("shows error on upload failure for single file", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({ maxVideosPerMonth: 25, videosUsedThisMonth: 0 })
      .mockRejectedValueOnce(new Error("monthly limit exceeded"));

    renderUpload();

    const file = createMockFile("demo.mp4", 2048);
    await user.upload(screen.getByTestId("file-input"), file);
    await user.click(screen.getByText("Upload 1 video"));

    await waitFor(() => {
      expect(screen.getByText(/monthly limit exceeded/)).toBeInTheDocument();
    });
  });

});
