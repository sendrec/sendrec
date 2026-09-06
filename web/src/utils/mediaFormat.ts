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

// Same chain as getSupportedMimeType with MP4 removed. Chrome's MP4 muxer
// mishandles some stream types, and the reliable escape is a WebM container.
export function getSupportedWebMMimeType(): string {
  if (typeof MediaRecorder === "undefined") return getSupportedMimeType();
  if (MediaRecorder.isTypeSupported("video/webm;codecs=vp9,opus"))
    return "video/webm;codecs=vp9,opus";
  if (MediaRecorder.isTypeSupported("video/webm;codecs=vp8,opus"))
    return "video/webm;codecs=vp8,opus";
  if (MediaRecorder.isTypeSupported("video/webm")) return "video/webm";
  return getSupportedMimeType();
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
