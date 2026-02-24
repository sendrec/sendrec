export function getSupportedMimeType(): string {
  if (typeof MediaRecorder === "undefined") return "video/mp4";
  if (MediaRecorder.isTypeSupported("video/mp4")) return "video/mp4";
  if (MediaRecorder.isTypeSupported("video/webm;codecs=vp9,opus"))
    return "video/webm;codecs=vp9,opus";
  return "video/mp4";
}

export function getSupportedVideoMimeType(): string {
  if (typeof MediaRecorder === "undefined") return "video/mp4";
  if (MediaRecorder.isTypeSupported("video/mp4")) return "video/mp4";
  if (MediaRecorder.isTypeSupported("video/webm;codecs=vp9"))
    return "video/webm;codecs=vp9";
  return "video/mp4";
}

export function blobTypeFromMimeType(mimeType: string): string {
  return mimeType.startsWith("video/webm") ? "video/webm" : "video/mp4";
}
