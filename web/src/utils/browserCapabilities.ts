export function supportsDocumentPictureInPicture(): boolean {
  return typeof window !== "undefined" && "documentPictureInPicture" in window;
}
