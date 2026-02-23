import { test, expect } from "@playwright/test";
import { TEST_USER, loginViaUI } from "../helpers/auth";

test.describe("Authentication", () => {
  test("login page renders with form fields", async ({ page }) => {
    await page.goto("/login");
    await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
    await expect(page.getByLabel("Email")).toBeVisible();
    await expect(page.getByLabel("Password")).toBeVisible();
    await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
  });

  test("login with valid credentials redirects to home", async ({ page }) => {
    await loginViaUI(page);
    await expect(page).toHaveURL("/");
  });

  test("login with wrong password shows error", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Email").fill(TEST_USER.email);
    await page.getByLabel("Password").fill("wrongpassword");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page.getByText(/invalid/i)).toBeVisible();
  });

  test("unauthenticated user is redirected to login", async ({ page }) => {
    await page.context().clearCookies();
    await page.goto("/library");
    await expect(page).toHaveURL(/\/login/);
  });

  test("logout returns to login page", async ({ page }) => {
    await loginViaUI(page);
    await page.getByRole("button", { name: "Sign out" }).click();
    await expect(page).toHaveURL(/\/login/);
  });

  test("register page renders with all fields", async ({ page }) => {
    await page.goto("/register");
    await expect(
      page.getByRole("heading", { name: "Create account" })
    ).toBeVisible();
    await expect(page.getByLabel("Name")).toBeVisible();
    await expect(page.getByLabel("Email")).toBeVisible();
    await expect(page.getByLabel("Password", { exact: true })).toBeVisible();
    await expect(page.getByLabel("Confirm password")).toBeVisible();
  });

  test("register with new email redirects to check-email", async ({
    page,
  }) => {
    const uniqueEmail = `e2e-register-${Date.now()}@test.sendrec.local`;
    await page.goto("/register");
    await page.getByLabel("Name").fill("New E2E User");
    await page.getByLabel("Email").fill(uniqueEmail);
    await page.getByLabel("Password", { exact: true }).fill("TestPassword123!");
    await page.getByLabel("Confirm password").fill("TestPassword123!");
    await page.getByRole("button", { name: "Create account" }).click();
    await expect(page).toHaveURL(/\/check-email/);
  });
});
