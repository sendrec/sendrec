import { chromium } from "@playwright/test";
import { dirname, resolve } from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const OUTPUT_DIR = resolve(__dirname, "../../.github/screenshots");

const BASE_URL = process.env.BASE_URL || "https://app.sendrec.eu";
const EMAIL = process.env.EMAIL || "";
const PASSWORD = process.env.PASSWORD || "";
const WATCH_VIDEO_ID = process.env.WATCH_VIDEO_ID || "d138698c-af90-4a56-b197-32d747413d70";

if (!EMAIL || !PASSWORD) {
  console.error("EMAIL and PASSWORD env vars required");
  process.exit(1);
}

async function main() {
  const browser = await chromium.launch();
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
    colorScheme: "dark",
  });

  // Use a single page to preserve auth state across navigations
  const page = await context.newPage();

  // Login
  console.log("Logging in...");
  await page.goto(`${BASE_URL}/login`, { waitUntil: "networkidle" });
  await page.getByLabel("Email").fill(EMAIL);
  await page.getByLabel("Password").fill(PASSWORD);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("**/", { timeout: 15000 });
  console.log("Logged in successfully");

  // 1. Recording page
  console.log("1/5 Capturing recording page...");
  await page.goto(`${BASE_URL}/`, { waitUntil: "networkidle" });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: `${OUTPUT_DIR}/recording.png`, fullPage: false });

  // 2. Upload page
  console.log("2/5 Capturing upload page...");
  await page.goto(`${BASE_URL}/upload`, { waitUntil: "networkidle" });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: `${OUTPUT_DIR}/upload.png`, fullPage: false });

  // 3. Library page
  console.log("3/5 Capturing library page...");
  await page.goto(`${BASE_URL}/library`, { waitUntil: "networkidle" });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: `${OUTPUT_DIR}/library.png`, fullPage: false });

  // Intercept API response to get video data
  let videos: { id: string; shareToken: string }[] = [];
  page.on("response", async (response) => {
    if (response.url().includes("/api/videos") && !response.url().includes("/analytics") && response.ok()) {
      try {
        videos = await response.json();
      } catch {}
    }
  });
  await page.goto(`${BASE_URL}/library`, { waitUntil: "networkidle" });
  await page.waitForTimeout(1500);
  console.log(`Found ${videos.length} videos`);

  const targetVideo = videos.find((v) => v.id === WATCH_VIDEO_ID);

  // 4. Analytics page (must come before watch page — watch is server-rendered and breaks SPA auth)
  // Pick video with most views for analytics screenshot
  const analyticsVideoId = videos.length > 0
    ? [...videos].sort((a: any, b: any) => (b.viewCount ?? 0) - (a.viewCount ?? 0))[0].id
    : null;
  if (analyticsVideoId) {
    console.log(`4/5 Capturing analytics page for video ${analyticsVideoId}...`);
    await page.goto(`${BASE_URL}/videos/${analyticsVideoId}/analytics`, { waitUntil: "networkidle" });
    await page.waitForTimeout(2000);
    await page.screenshot({ path: `${OUTPUT_DIR}/analytics.png`, fullPage: false });
  } else {
    console.warn("4/5 Skipping analytics — no videos");
  }

  // 5. Watch page (public server-rendered page — do this last as it leaves the SPA)
  if (targetVideo?.shareToken) {
    console.log("5/5 Capturing watch page...");
    await page.goto(`${BASE_URL}/watch/${targetVideo.shareToken}`, { waitUntil: "networkidle" });
    await page.waitForTimeout(2000);
    await page.screenshot({ path: `${OUTPUT_DIR}/watch.png`, fullPage: false });
  } else {
    console.warn(`5/5 Skipping watch — video ${WATCH_VIDEO_ID} not found`);
  }

  await page.close();
  await context.close();
  await browser.close();
  console.log(`Done! Screenshots saved to ${OUTPUT_DIR}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
