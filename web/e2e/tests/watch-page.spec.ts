import { test, expect } from "@playwright/test";
import { queryRows } from "../helpers/db";

test.describe("Watch Page", () => {
  test("watch page renders for a valid share token", async ({ page }) => {
    const rows = await queryRows<{ share_token: string }>(
      "SELECT share_token FROM videos WHERE status = 'ready' LIMIT 1"
    );

    test.skip(rows.length === 0, "No video available for watch page test");

    await page.goto(`/watch/${rows[0].share_token}`);
    await expect(page.locator("video")).toBeVisible({ timeout: 10000 });
  });

  test("watch page returns 404 for invalid token", async ({ page }) => {
    const response = await page.goto("/watch/nonexistent-token-12345");
    expect(response?.status()).toBe(404);
  });
});
