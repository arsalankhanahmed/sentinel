import { test, expect } from "@playwright/test";
import { deepLinkPath, waitForIncident, MOCK } from "../../lib/helpers";

/**
 * Regenerates the screenshots used in docs/USER_MANUAL.md.
 *   npx playwright test --project=docs
 */
const OUT = "../../docs/images";

test.describe.configure({ mode: "serial" });

test("capture documentation screenshots", async ({ page, request }) => {
  test.setTimeout(180_000);
  await waitForIncident(request, "Invoices posted without GL entry");
  await waitForIncident(request, "Database unreachable: Finance SQL Server (offline)");
  await waitForIncident(request, "Endpoint down: IIS web02 – OrderPortal");

  await page.goto("/");
  await expect(page.locator("tbody tr").first()).toBeVisible();
  await page.screenshot({ path: `${OUT}/01-dashboard.png` });

  await page.goto(await deepLinkPath(request, "Invoices posted without GL entry"));
  await expect(page.locator("table tbody tr").first()).toBeVisible();
  await page.screenshot({ path: `${OUT}/02-incident-deeplink.png` });

  await page.goto(await deepLinkPath(request, "Endpoint down: IIS web02 – OrderPortal"));
  await expect(page.locator("table tbody tr").first()).toBeVisible();
  await page.screenshot({ path: `${OUT}/03-incident-outage.png` });

  for (const [path, file] of [
    ["/config/sources", "04-data-sources"], ["/config/rules", "05-rules"], ["/config/watchers", "06-log-watchers"],
    ["/config/probes", "07-probes"], ["/config/channels", "08-channels"],
  ]) {
    await page.goto(path);
    await expect(page.locator("tbody tr").first()).toBeVisible();
    await page.screenshot({ path: `${OUT}/${file}.png`, fullPage: true });
  }

  await page.goto(MOCK);
  await expect(page.locator(".card").first()).toBeVisible();
  await page.screenshot({ path: `${OUT}/09-mock-inbox.png` });

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(await deepLinkPath(request, "Orders stuck in PROCESSING > 2 hours"));
  await expect(page.locator("table tbody tr").first()).toBeVisible();
  await page.screenshot({ path: `${OUT}/10-mobile-deeplink.png`, fullPage: true });
});
