import { test, expect } from "@playwright/test";
import { loginViaAPI, getAccessToken } from "../helpers/auth";
import { createWorkspace, navigateToOrgSettings } from "../helpers/workspace";

test.describe.serial("Workspace CRUD", () => {
  let workspaceId: string;
  const workspaceName = `E2E Test Workspace ${Date.now()}`;

  test.beforeEach(async ({ page }) => {
    await loginViaAPI(page);
  });

  test("create workspace shows in org switcher", async ({ page }) => {
    await page.goto("/");
    const ws = await createWorkspace(page, workspaceName);
    workspaceId = ws.id;

    await page.goto("/");
    await page.getByLabel("Switch workspace").click();
    await expect(page.getByRole("option", { name: workspaceName })).toBeVisible();
  });

  test("workspace starts on Free plan", async ({ page }) => {
    await navigateToOrgSettings(page, workspaceId);
    await expect(page.getByText("Free")).toBeVisible({ timeout: 5000 });
  });

  test("rename workspace", async ({ page }) => {
    const newName = "Renamed Workspace";
    const response = await page.request.patch(`/api/organizations/${workspaceId}`, {
      data: { name: newName },
      headers: { Authorization: `Bearer ${getAccessToken(page)}` },
    });
    expect(response.ok()).toBeTruthy();

    await navigateToOrgSettings(page, workspaceId);
    await expect(page.getByLabel("Workspace name")).toBeVisible({ timeout: 10000 });
    await expect(page.getByLabel("Workspace name")).toHaveValue(newName);
  });

  test("workspace settings shows owner in members list", async ({ page }) => {
    await navigateToOrgSettings(page, workspaceId);
    await expect(page.getByText("e2e@test.sendrec.local")).toBeVisible({ timeout: 5000 });
  });

  test("delete workspace removes from org switcher", async ({ page }) => {
    const response = await page.request.delete(`/api/organizations/${workspaceId}`, {
      headers: { Authorization: `Bearer ${getAccessToken(page)}` },
    });
    expect(response.ok()).toBeTruthy();

    await page.goto("/");
    await page.getByLabel("Switch workspace").click();
    await expect(page.getByRole("option", { name: "Renamed Workspace" })).not.toBeVisible();
  });
});
