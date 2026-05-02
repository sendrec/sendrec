import { test, expect } from "@playwright/test";
import { loginViaAPI } from "../helpers/auth";

test.describe("Recording floating controls", () => {
  test.beforeEach(async ({ page }) => {
    await loginViaAPI(page);
  });

  test("opens Document Picture-in-Picture controls and stops recording from there", async ({
    page,
  }) => {
    await page.goto("/");
    await expect(
      page.getByRole("heading", { name: /new recording/i }),
    ).toBeVisible();

    const supportsDocumentPictureInPicture = await page.evaluate(
      () => "documentPictureInPicture" in window,
    );
    test.skip(
      !supportsDocumentPictureInPicture,
      "Document Picture-in-Picture is not available in this Chromium build",
    );

    await page.getByRole("button", { name: "Start recording" }).click();
    await page.getByTestId("countdown-overlay").click();

    await expect
      .poll(() =>
        page.evaluate(
          () =>
            window.documentPictureInPicture?.window?.document.body
              .textContent ?? "",
        ),
      )
      .toContain("Stop");

    await expect(page.locator(".recording-header")).toHaveCount(0);
    await expect
      .poll(() =>
        page.evaluate(() =>
          window.documentPictureInPicture?.window?.document.querySelector(
            ".recording-dot--active",
          ) !== null,
        ),
      )
      .toBe(true);

    await page.evaluate(() => {
      const pipDocument = window.documentPictureInPicture?.window?.document;
      const stopButton = pipDocument?.querySelector<HTMLButtonElement>(
        '[aria-label="Stop recording"]',
      );
      stopButton?.click();
    });

    await expect(
      page.getByText(
        /Creating video|Uploading recording|Finalizing|Your video is ready/i,
      ),
    ).toBeVisible({ timeout: 30_000 });
  });
});
