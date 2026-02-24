import { test, expect } from "@playwright/test";
import { loginViaAPI } from "../helpers/auth";

test.describe("Library", () => {
  test.beforeEach(async ({ page }) => {
    await loginViaAPI(page);
  });

  test("library shows empty state for new user", async ({ page }) => {
    await page.goto("/library");
    await expect(page.getByText(/no recordings yet/i)).toBeVisible();
  });

  test("library nav link is accessible", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: /library/i }).click();
    await expect(page).toHaveURL("/library");
  });
});
