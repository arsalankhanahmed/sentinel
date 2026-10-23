import { test, expect } from "@playwright/test";
import { api, incidents, mockMessages, waitForIncident } from "../../lib/helpers";

test.describe("Incident lifecycle: open → de-dupe → auto-resolve", () => {
  test("rule toggled from failing to passing auto-resolves and notifies once each way", async ({ request }) => {
    const title = `E2E lifecycle ${Date.now()}`;
    const ds = (await api<any[]>(request, "GET", "/datasources")).body.find((d) => d.driver === "postgres");
    const failing = { name: title, data_source_id: ds.id, schedule: "@every 2s", severity: "critical",
      sql_text: "SELECT 'E2E' AS marker, 42 AS value" };

    const created = await api<any>(request, "POST", "/rules", failing);
    expect(created.status).toBe(201);
    const id = created.body.id;

    // 1. opens
    const inc = await waitForIncident(request, title, "open");
    expect(inc.row_count).toBe(1);

    // 2. persists across several runs → still ONE notification per channel
    await expect.poll(async () => (await incidents(request)).find((i) => i.id === inc.id)!.occurrences, { timeout: 20_000 }).toBeGreaterThan(2);
    const opened = (await mockMessages(request)).filter((m) => m.channel === "teams" && m.view.title === title && !m.view.badge.startsWith("RESOLVED"));
    expect(opened).toHaveLength(1);

    // 3. data "fixed" → query returns 0 rows → auto-resolved
    expect((await api(request, "PUT", `/rules/${id}`, { ...failing, sql_text: "SELECT 1 AS x WHERE 1 = 0" })).status).toBe(200);
    await waitForIncident(request, title, "resolved", 30_000);

    // 4. RESOLVED notification delivered without a deep link
    await expect
      .poll(async () => (await mockMessages(request)).find((m) => m.channel === "teams" && m.view.title === title && m.view.badge.startsWith("RESOLVED")), { timeout: 20_000 })
      .toBeTruthy();
    const resolved = (await mockMessages(request)).find((m) => m.channel === "teams" && m.view.title === title && m.view.badge.startsWith("RESOLVED"))!;
    expect(resolved.view.link).toBe("");

    await api(request, "DELETE", `/rules/${id}`);
  });

  test("manual acknowledge and resolve", async ({ request }) => {
    const inc = await waitForIncident(request, "Endpoint down: Legacy SQL Server (DR) :1433");
    if (inc.status === "open") {
      expect((await api(request, "POST", `/incidents/${inc.id}/ack`)).status).toBe(200);
      expect((await incidents(request)).find((i) => i.id === inc.id)!.status).toBe("acknowledged");
      expect((await api(request, "POST", `/incidents/${inc.id}/ack`)).status).toBe(404); // only open → ack
    }
    expect((await api(request, "POST", `/incidents/${inc.id}/resolve`)).status).toBe(200);
    expect((await api(request, "POST", `/incidents/${inc.id}/resolve`)).status).toBe(404);
  });

  test("'Run now' queues a rule immediately", async ({ request }) => {
    const rule = (await api<any[]>(request, "GET", "/rules")).body[0];
    const r = await api(request, "POST", `/rules/${rule.id}/run`);
    expect([202, 409]).toContain(r.status); // 409 = already running (in-flight guard)
  });
});
