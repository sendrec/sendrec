import { test, expect } from "@playwright/test";
import { loginViaAPI, loginAsSecondUser, getAccessToken, TEST_USER_2 } from "../helpers/auth";
import {
  createWorkspace,
  inviteToWorkspace,
  acceptInviteViaAPI,
  switchToWorkspace,
  uploadTestVideo,
  navigateToOrgSettings,
} from "../helpers/workspace";

test.describe.serial("Workspace Viewer Role", () => {
  let workspaceId: string;
  const workspaceName = `Viewer Test Workspace ${Date.now()}`;

  test("setup: create workspace, upload video, invite viewer", async ({ page }) => {
    await loginViaAPI(page);
    const ws = await createWorkspace(page, workspaceName);
    workspaceId = ws.id;

    await page.goto("/");
    await switchToWorkspace(page, workspaceName);
    await uploadTestVideo(page);

    const token = await inviteToWorkspace(page, workspaceId, TEST_USER_2.email, "viewer");
    await loginAsSecondUser(page);
    await acceptInviteViaAPI(page, token);
  });

  test("viewer appears in members list", async ({ page }) => {
    await loginViaAPI(page);
    await navigateToOrgSettings(page, workspaceId);
    await expect(page.getByLabel("Workspace name")).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(TEST_USER_2.email)).toBeVisible({ timeout: 5000 });
    await expect(page.getByLabel(`Role for ${TEST_USER_2.name}`)).toHaveValue("viewer");
  });

  test("viewer can browse library", async ({ page }) => {
    await loginAsSecondUser(page);
    await page.goto("/");
    await switchToWorkspace(page, workspaceName);
    await page.goto("/library");
    await expect(page.locator(".video-card").first()).toBeVisible({ timeout: 15000 });
  });

  test("viewer does not see Record nav link", async ({ page }) => {
    await loginAsSecondUser(page);
    await page.goto("/");
    await switchToWorkspace(page, workspaceName);
    await expect(page.getByRole("link", { name: /record/i })).not.toBeVisible();
  });

  test("viewer does not see Delete in video dropdown", async ({ page }) => {
    await loginAsSecondUser(page);
    await page.goto("/");
    await switchToWorkspace(page, workspaceName);
    await page.goto("/library");
    await expect(page.locator(".video-card").first()).toBeVisible({ timeout: 15000 });

    await page.getByLabel("More actions").first().click();
    const dropdown = page.locator(".dropdown-menu");
    await expect(dropdown.getByText("Analytics")).toBeVisible();
    await expect(dropdown.getByText("Download")).toBeVisible();
    await expect(dropdown.getByText("Delete")).not.toBeVisible();
  });

  test("viewer does not see Move to... in dropdown", async ({ page }) => {
    await loginAsSecondUser(page);
    await page.goto("/");
    await switchToWorkspace(page, workspaceName);
    await page.goto("/library");
    await expect(page.locator(".video-card").first()).toBeVisible({ timeout: 15000 });

    await page.getByLabel("More actions").first().click();
    const dropdown = page.locator(".dropdown-menu");
    await expect(dropdown.getByText("Move to...")).not.toBeVisible();
  });

  test("owner changes viewer to member, Record link appears", async ({ page }) => {
    await loginViaAPI(page);
    const authHeaders = { Authorization: `Bearer ${getAccessToken(page)}` };
    const membersResp = await page.request.get(`/api/organizations/${workspaceId}/members`, {
      headers: authHeaders,
    });
    const members = await membersResp.json();
    const viewer = members.find((m: { email: string }) => m.email === TEST_USER_2.email);

    const response = await page.request.patch(
      `/api/organizations/${workspaceId}/members/${viewer.id}`,
      { data: { role: "member" }, headers: authHeaders }
    );
    expect(response.ok()).toBeTruthy();

    await loginAsSecondUser(page);
    await page.goto("/");
    await switchToWorkspace(page, workspaceName);
    await expect(page.getByRole("link", { name: /record/i })).toBeVisible();
  });

  test("owner changes member back to viewer, Record link hidden", async ({ page }) => {
    await loginViaAPI(page);
    const authHeaders = { Authorization: `Bearer ${getAccessToken(page)}` };
    const membersResp = await page.request.get(`/api/organizations/${workspaceId}/members`, {
      headers: authHeaders,
    });
    const members = await membersResp.json();
    const member = members.find((m: { email: string }) => m.email === TEST_USER_2.email);

    const response = await page.request.patch(
      `/api/organizations/${workspaceId}/members/${member.id}`,
      { data: { role: "viewer" }, headers: authHeaders }
    );
    expect(response.ok()).toBeTruthy();

    await loginAsSecondUser(page);
    await page.goto("/");
    await switchToWorkspace(page, workspaceName);
    await expect(page.getByRole("link", { name: /record/i })).not.toBeVisible();
  });
});
