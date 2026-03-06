import { test, expect } from "@playwright/test";
import { loginViaAPI } from "../helpers/auth";
import {
  createWorkspace,
  switchToWorkspace,
  switchToPersonal,
  uploadTestVideo,
} from "../helpers/workspace";

test.describe.serial("Video Transfer", () => {
  let workspaceId: string;
  const workspaceName = `Transfer Test Workspace ${Date.now()}`;

  test("setup: create workspace and upload video in personal", async ({ page }) => {
    await loginViaAPI(page);
    const ws = await createWorkspace(page, workspaceName);
    workspaceId = ws.id;

    await page.goto("/");
    await switchToPersonal(page);
    await uploadTestVideo(page);
  });

  test("Move to... appears in personal video dropdown", async ({ page }) => {
    await loginViaAPI(page);
    await page.goto("/library");
    await expect(page.locator(".video-card").first()).toBeVisible({ timeout: 15000 });

    await page.getByLabel("More actions").first().click();
    await expect(page.getByText("Move to...")).toBeVisible();
  });

  test("transfer personal video to workspace", async ({ page }) => {
    await loginViaAPI(page);
    await page.goto("/library");
    await expect(page.locator(".video-card").first()).toBeVisible({ timeout: 15000 });

    await page.getByLabel("More actions").first().click();
    await page.getByText("Move to...").click();

    const dialog = page.getByRole("dialog", { name: "Transfer video" });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText(workspaceName)).toBeVisible();

    await dialog.getByText(workspaceName).click();
    await dialog.getByRole("button", { name: "Move" }).click();

    await expect(dialog).not.toBeVisible({ timeout: 5000 });

    await switchToWorkspace(page, workspaceName);
    await page.goto("/library");
    await expect(page.locator(".video-card").first()).toBeVisible({ timeout: 15000 });
  });

  test("transfer dialog shows Personal when in workspace", async ({ page }) => {
    await loginViaAPI(page);
    await page.goto("/");
    await switchToWorkspace(page, workspaceName);
    await page.goto("/library");
    await expect(page.locator(".video-card").first()).toBeVisible({ timeout: 15000 });

    await page.getByLabel("More actions").first().click();
    await page.getByText("Move to...").click();

    const dialog = page.getByRole("dialog", { name: "Transfer video" });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText("Personal")).toBeVisible();
  });

  test("transfer workspace video back to personal", async ({ page }) => {
    await loginViaAPI(page);
    await page.goto("/");
    await switchToWorkspace(page, workspaceName);
    await page.goto("/library");
    await expect(page.locator(".video-card").first()).toBeVisible({ timeout: 15000 });

    await page.getByLabel("More actions").first().click();
    await page.getByText("Move to...").click();

    const dialog = page.getByRole("dialog", { name: "Transfer video" });
    await dialog.getByText("Personal").click();
    await dialog.getByRole("button", { name: "Move" }).click();

    await expect(dialog).not.toBeVisible({ timeout: 5000 });

    await switchToPersonal(page);
    await page.goto("/library");
    await expect(page.locator(".video-card").first()).toBeVisible({ timeout: 15000 });
  });
});
