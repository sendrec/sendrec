import { describe, it, expect, afterEach } from "vitest";
import { supportsDocumentPictureInPicture } from "./browserCapabilities";

describe("supportsDocumentPictureInPicture", () => {
  const original = (window as unknown as { documentPictureInPicture?: unknown })
    .documentPictureInPicture;

  afterEach(() => {
    if (original === undefined) {
      delete (window as unknown as { documentPictureInPicture?: unknown })
        .documentPictureInPicture;
    } else {
      (window as unknown as { documentPictureInPicture?: unknown })
        .documentPictureInPicture = original;
    }
  });

  it("returns true when documentPictureInPicture is on window", () => {
    (window as unknown as { documentPictureInPicture?: unknown })
      .documentPictureInPicture = { requestWindow: () => Promise.resolve({}) };
    expect(supportsDocumentPictureInPicture()).toBe(true);
  });

  it("returns false when documentPictureInPicture is missing", () => {
    delete (window as unknown as { documentPictureInPicture?: unknown })
      .documentPictureInPicture;
    expect(supportsDocumentPictureInPicture()).toBe(false);
  });
});
