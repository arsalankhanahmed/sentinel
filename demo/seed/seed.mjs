#!/usr/bin/env node
// ============================================================================
// Sentinel demo seeder — registers data sources, rules, log watcher, probes and
// notification channels through the public REST API (no direct DB writes).
// Zero dependencies (Node 18+). Also imported by the Playwright global setup.
//
//   node demo/seed/seed.mjs \
//     --api http://localhost:8080 \
//     --dsn "postgres://sentinel_ro:sentinel_ro@appdb:5432/erp?sslmode=disable" \
//     --webhook http://mock-webhook:9911 \
//     [--token <JWT>] [--reset] [--no-outages] [--fast]
// ============================================================================

export const DEMO_RULES = [
  {
    name: "Invoices posted without GL entry",
    severity: "critical",
    sql_text: `SELECT i.invoice_no, i.order_id, i.amount, i.posted_at
FROM invoices i
WHERE i.posted
  AND NOT EXISTS (SELECT 1 FROM gl_entries g WHERE g.invoice_no = i.invoice_no)
ORDER BY i.posted_at`,
    corrective_action: `Check the ERP → GL integration job log for failures at the posted_at time.
Re-run the GL posting job for the listed invoice numbers.
Verify receivables (1200) and sales (4000) balances after re-posting.
Inform Finance Controller if the amount exceeds PKR 1,000,000.`,
  },
  {
    name: "Negative stock on hand",
    severity: "warning",
    sql_text: `SELECT inv.sku, p.name AS product, inv.warehouse, inv.qty_on_hand
FROM inventory inv JOIN products p ON p.sku = inv.sku
WHERE inv.qty_on_hand < 0
ORDER BY inv.qty_on_hand`,
    corrective_action: `Freeze further sales of the listed SKUs in the affected warehouse.
Run a physical cycle count and post a stock adjustment.
Review goods-receipt postings for the last 48 hours (likely missing GRN).`,
  },
  {
    name: "Orders stuck in PROCESSING > 2 hours",
    severity: "critical",
    sql_text: `SELECT o.order_id, c.name AS customer, o.total_amount, o.created_at,
       round(extract(epoch FROM now() - o.created_at) / 3600, 1) AS hours_waiting
FROM orders o JOIN customers c ON c.customer_id = o.customer_id
WHERE o.status = 'PROCESSING' AND o.created_at < now() - interval '2 hours'
ORDER BY o.created_at`,
    corrective_action: `Check the fulfilment worker / message queue for stuck messages.
Contact the warehouse supervisor for the listed orders.
Notify affected customers if SLA (4h) will be breached.`,
  },
  {
    name: "Duplicate payments against same invoice",
    severity: "critical",
    sql_text: `SELECT p.invoice_no, count(*) AS payment_count, sum(p.amount) AS total_paid,
       string_agg(p.reference, ', ') AS references
FROM payments p
GROUP BY p.invoice_no
HAVING count(*) > 1`,
    corrective_action: `Hold settlement for the listed invoices immediately.
Confirm with the bank which reference is the duplicate.
Raise a refund / credit note and record it against the invoice.`,
  },
  {
    name: "Shipped orders without invoice",
    severity: "warning",
    sql_text: `SELECT o.order_id, o.total_amount, o.updated_at
FROM orders o
WHERE o.status = 'SHIPPED'
  AND NOT EXISTS (SELECT 1 FROM invoices i WHERE i.order_id = o.order_id)`,
    corrective_action: "",
  },
];

export const DEMO_WATCHER = {
  name: "OrderPortal & BillingSvc errors",
  table_name: "app_error_log",
  id_column: "log_id",
  select_columns: "app_name, level, message, stack_trace, user_name, logged_at",
  filter_sql: "level IN ('ERROR','FATAL')",
  interval_sec: 10,
  severity: "critical",
  corrective_action: "",
};

function args(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (!a.startsWith("--")) continue;
    const k = a.slice(2);
    const v = argv[i + 1] && !argv[i + 1].startsWith("--") ? argv[++i] : true;
    out[k] = v;
  }
  return out;
}

