import { test, expect } from "@playwright/test";
import { mockMessages, waitForIncident } from "../../lib/helpers";

test.describe("Notification dispatcher (Teams / Zoom / WhatsApp)", () => {
  test.beforeAll(async ({ request }) => {
    await waitForIncident(request, "Invoices posted without GL entry");
    await waitForIncident(request, "Negative stock on hand");
  });

  test("Teams receives an Adaptive Card with an Open Incident deep link", async ({ request }) => {
    await expect.poll(async () => (await mockMessages(request)).some((m) => m.channel === "teams"), { timeout: 30_000 }).toBe(true);
    const m = (await mockMessages(request)).find((x) => x.channel === "teams" && x.view.title === "Invoices posted without GL entry")!;
    const card = m.body.attachments[0];
    expect(card.contentType).toBe("application/vnd.microsoft.card.adaptive");
    expect(card.content.version).toBe("1.4");
    expect(card.content.actions[0].type).toBe("Action.OpenUrl");
    expect(card.content.actions[0].url).toMatch(/\/i\/[A-Za-z0-9_-]{43}$/);
    const facts = card.content.body.find((b: any) => b.type === "FactSet").facts.map((f: any) => f.title);
    expect(facts).toEqual(expect.arrayContaining(["Exception", "Source", "Timestamp"]));
  });

  test("Zoom receives markdown with the deep link and verification token", async ({ request }) => {
    await expect.poll(async () => (await mockMessages(request)).some((m) => m.channel === "zoom"), { timeout: 30_000 }).toBe(true);
    const z = (await mockMessages(request)).find((x) => x.channel === "zoom")!;
    expect(z.auth).toBe("demo-token");
    expect(z.body.is_markdown_support).toBe(true);
    expect(z.body.content.body[0].text).toMatch(/<http.*\/i\/.*\|Open incident/);
  });

  test("WhatsApp (Meta template) receives the 5 template parameters", async ({ request }) => {
    await expect.poll(async () => (await mockMessages(request)).some((m) => m.channel === "whatsapp"), { timeout: 30_000 }).toBe(true);
    const w = (await mockMessages(request)).find((x) => x.channel === "whatsapp")!;
    expect(w.body.messaging_product).toBe("whatsapp");
    expect(w.body.template.name).toBe("incident_alert");
    expect(w.body.template.components[0].parameters).toHaveLength(5);
    expect(w.auth).toBe("Bearer demo");
  });

  test("severity routing: WhatsApp only gets CRITICAL, Zoom gets WARNING+", async ({ request }) => {
    const msgs = await mockMessages(request);
    expect(msgs.filter((m) => m.channel === "whatsapp").every((m) => ["CRITICAL", "RESOLVED"].includes(m.view.badge))).toBe(true);
    // "Negative stock on hand" is a WARNING → Teams + Zoom, not WhatsApp
    expect(msgs.some((m) => m.channel === "zoom" && m.body.content.head.text.includes("Negative stock on hand"))).toBe(true);
    expect(msgs.some((m) => m.channel === "whatsapp" && m.view.title === "Negative stock on hand")).toBe(false);
  });

  test("COMPLIANCE: no captured business data ever leaves in a notification", async ({ request }) => {
    const all = JSON.stringify((await mockMessages(request)).map((m) => m.body));
    for (const secretValue of ["INV-90001", "INV-90002", "BANK-DUP-7781", "SKU-1004", "1180000", "Punjab Textiles"]) {
      expect(all, `leaked ${secretValue}`).not.toContain(secretValue);
    }
  });
});
