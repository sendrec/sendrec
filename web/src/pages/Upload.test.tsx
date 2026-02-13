import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Upload } from "./Upload";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

function renderUpload() {
  return render(
    <MemoryRouter>
      <Upload />
    </MemoryRouter>
  );
}

function createMockFile(name: string, size: number): File {
  const content = new Uint8Array(size);
  return new File([content], name, { type: "video/mp4" });
}

describe("Upload", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(null, { status: 200 }));
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders file input and heading", () => {
    renderUpload();
    expect(screen.getByText("Upload Video")).toBeInTheDocument();
    expect(screen.getByTestId("file-input")).toBeInTheDocument();
  });

  it("does not show title or upload button before file is selected", () => {
    renderUpload();
    expect(screen.queryByText("Upload")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Title")).not.toBeInTheDocument();
  });

  it("auto-fills title from filename when file is selected", async () => {
    const user = userEvent.setup();
    renderUpload();

    const file = createMockFile("my-presentation.mp4", 1024);
    const input = screen.getByTestId("file-input");
    await user.upload(input, file);

    const titleInput = screen.getByLabelText("Title") as HTMLInputElement;
    expect(titleInput.value).toBe("my-presentation");
    expect(screen.getByText("Upload")).toBeInTheDocument();
  });

  it("uploads file and shows share URL on success", async () => {
    const user = userEvent.setup();
    mockApiFetch
      .mockResolvedValueOnce({
        id: "video-1",
        uploadUrl: "https://s3.example.com/upload?signed=xyz",
        shareToken: "abc123defghi",
      })
      .mockResolvedValueOnce(undefined);

    renderUpload();

    const file = createMockFile("demo.mp4", 2048);
    await user.upload(screen.getByTestId("file-input"), file);
    await user.click(screen.getByText("Upload"));

    await waitFor(() => {
      expect(screen.getByText("Upload complete")).toBeInTheDocument();
    });

    expect(screen.getByText(/abc123defghi/)).toBeInTheDocument();

    expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/upload", {
      method: "POST",
      body: JSON.stringify({
        title: "demo",
        fileSize: 2048,
        contentType: "video/mp4",
      }),
    });

    expect(globalThis.fetch).toHaveBeenCalledWith(
      "https://s3.example.com/upload?signed=xyz",
      expect.objectContaining({
        method: "PUT",
        headers: { "Content-Type": "video/mp4" },
      })
    );

    expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/video-1", {
      method: "PATCH",
      body: JSON.stringify({ status: "ready" }),
    });
  });

  it("shows error on upload failure", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockRejectedValueOnce(new Error("monthly limit exceeded"));

    renderUpload();

    const file = createMockFile("demo.mp4", 2048);
    await user.upload(screen.getByTestId("file-input"), file);
    await user.click(screen.getByText("Upload"));

    await waitFor(() => {
      expect(screen.getByText("monthly limit exceeded")).toBeInTheDocument();
    });
    expect(screen.getByText("Try again")).toBeInTheDocument();
  });

  it("shows uploading state", async () => {
    const user = userEvent.setup();
    let resolveUpload: (value: unknown) => void;
    mockApiFetch.mockReturnValueOnce(new Promise((r) => { resolveUpload = r; }));

    renderUpload();

    const file = createMockFile("demo.mp4", 2048);
    await user.upload(screen.getByTestId("file-input"), file);
    await user.click(screen.getByText("Upload"));

    expect(screen.getByText("Uploading...")).toBeInTheDocument();

    resolveUpload!({
      id: "video-1",
      uploadUrl: "https://s3.example.com/upload",
      shareToken: "abc123defghi",
    });
  });
});
