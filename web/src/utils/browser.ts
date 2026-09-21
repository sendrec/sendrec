/**
 * Whether this browser stops capturing the screen once its window is hidden.
 *
 * WebKit mutes a capture track when the page loses visibility, and a muted video
 * track repeats its last frame while the microphone keeps recording — so the
 * moment the user switches to the app they are demoing, the recording becomes a
 * full-length soundtrack over one still image. The page cannot prevent it, so
 * Safari users are warned before they start; useCaptureStallWatch catches it
 * once it happens, in whichever browser, and throws the recording away rather
 * than uploading it.
 *
 * The behaviour has no feature to test for, so the user agent is the only signal
 * available. Chromium browsers carry "Safari" in their user agent too, hence the
 * exclusions.
 */
export function stallsScreenCaptureWhenHidden(): boolean {
  const ua = navigator.userAgent;
  return /safari/i.test(ua) && !/chrome|chromium|crios|fxios|edg|android/i.test(ua);
}
