import { test, expect } from "@playwright/test";
import { deepLinkPath, waitForIncident } from "../../lib/helpers";

const TITLE = "Invoices posted without GL entry";

test.describe("Deep-linked incident page (/i/:token)", () => {
  let path = "";
  test.beforeAll(async ({ request }) => {
    await waitForIncident(request, TITLE);
    path = await deepLinkPath(request, TITLE);
  });

  test("shows header, facts and the exact captured rows", async ({ page }) => {
    await page.goto(path);
    await expect(page.getByRole("heading", { level: 1, name: TITLE })).toBeVisible();
    await expect(page.getByText("Business Exception", { exact: true })).toBeVisible();
    await expect(page.getByText("ERP Production (demo)", { exact: true })).toBeVisible();

    const grid = page.locator("table").first();
    await expect(grid.locator("thead th")).toContainText(["#", "invoice_no", "order_id", "amount", "posted_at"]);
    await expect(grid.locator("tbody tr")).toHaveCount(2);
    await expect(grid).toContainText("INV-90001");
    await expect(grid).toContainText("INV-90002");
  });

  test("grid filter, sort and CSV export", async ({ page }) => {
    await page.goto(path);
    const grid = page.locator("table").first();
    await page.getByPlaceholder("Filter rows…").fill("90002");
    await expect(grid.locator("tbody tr")).toHaveCount(1);
    await expect(page.getByText("1 of 2 rows")).toBeVisible();
    await page.getByPlaceholder("Filter rows…").fill("");

    await grid.getByRole("columnheader", { name: "amount" }).click(); // ascending
    await expect(grid.locator("tbody tr").first()).toContainText("INV-90001");
    await grid.getByRole("columnheader", { name: /amount/ }).click(); // descending
    await expect(grid.locator("tbody tr").first()).toContainText("INV-90002");

    const [download] = await Promise.all([page.waitForEvent("download"), page.getByRole("button", { name: "Export CSV" }).click()]);
    expect(download.suggestedFilename()).toBe("incident-rows.csv");
    const csv = await (await download.createReadStream()).toArray().then((c) => Buffer.concat(c).toString());
    expect(csv.split("\n")[0]).toBe("invoice_no,order_id,amount,posted_at");
    expect(csv).toContain("INV-90001");
  });

  test("shows the configured corrective action plan as a checklist", async ({ page }) => {
    await page.goto(path);
    const plan = page.locator("ol");
    await expect(plan.locator("li")).toHaveCount(4);
    await expect(plan).toContainText("Re-run the GL posting job");
    const first = plan.locator("li").first();
    await first.getByRole("checkbox").check();
    await expect(first.locator("span.text-sm")).toHaveClass(/line-through/);
  });

  test("acknowledge from the deep link updates status", async ({ page }) => {
    await page.goto(path);
    const ack = page.getByRole("button", { name: "Acknowledge" });
    if (await ack.isVisible()) {
      await ack.click();
      await expect(page.getByText("Acknowledged by")).toBeVisible();
      await expect(page.locator("header")).toContainText("acknowledged");
    }
  });

  test("invalid / expired link shows a safe error page", async ({ page }) => {
    await page.goto("/i/" + "x".repeat(43));
    await expect(page.getByRole("heading", { name: "Incident link not found" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Go to incidents" })).toBeVisible();
  });
});
