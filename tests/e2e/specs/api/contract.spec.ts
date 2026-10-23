import { test, expect } from "@playwright/test";
import { api, incidents } from "../../lib/helpers";

test.describe("API contract & input validation", () => {
  test("health and readiness probes", async ({ request }) => {
    expect((await request.get("/healthz")).status()).toBe(200);
    const ready = await request.get("/readyz");
    expect(ready.status()).toBe(200);
    expect(await ready.json()).toEqual({ status: "ready" });
  });

  test("security headers on every API response", async ({ request }) => {
    const r = await request.get("/api/v1/stats");
    expect(r.headers()["referrer-policy"]).toBe("no-referrer");
    expect(r.headers()["x-frame-options"]).toBe("DENY");
    expect(r.headers()["cache-control"]).toBe("no-store");
    expect(r.headers()["x-request-id"]).toBeTruthy();
  });

  test("connection strings are never returned by the API", async ({ request }) => {
    const { body } = await api<any[]>(request, "GET", "/datasources");
    expect(body.length).toBeGreaterThan(0);
    for (const d of body) {
      expect(d).not.toHaveProperty("dsn");
      expect(d).not.toHaveProperty("dsn_enc");
      expect(JSON.stringify(d)).not.toContain("sentinel_ro:");
    }
  });

  test("channel secrets are never returned by the API", async ({ request }) => {
    const { body } = await api<any[]>(request, "GET", "/channels");
    expect(JSON.stringify(body)).not.toMatch(/webhook_url|access_token|verification_token/);
  });

  test("rejects invalid cron schedule", async ({ request }) => {
    const ds = (await api<any[]>(request, "GET", "/datasources")).body[0];
    const r = await api(request, "POST", "/rules", { name: "x", data_source_id: ds.id, sql_text: "SELECT 1", schedule: "every banana" });
    expect(r.status).toBe(400);
    expect((r.body as any).error).toContain("invalid schedule");
  });

  test("rejects unknown database engine", async ({ request }) => {
    const r = await api(request, "POST", "/datasources", { name: "x", driver: "mongodb", dsn: "mongodb://x" });
    expect(r.status).toBe(400);
  });

  test("log watcher rejects SQL injection in identifiers and filters", async ({ request }) => {
    const ds = (await api<any[]>(request, "GET", "/datasources")).body[0];
    for (const bad of [
      { table_name: "logs; DROP TABLE users", id_column: "id" },
      { table_name: "logs", id_column: "id) OR (1=1" },
      { table_name: "logs", id_column: "id", filter_sql: "1=1; DELETE FROM logs" },
      { table_name: "logs", id_column: "id", filter_sql: "1=1 -- comment" },
    ]) {
      const r = await api(request, "POST", "/watchers", { name: "bad", data_source_id: ds.id, ...bad });
      expect(r.status, JSON.stringify(bad)).toBe(400);
    }
  });

  test("probe target validation", async ({ request }) => {
    expect((await api(request, "POST", "/probes", { name: "x", kind: "tcp", target: "no-port" })).status).toBe(400);
    expect((await api(request, "POST", "/probes", { name: "x", kind: "http", target: "ftp://x" })).status).toBe(400);
    expect((await api(request, "POST", "/probes", { name: "x", kind: "icmp", target: "x:1" })).status).toBe(400);
  });

  test("incident list never contains captured row data", async ({ request }) => {
    for (const i of await incidents(request)) {
      expect(i).not.toHaveProperty("payload");
      expect(i).not.toHaveProperty("columns");
    }
  });

  test("unknown / malformed deep-link tokens return 404 (no oracle)", async ({ request }) => {
    expect((await request.get("/api/v1/incidents/link/" + "A".repeat(43))).status()).toBe(404);
    expect((await request.get("/api/v1/incidents/link/short")).status()).toBe(404);
  });

  test("invalid ids are rejected", async ({ request }) => {
    expect((await api(request, "DELETE", "/rules/abc")).status).toBe(400);
    expect((await api(request, "POST", "/incidents/not-a-uuid/ack")).status).toBe(400);
    expect((await api(request, "POST", "/incidents/00000000-0000-0000-0000-000000000000/ack")).status).toBe(404);
  });

  test("data source connection test reports latency", async ({ request }) => {
    const ds = (await api<any[]>(request, "GET", "/datasources")).body.find((d) => d.driver === "postgres");
    const r = await api<any>(request, "POST", `/datasources/${ds.id}/test`);
    expect(r.body.ok).toBe(true);
    expect(r.body.latency_ms).toBeGreaterThanOrEqual(0);
  });
});
