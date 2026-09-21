/**
 * Whether this browser stops capturing the screen once its window is hidden.
 *
 * WebKit mutes a capture track when the page loses visibility, and a muted video
 * track repeats its last frame while the microphone keeps recording — so the
 * moment the user switches to the app they are demoing, the recording becomes a
 * full-length soundtrack over one still image. Nothing in the page can detect or
 * prevent it, which is why Safari users are warned before they start.
 *
 * The behaviour has no feature to test for, so the user agent is the only signal
 * available. Chromium browsers carry "Safari" in their user agent too, hence the
 * exclusions.
 */
export function stallsScreenCaptureWhenHidden(): boolean {
  const ua = navigator.userAgent;
  return /safari/i.test(ua) && !/chrome|chromium|crios|fxios|edg|android/i.test(ua);
}
