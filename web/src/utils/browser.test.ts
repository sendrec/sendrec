import { describe, it, expect, afterEach } from "vitest";
import { stallsScreenCaptureWhenHidden } from "./browser";

const realUserAgent = navigator.userAgent;

function withUserAgent(value: string) {
  Object.defineProperty(navigator, "userAgent", { value, configurable: true });
  return stallsScreenCaptureWhenHidden();
}

afterEach(() => {
  Object.defineProperty(navigator, "userAgent", {
    value: realUserAgent,
    configurable: true,
  });
});

describe("stallsScreenCaptureWhenHidden", () => {
  it("is true for Safari", () => {
    expect(
      withUserAgent(
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Safari/605.1.15",
      ),
    ).toBe(true);
  });

  // Every Chromium browser carries "Safari" in its user agent, so each one is a
  // chance to warn the wrong people.
  it.each([
    [
      "Chrome",
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36",
    ],
    [
      "Edge",
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 Edg/140.0.0.0",
    ],
    [
      "Chrome on iOS",
      "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/140.0.0.0 Mobile/15E148 Safari/604.1",
    ],
    [
      "Firefox",
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 14.6; rv:130.0) Gecko/20100101 Firefox/130.0",
    ],
  ])("is false for %s", (_name, ua) => {
    expect(withUserAgent(ua)).toBe(false);
  });
});
