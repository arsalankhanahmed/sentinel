// @ts-ignore — plain ESM module shared with the demo CLI
import { seed } from "../../demo/seed/seed.mjs";

export default async function globalSetup() {
  const api = process.env.SENTINEL_URL ?? "http://localhost:8080";
  const mock = process.env.MOCK_URL ?? "http://localhost:9911";

  // wait for the stack
  for (let i = 0; i < 60; i++) {
    try {
      const r = await fetch(`${api}/readyz`);
      if (r.ok) break;
    } catch { /* not up yet */ }
    await new Promise((r) => setTimeout(r, 1000));
  }

  // Make the suite re-runnable: remove previous config, then close leftover incidents.
  const headers: Record<string, string> = process.env.SENTINEL_TOKEN ? { Authorization: `Bearer ${process.env.SENTINEL_TOKEN}` } : {};
  const j = async (path: string, method = "GET") => {
    const r = await fetch(`${api}/api/v1${path}`, { method, headers });
    return method === "GET" ? r.json() : null;
  };
  for (const p of await j("/probes")) await j(`/probes/${p.id}`, "DELETE");
  for (const c of await j("/channels")) await j(`/channels/${c.id}`, "DELETE");
  for (const d of await j("/datasources")) await j(`/datasources/${d.id}`, "DELETE");
  await new Promise((r) => setTimeout(r, 2000)); // let in-flight checks drain
  for (const i of (await j("/incidents?limit=500")) as { id: string; status: string }[]) {
    if (i.status !== "resolved") await j(`/incidents/${i.id}/resolve`, "POST");
  }

  await fetch(`${mock}/api/messages`, { method: "DELETE" });
  await seed({
    api,
    dsn: process.env.DEMO_DSN ?? "postgres://sentinel_ro:sentinel_ro@appdb:5432/erp?sslmode=disable",
    webhook: process.env.MOCK_URL_FROM_SENTINEL ?? mock,
    token: process.env.SENTINEL_TOKEN,
    reset: false,
    fast: true,
    log: (m: string) => console.log(`[setup] ${m}`),
  });
}