export async function seed({ api, dsn, webhook, token, reset = false, outages = true, fast = false, log = console.log }) {
  const base = api.replace(/\/$/, "") + "/api/v1";
  const call = async (method, path, body) => {
    const res = await fetch(base + path, {
      method,
      headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
      body: body ? JSON.stringify(body) : undefined,
    });
    const text = await res.text();
    if (!res.ok) throw new Error(`${method} ${path} → ${res.status} ${text}`);
    return text ? JSON.parse(text) : null;
  };

  if (reset) {
    for (const p of await call("GET", "/probes")) await call("DELETE", `/probes/${p.id}`);
    for (const c of await call("GET", "/channels")) await call("DELETE", `/channels/${c.id}`);
    for (const d of await call("GET", "/datasources")) await call("DELETE", `/datasources/${d.id}`); // cascades rules/watchers
    log("✓ reset existing configuration");
  }

  const schedule = fast ? "@every 5s" : "@every 30s";
  const hook = webhook.replace(/\/$/, "");
  const ids = { rules: [], probes: [], channels: [] };

  ids.channels.push((await call("POST", "/channels", {
    name: "Ops Teams channel (mock)", kind: "teams", min_severity: "info",
    config: { webhook_url: `${hook}/hook/teams` },
  })).id);
  ids.channels.push((await call("POST", "/channels", {
    name: "Zoom #it-ops (mock)", kind: "zoom", min_severity: "warning",
    config: { webhook_url: `${hook}/hook/zoom`, verification_token: "demo-token" },
  })).id);
  ids.channels.push((await call("POST", "/channels", {
    name: "On-call WhatsApp (mock)", kind: "whatsapp", min_severity: "critical",
    config: { provider: "meta", base_url: `${hook}/hook/whatsapp`, phone_number_id: "1234567890",
      access_token: "demo", template: "incident_alert", recipients: ["923001234567"] },
  })).id);
  if (!fast) {
    ids.channels.push((await call("POST", "/channels", {
      name: "Ops mailbox", kind: "email", min_severity: "critical", config: { to: ["oncall@demo.local"] },
    })).id);
  }
  log(`✓ ${ids.channels.length} notification channels`);

  ids.erp = (await call("POST", "/datasources", { name: "ERP Production (demo)", driver: "postgres", dsn })).id;
  if (outages) {
    ids.offline = (await call("POST", "/datasources", {
      name: "Finance SQL Server (offline)", driver: "sqlserver",
      dsn: "sqlserver://sentinel_ro:x@10.255.255.1:1433?database=Finance&connection+timeout=3",
    })).id;
  }
  log(`✓ data sources (erp=${ids.erp}${outages ? `, offline=${ids.offline}` : ""})`);

  for (const r of DEMO_RULES) {
    ids.rules.push((await call("POST", "/rules", { ...r, data_source_id: ids.erp, schedule, max_rows: 500 })).id);
  }
  log(`✓ ${ids.rules.length} business exception rules (${schedule})`);

  ids.watcher = (await call("POST", "/watchers", { ...DEMO_WATCHER, data_source_id: ids.erp, interval_sec: fast ? 5 : 10 })).id;
  log("✓ runtime log watcher on app_error_log");

  const iv = fast ? 5 : 30;
  ids.probes.push((await call("POST", "/probes", { name: "Notification gateway health", kind: "http", target: `${hook}/health`, interval_sec: iv })).id);
  if (outages) {
    ids.probes.push((await call("POST", "/probes", { name: "IIS web02 – OrderPortal", kind: "http", target: `${hook}/down`, interval_sec: iv, timeout_ms: 3000 })).id);
    const h = new URL(hook).hostname;
    ids.probes.push((await call("POST", "/probes", { name: "Legacy SQL Server (DR) :1433", kind: "tcp", target: `${h}:1`, interval_sec: iv, timeout_ms: 1500 })).id);
  }
  log(`✓ ${ids.probes.length} infrastructure probes`);
  return ids;
}

// CLI entry
if (import.meta.url === `file://${process.argv[1]}`) {
  const a = args(process.argv.slice(2));
  seed({
    api: a.api ?? "http://localhost:8080",
    dsn: a.dsn ?? "postgres://sentinel_ro:sentinel_ro@appdb:5432/erp?sslmode=disable",
    webhook: a.webhook ?? "http://mock-webhook:9911",
    token: a.token ?? process.env.SENTINEL_TOKEN,
    reset: !!a.reset,
    outages: !a["no-outages"],
    fast: !!a.fast,
  })
    .then(() => console.log("\nDemo configuration loaded. Open the dashboard and the mock inbox."))
    .catch((e) => { console.error("Seed failed:", e.message); process.exit(1); });
}
