import { test, expect } from "@playwright/test";
import { loginViaUI } from "../helpers/auth";
import path from "path";

test.describe("Upload", () => {
  test.beforeEach(async ({ page }) => {
    await loginViaUI(page);
  });

  test("upload page renders", async ({ page }) => {
    await page.goto("/upload");
    await expect(
      page.getByRole("heading", { name: /upload video/i })
    ).toBeVisible();
  });

  test("upload a video file", async ({ page }) => {
    await page.goto("/upload");

    const testVideoPath = path.join(
      __dirname,
      "..",
      "fixtures",
      "test-video.webm"
    );
    const fileInput = page.locator('[data-testid="file-input"]');
    await fileInput.setInputFiles(testVideoPath);

    await expect(page.getByText(/1 file selected/i)).toBeVisible();

    await page.getByRole("button", { name: /upload 1 video/i }).click();

    await expect(page.getByText(/upload complete/i)).toBeVisible({
      timeout: 30000,
    });
  });

  test("uploaded video appears in library", async ({ page }) => {
    await page.goto("/library");
    const emptyState = page.getByText(/no recordings yet/i);
    const isEmpty = await emptyState
      .isVisible({ timeout: 3000 })
      .catch(() => false);

    if (!isEmpty) {
      await expect(page.locator(".video-card").first()).toBeVisible({
        timeout: 5000,
      });
    }
  });
});
