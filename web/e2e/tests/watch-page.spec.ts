import { test, expect } from "@playwright/test";
import { queryRows } from "../helpers/db";
import { getAccessToken, loginViaAPI } from "../helpers/auth";
import { readFile } from "fs/promises";
import { fileURLToPath } from "url";
import { dirname, join } from "path";

const __dirname = dirname(fileURLToPath(import.meta.url));

test.describe("Watch Page", () => {
  test("watch page renders for a valid share token", async ({ page }) => {
    const rows = await queryRows<{ share_token: string }>(
      "SELECT share_token FROM videos WHERE status IN ('ready', 'processing') LIMIT 1"
    );

    test.skip(rows.length === 0, "No video available for watch page test");

    await page.goto(`/watch/${rows[0].share_token}`);
    await expect(page.locator("video")).toBeVisible({ timeout: 10000 });
  });

  test("watch page returns 404 for invalid token", async ({ page }) => {
    const response = await page.goto("/watch/nonexistent-token-12345");
    expect(response?.status()).toBe(404);
  });

  test("shows the CTA overlay after video end even when localStorage is blocked", async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 640 });
    await loginViaAPI(page);
    const file = await readFile(join(__dirname, "..", "fixtures", "test-video.webm"));
    const create = await page.request.post("/api/videos/", {
      data: {
        title: "CTA overlay test",
        duration: 1,
        fileSize: file.length,
        contentType: "video/webm",
      },
      headers: { Authorization: `Bearer ${getAccessToken(page)}` },
    });
    expect(create.ok()).toBe(true);
    const video = await create.json();

    const upload = await page.request.put(video.uploadUrl, {
      data: file,
      headers: { "Content-Type": "video/webm" },
    });
    expect(upload.ok()).toBe(true);
    const ready = await page.request.patch(`/api/videos/${video.id}`, {
      data: { status: "ready" },
      headers: { Authorization: `Bearer ${getAccessToken(page)}` },
    });
    expect(ready.ok()).toBe(true);
    const cta = await page.request.put(`/api/videos/${video.id}/cta`, {
      data: { text: "Book a demo", url: "https://example.com/demo" },
      headers: { Authorization: `Bearer ${getAccessToken(page)}` },
    });
    expect(cta.ok()).toBe(true);

    await page.addInitScript(() => {
      Object.defineProperty(window, "localStorage", {
        get: () => ({ getItem: () => { throw new DOMException("Blocked", "SecurityError"); } }),
      });
    });
    await page.goto(`/watch/${video.shareToken}`);

    await page.locator("video").dispatchEvent("ended");
    await expect(page.locator("#player-container #cta-card")).toBeVisible();
    const playerBox = await page.locator("#player-container").boundingBox();
    const dismissBox = await page.locator("#cta-dismiss").boundingBox();
    const ctaButtonBox = await page.locator("#cta-btn").boundingBox();
    if (!playerBox || !dismissBox || !ctaButtonBox) {
      throw new Error("expected CTA controls and player bounds");
    }
    for (const box of [dismissBox, ctaButtonBox]) {
      expect(box.x).toBeGreaterThanOrEqual(playerBox.x);
      expect(box.y).toBeGreaterThanOrEqual(playerBox.y);
      expect(box.x + box.width).toBeLessThanOrEqual(playerBox.x + playerBox.width);
      expect(box.y + box.height).toBeLessThanOrEqual(playerBox.y + playerBox.height);
    }
    await page.locator("video").dispatchEvent("play");
    await expect(page.locator("#player-container #cta-card")).toBeHidden();
  });
});
