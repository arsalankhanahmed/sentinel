import { test, expect } from "@playwright/test";
import { api, DEMO_DSN, waitForIncident } from "../../lib/helpers";

test.describe("Configuration through the UI", () => {
  test("add a data source and test the connection", async ({ page, request }) => {
    const name = `UI source ${Date.now()}`;
    await page.goto("/config/sources");
    await page.locator("form input").first().fill(name);
    await page.locator("form select").selectOption("postgres");
    await page.locator("form input[type=password]").fill(DEMO_DSN);
    await page.getByRole("button", { name: "Add" }).click();

    const row = page.locator("tbody tr", { hasText: name });
    await expect(row).toBeVisible();
    await row.getByRole("button", { name: "Test" }).click();
    await expect(row).toContainText(/✓ \d+ ms/);

    // DSN is not rendered anywhere after save
    await expect(page.locator("body")).not.toContainText("sentinel_ro:");

    page.once("dialog", (d) => d.accept());
    await row.getByRole("button", { name: "Delete" }).click();
    await expect(page.locator("tbody tr", { hasText: name })).toHaveCount(0);
    expect((await api<any[]>(request, "GET", "/datasources")).body.some((d) => d.name === name)).toBe(false);
  });

  test("schedule builder converts UI selections to cron", async ({ page }) => {
    await page.goto("/config/rules");
    const builder = page.locator("form").getByText("Schedule").locator("..");
    await builder.getByRole("button", { name: "minutes" }).click();
    await expect(builder.locator("code")).toHaveText("*/5 * * * *");
    await builder.getByRole("button", { name: "daily" }).click();
    await expect(builder.locator("code")).toHaveText("0 8 * * *");
    await builder.getByRole("button", { name: "weekly" }).click();
    await builder.locator("select").selectOption("5"); // Friday
    await expect(builder.locator("code")).toHaveText("0 8 * * 5");
    await builder.getByRole("button", { name: "hourly" }).click();
    await expect(builder.locator("code")).toHaveText("0 * * * *");
  });

  test("create a rule in the UI, run it, see the incident, then delete it", async ({ page, request }) => {
    const name = `UI rule ${Date.now()}`;
    await page.goto("/config/rules");
    const form = page.locator("form");
    await form.getByPlaceholder("Invoices posted without GL entry").fill(name);
    await form.locator("select").first().selectOption({ label: "ERP Production (demo) (postgres)" });
    await form.locator("textarea").first().fill("SELECT sku, warehouse, qty_on_hand FROM inventory WHERE qty_on_hand < 0");
    await form.getByRole("button", { name: "Custom cron" }).click();
    await form.getByPlaceholder("*/10 * * * *   or   @every 30s").fill("@every 1h");
    await form.locator("select").nth(1).selectOption("warning");
    await form.getByRole("button", { name: "Create rule" }).click();

    const row = page.locator("tbody tr", { hasText: name });
    await expect(row).toContainText("@every 1h");
    await row.getByRole("button", { name: "Run now" }).click();

    const inc = await waitForIncident(request, name, "", 30_000);
    expect(inc.row_count).toBe(2);
    await page.reload();
    await expect(page.locator("tbody tr", { hasText: name })).toContainText("exception: 2 rows");

    // edit → disable
    await page.locator("tbody tr", { hasText: name }).getByRole("button", { name: "Edit" }).click();
    await page.getByLabel("Enabled").uncheck();
    await page.getByRole("button", { name: "Save changes" }).click();
    await expect(page.locator("tbody tr", { hasText: name })).toHaveClass(/opacity-50/);

    page.once("dialog", (d) => d.accept());
    await page.locator("tbody tr", { hasText: name }).getByRole("button", { name: "Delete" }).click();
    await expect(page.locator("tbody tr", { hasText: name })).toHaveCount(0);
  });

  test("invalid cron is rejected with a readable error", async ({ page }) => {
    await page.goto("/config/rules");
    const form = page.locator("form");
    await form.getByPlaceholder("Invoices posted without GL entry").fill("bad schedule");
    await form.locator("select").first().selectOption({ index: 1 });
    await form.locator("textarea").first().fill("SELECT 1");
    await form.getByRole("button", { name: "Custom cron" }).click();
    await form.getByPlaceholder("*/10 * * * *   or   @every 30s").fill("every banana");
    await form.getByRole("button", { name: "Create rule" }).click();
    await expect(form).toContainText("invalid schedule");
  });

  test("send a test notification to a channel", async ({ page }) => {
    await page.goto("/config/channels");
    const row = page.locator("tbody tr", { hasText: "Ops Teams channel (mock)" });
    await row.getByRole("button", { name: "Send test" }).click();
    await expect(row).toContainText("✓ delivered");
  });

  test("watchers and probes pages list the demo configuration", async ({ page }) => {
    await page.goto("/config/watchers");
    await expect(page.locator("tbody")).toContainText("app_error_log");
    await page.goto("/config/probes");
    await expect(page.locator("tbody")).toContainText("IIS web02");
    await expect(page.locator("tbody")).toContainText("tcp");
  });
});
