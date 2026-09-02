import { test, expect } from "@playwright/test";
import { queryRows } from "../helpers/db";
import { getAccessToken, loginViaAPI } from "../helpers/auth";
import { uploadTestVideo } from "../helpers/workspace";

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
    await loginViaAPI(page);
    await uploadTestVideo(page);

    const [video] = await queryRows<{ id: string; share_token: string }>(
      "SELECT id, share_token FROM videos WHERE title = 'Untitled Video' ORDER BY created_at DESC LIMIT 1"
    );
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
    await page.goto(`/watch/${video.share_token}`);

    await page.locator("video").dispatchEvent("ended");
    await expect(page.locator("#player-container #cta-card")).toBeVisible();
    await page.locator("video").dispatchEvent("play");
    await expect(page.locator("#player-container #cta-card")).toBeHidden();
  });
});
