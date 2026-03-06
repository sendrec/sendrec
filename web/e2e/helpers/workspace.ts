import { type Page, expect } from "@playwright/test";
import { getAccessToken } from "./auth";
import { queryRows } from "./db";
import { fileURLToPath } from "url";
import { dirname, join } from "path";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

export async function createWorkspace(
  page: Page,
  name: string
): Promise<{ id: string; slug: string }> {
  const response = await page.request.post("/api/organizations", {
    data: { name },
    headers: { Authorization: `Bearer ${getAccessToken(page)}` },
  });
  if (!response.ok()) {
    throw new Error(`Create workspace failed: ${response.status()}`);
  }
  return response.json();
}

export async function inviteToWorkspace(
  page: Page,
  orgId: string,
  email: string,
  role: string
): Promise<{ acceptLink: string }> {
  const response = await page.request.post(
    `/api/organizations/${orgId}/invites`,
    {
      data: { email, role },
      headers: { Authorization: `Bearer ${getAccessToken(page)}` },
    }
  );
  if (!response.ok()) {
    throw new Error(
      `Invite failed: ${response.status()} ${await response.text()}`
    );
  }
  return response.json();
}

export async function getInviteToken(email: string): Promise<string> {
  const rows = await queryRows<{ token: string }>(
    "SELECT token FROM organization_invites WHERE email = $1 ORDER BY created_at DESC LIMIT 1",
    [email]
  );
  if (rows.length === 0) throw new Error(`No invite found for ${email}`);
  return rows[0].token;
}

export async function acceptInviteViaAPI(
  page: Page,
  token: string
): Promise<void> {
  const response = await page.request.post("/api/invites/accept", {
    data: { token },
    headers: { Authorization: `Bearer ${getAccessToken(page)}` },
  });
  if (!response.ok()) {
    throw new Error(
      `Accept invite failed: ${response.status()} ${await response.text()}`
    );
  }
}

export async function switchToWorkspace(
  page: Page,
  workspaceName: string
): Promise<void> {
  await page.getByLabel("Switch workspace").click();
  await page.getByRole("option", { name: workspaceName }).click();
  await page.waitForTimeout(500);
}

export async function switchToPersonal(page: Page): Promise<void> {
  await page.getByLabel("Switch workspace").click();
  await page.getByRole("option", { name: "Personal" }).click();
  await page.waitForTimeout(500);
}

export async function uploadTestVideo(page: Page): Promise<void> {
  await page.goto("/?tab=upload");

  const testVideoPath = join(__dirname, "..", "fixtures", "test-video.webm");
  const fileInput = page.locator('[data-testid="file-input"]');
  await fileInput.setInputFiles(testVideoPath);

  await expect(page.getByText(/1 file/i)).toBeVisible();
  await page.getByRole("button", { name: /upload \d+ video/i }).click();
  await expect(page.getByText(/upload complete/i)).toBeVisible({
    timeout: 60000,
  });
}
