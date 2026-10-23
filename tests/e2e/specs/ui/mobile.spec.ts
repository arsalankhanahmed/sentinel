import { test, expect } from "@playwright/test";
import { deepLinkPath, waitForIncident } from "../../lib/helpers";

// Engineers usually open the deep link from Teams / WhatsApp on a phone.
test.describe("Mobile deep link experience", () => {
  const TITLE = "Duplicate payments against same invoice";

  test("deep link is fully usable on a phone with no horizontal page scroll", async ({ page, request }) => {
    await waitForIncident(request, TITLE);
    const path = await deepLinkPath(request, TITLE);
    await page.goto(path);

    await expect(page.getByRole("heading", { level: 1, name: TITLE })).toBeVisible();
    await expect(page.locator("table").first()).toContainText("BANK-DUP-7781");
    await expect(page.getByRole("button", { name: "Resolve" })).toBeVisible();
    await expect(page.getByText("Recommended corrective action plan")).toBeVisible();

    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(1);
  });

  test("dashboard renders on mobile", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "Incidents" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Data Sources" })).toBeVisible();
  });
});
