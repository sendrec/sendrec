import { test, expect } from "@playwright/test";
import { TEST_USER, loginViaUI } from "../helpers/auth";

test.describe("Settings", () => {
  test.beforeEach(async ({ page }) => {
    await loginViaUI(page);
  });

  test("settings page loads with user profile", async ({ page }) => {
    await page.goto("/settings");
    await expect(page.locator(`input[value="${TEST_USER.name}"]`)).toBeVisible({
      timeout: 5000,
    });
    await expect(
      page.locator(`input[value="${TEST_USER.email}"]`)
    ).toBeVisible();
  });
});
