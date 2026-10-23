import { test, expect } from "@playwright/test";
import { waitForIncident } from "../../lib/helpers";

test.describe("Incidents dashboard", () => {
  test.beforeAll(async ({ request }) => {
    await waitForIncident(request, "Orders stuck in PROCESSING > 2 hours");
    await waitForIncident(request, "Database unreachable: Finance SQL Server (offline)");
  });

  test("KPI cards show open counts per incident type", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "Incidents" })).toBeVisible();
    for (const label of ["Business Exception", "Runtime Exception", "DB Outage", "Endpoint Outage", "Last 24h"]) {
      await expect(page.locator(".card", { hasText: label }).first()).toBeVisible();
    }
    const business = page.locator(".card", { hasText: "Business Exception" }).first();
    await expect(business.locator("div").nth(1)).not.toHaveText("0");
  });

  test("lists open incidents with severity and type", async ({ page }) => {
    await page.goto("/");
    const row = page.locator("tbody tr", { hasText: "Orders stuck in PROCESSING > 2 hours" });
    await expect(row).toContainText("critical");
    await expect(row).toContainText("Business Exception");
    await expect(row).toContainText("ERP Production (demo)");
    await expect(page.locator("tbody tr", { hasText: "Database unreachable" })).toContainText("DB Outage");
  });

  test("status tabs filter the list", async ({ page }) => {
    await page.goto("/");
    const statusCells = page.locator("tbody tr td:nth-child(7)");
    // default tab = open: every row is open
    await expect(statusCells.first()).toBeVisible();
    expect(new Set(await statusCells.allTextContents())).toEqual(new Set(["open"]));

    await page.getByRole("button", { name: "resolved", exact: true }).click();
    await expect
      .poll(async () => [...new Set(await statusCells.allTextContents())].join(","))
      .toBe("resolved");

    await page.getByRole("button", { name: "all", exact: true }).click();
    await expect(page.locator("tbody tr", { hasText: "Orders stuck in PROCESSING" }).first()).toBeVisible();
  });

  test("acknowledge an incident from the list", async ({ page }) => {
    await page.goto("/");
    const row = page.locator("tbody tr", { hasText: "Orders stuck in PROCESSING > 2 hours" });
    const ack = row.getByRole("button", { name: "Ack" });
    if (await ack.count()) {
      await ack.click();
      await page.getByRole("button", { name: "acknowledged", exact: true }).click();
      await expect(page.locator("tbody tr", { hasText: "Orders stuck in PROCESSING > 2 hours" })).toContainText("acknowledged");
    }
  });

  test("sidebar navigation reaches every configuration page", async ({ page }) => {
    await page.goto("/");
    for (const [link, heading] of [
      ["Data Sources", "Data Sources"], ["Exception Rules", "Business Exception Rules"],
      ["Log Watchers", "Runtime Log Watchers"], ["Probes", "Infrastructure Probes"], ["Channels", "Notification Channels"],
    ]) {
      await page.getByRole("link", { name: link }).click();
      await expect(page.getByRole("heading", { level: 1, name: heading })).toBeVisible();
    }
  });
});
