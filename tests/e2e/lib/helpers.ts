import { expect, type APIRequestContext } from "@playwright/test";

export const MOCK = process.env.MOCK_URL ?? "http://localhost:9911";

export type Incident = {
  id: string; kind: string; title: string; severity: string; status: string;
  row_count: number; occurrences: number; source_name: string; payload?: unknown; columns?: string[];
  corrective_action: string;
};

export type MockMessage = {
  channel: string; received_at: number; body: any; auth: string;
  view: { title: string; badge: string; lines: string[]; link: string };
};

export async function incidents(request: APIRequestContext, status = ""): Promise<Incident[]> {
  const r = await request.get(`/api/v1/incidents?limit=500${status ? `&status=${status}` : ""}`);
  expect(r.ok()).toBeTruthy();
  return r.json();
}

/** Polls until an incident with the given title (and optional status) exists. */
export async function waitForIncident(request: APIRequestContext, title: string, status = "", timeout = 60_000) {
  let found: Incident | undefined;
  await expect
    .poll(async () => {
      found = (await incidents(request, status)).find((i) => i.title === title);
      return !!found;
    }, { timeout, intervals: [1000] })
    .toBe(true);
  return found!;
}

export async function mockMessages(request: APIRequestContext): Promise<MockMessage[]> {
  const r = await request.get(`${MOCK}/api/messages`);
  return r.json();
}

/** Finds the deep link Sentinel delivered for an incident title (from the mock Teams channel). */
export async function deepLinkPath(request: APIRequestContext, title: string, timeout = 45_000): Promise<string> {
  let link = "";
  await expect
    .poll(async () => {
      const m = (await mockMessages(request)).find((x) => x.channel === "teams" && x.view.title === title && x.view.link);
      link = m?.view.link ?? "";
      return link;
    }, { timeout, intervals: [1000] })
    .not.toBe("");
  return new URL(link).pathname; // PUBLIC_BASE_URL host may differ from the test host
}

export async function api<T = any>(request: APIRequestContext, method: string, path: string, data?: unknown): Promise<{ status: number; body: T }> {
  const r = await request.fetch(`/api/v1${path}`, { method, data });
  const text = await r.text();
  return { status: r.status(), body: text ? JSON.parse(text) : (undefined as T) };
}

export const DEMO_DSN = process.env.DEMO_DSN ?? "postgres://sentinel_ro:sentinel_ro@appdb:5432/erp?sslmode=disable";
