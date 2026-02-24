export function getSupportedMimeType(): string {
  if (typeof MediaRecorder === "undefined") return "video/mp4";
  if (MediaRecorder.isTypeSupported("video/mp4")) return "video/mp4";
  if (MediaRecorder.isTypeSupported("video/webm;codecs=vp9,opus"))
    return "video/webm;codecs=vp9,opus";
  if (MediaRecorder.isTypeSupported("video/webm;codecs=vp8,opus"))
    return "video/webm;codecs=vp8,opus";
  if (MediaRecorder.isTypeSupported("video/webm")) return "video/webm";
  return "video/mp4";
}

export function getSupportedVideoMimeType(): string {
  if (typeof MediaRecorder === "undefined") return "video/mp4";
  if (MediaRecorder.isTypeSupported("video/mp4")) return "video/mp4";
  if (MediaRecorder.isTypeSupported("video/webm;codecs=vp9"))
    return "video/webm;codecs=vp9";
  if (MediaRecorder.isTypeSupported("video/webm;codecs=vp8"))
    return "video/webm;codecs=vp8";
  if (MediaRecorder.isTypeSupported("video/webm")) return "video/webm";
  return "video/mp4";
}

export function blobTypeFromMimeType(mimeType: string): string {
  return mimeType.startsWith("video/webm") ? "video/webm" : "video/mp4";
}
