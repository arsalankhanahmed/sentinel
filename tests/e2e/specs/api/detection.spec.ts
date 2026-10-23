import { test, expect } from "@playwright/test";
import { api, incidents, waitForIncident, deepLinkPath } from "../../lib/helpers";

// The demo ERP seed plants a known number of exceptions per rule (see demo/appdb/postgres/02_seed.sql).
const EXPECTED = [
  { title: "Invoices posted without GL entry", rows: 2, severity: "critical", contains: "INV-90001" },
  { title: "Negative stock on hand", rows: 2, severity: "warning", contains: "SKU-1004" },
  { title: "Orders stuck in PROCESSING > 2 hours", rows: 3, severity: "critical", contains: "Punjab Textiles Ltd" },
  { title: "Duplicate payments against same invoice", rows: 1, severity: "critical", contains: "BANK-DUP-7781" },
];

test.describe("Module A — Business data exceptions", () => {
  for (const e of EXPECTED) {
    test(`detects: ${e.title} (${e.rows} rows)`, async ({ request }) => {
      const inc = await waitForIncident(request, e.title);
      expect(inc.kind).toBe("business_exception");
      expect(inc.severity).toBe(e.severity);
      expect(inc.row_count).toBe(e.rows);

      // Captured rows are reachable ONLY through the deep link
      const path = await deepLinkPath(request, e.title);
      const full = await (await request.get(`/api/v1/incidents/link/${path.split("/").pop()}`)).json();
      expect(full.payload).toHaveLength(e.rows);
      expect(JSON.stringify(full.payload)).toContain(e.contains);
      expect(full.corrective_action.length).toBeGreaterThan(10);
    });
  }

  test("a rule returning 0 rows raises no incident and reports ok", async ({ request }) => {
    await expect
      .poll(async () => (await api<any[]>(request, "GET", "/rules")).body.find((r) => r.name === "Shipped orders without invoice")?.last_status, { timeout: 30_000 })
      .toBe("ok");
    expect((await incidents(request)).some((i) => i.title === "Shipped orders without invoice")).toBe(false);
  });

  test("a persisting exception is de-duplicated (one incident, occurrences grow)", async ({ request }) => {
    const title = "Negative stock on hand";
    await waitForIncident(request, title);
    await expect
      .poll(async () => (await incidents(request)).find((i) => i.title === title)!.occurrences, { timeout: 30_000 })
      .toBeGreaterThan(1);
    expect((await incidents(request)).filter((i) => i.title === title && i.status !== "resolved")).toHaveLength(1);
  });
});

test.describe("Module B — Zero-code runtime exceptions (log watcher)", () => {
  test("historical log rows are NOT alerted (watcher starts at current max id)", async ({ request }) => {
    await waitForIncident(request, "Negative stock on hand"); // engine is warm
    const runtime = (await incidents(request)).filter((i) => i.kind === "runtime_exception");
    for (const r of runtime) {
      const path = await deepLinkPath(request, r.title).catch(() => "");
      if (!path) continue;
      const full = await (await request.get(`/api/v1/incidents/link/${path.split("/").pop()}`)).json();
      expect(JSON.stringify(full.payload)).not.toContain("(historical)");
    }
  });
});

test.describe("Module C — Infrastructure & DB outages", () => {
  test("unreachable database raises a DB outage", async ({ request }) => {
    const inc = await waitForIncident(request, "Database unreachable: Finance SQL Server (offline)");
    expect(inc.kind).toBe("db_outage");
    expect(inc.severity).toBe("critical");
  });

  test("HTTP 503 endpoint raises an endpoint outage with IIS guidance", async ({ request }) => {
    const title = "Endpoint down: IIS web02 – OrderPortal";
    await waitForIncident(request, title);
    const path = await deepLinkPath(request, title);
    const full = await (await request.get(`/api/v1/incidents/link/${path.split("/").pop()}`)).json();
    expect(full.payload[0].http_status).toBe(503);
    expect(full.corrective_action).toContain("App Pool");
  });

  test("refused TCP port raises an endpoint outage", async ({ request }) => {
    const inc = await waitForIncident(request, "Endpoint down: Legacy SQL Server (DR) :1433");
    expect(inc.kind).toBe("endpoint_outage");
  });

  test("healthy endpoint raises nothing", async ({ request }) => {
    await waitForIncident(request, "Endpoint down: IIS web02 – OrderPortal");
    expect((await incidents(request)).some((i) => i.title === "Endpoint down: Notification gateway health")).toBe(false);
  });
});
