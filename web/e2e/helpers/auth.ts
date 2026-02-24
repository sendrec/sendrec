import { type Page } from "@playwright/test";
import { query } from "./db";

export const TEST_USER = {
  name: "E2E Test User",
  email: "e2e@test.sendrec.local",
  password: "TestPassword123!",
};

export async function createVerifiedUser(): Promise<void> {
  const baseURL = process.env.BASE_URL || "http://localhost:8080";

  const resp = await fetch(`${baseURL}/api/auth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(TEST_USER),
  });

  if (resp.status !== 201 && resp.status !== 409) {
    throw new Error(`Registration failed: ${resp.status}`);
  }

  await query("UPDATE users SET email_verified = true WHERE email = $1", [
    TEST_USER.email,
  ]);
}

export async function loginViaUI(page: Page): Promise<void> {
  await page.goto("/login");
  await page.getByLabel("Email").fill(TEST_USER.email);
  await page.getByLabel("Password").fill(TEST_USER.password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("/");
}

export async function loginViaAPI(page: Page): Promise<void> {
  for (let attempt = 0; attempt < 3; attempt++) {
    const response = await page.request.post("/api/auth/login", {
      data: { email: TEST_USER.email, password: TEST_USER.password },
    });
    if (response.ok()) return;
    if (response.status() === 429) {
      await page.waitForTimeout(2000);
      continue;
    }
    throw new Error(`Login API failed: ${response.status()}`);
  }
  throw new Error("Login API failed: exceeded retries (429)");
}
