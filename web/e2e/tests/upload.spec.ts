import { test, expect } from "@playwright/test";
import { loginViaAPI } from "../helpers/auth";
import { fileURLToPath } from "url";
import { dirname, join } from "path";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

test.describe("Upload", () => {
  test.beforeEach(async ({ page }) => {
    await loginViaAPI(page);
  });

  test("upload page renders", async ({ page }) => {
    await page.goto("/upload");
    await expect(
      page.getByRole("heading", { name: /upload video/i })
    ).toBeVisible();
  });

  test("upload a video file", async ({ page }) => {
    await page.goto("/upload");

    const testVideoPath = join(__dirname, "..", "fixtures", "test-video.webm");
    const fileInput = page.locator('[data-testid="file-input"]');
    await fileInput.setInputFiles(testVideoPath);

    await expect(page.getByText(/1 file/i)).toBeVisible();

    await page.getByRole("button", { name: /upload/i }).click();

    await expect(page.getByText(/upload complete/i)).toBeVisible({
      timeout: 60000,
    });
  });

  test("uploaded video appears in library", async ({ page }) => {
    await page.goto("/library");
    await expect(page.locator(".video-card").first()).toBeVisible({
      timeout: 15000,
    });
  });
});
