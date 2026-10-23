# Sentinel — Demo & Presentation Guide

A ready-made, repeatable demo you can run on a laptop in about 5 minutes and present in 15–20 minutes. Everything here uses **test data only**.

---

## 1. What the demo contains

| Piece | What it is | Where |
|---|---|---|
| **Demo ERP database** | A fake company ERP (customers in Karachi, Lahore, Islamabad…; orders, invoices, GL entries, payments, inventory, application error log) with **deliberately planted problems** | `demo/appdb/postgres/` (also a SQL Server version in `demo/appdb/sqlserver/`) |
| **Demo configuration** | 2 data sources, 5 business rules, 1 log watcher, 3 probes, 4 channels | `demo/seed/seed.mjs` |
| **Mock notification inbox** | Pretends to be Teams / Zoom / WhatsApp and shows the messages live in a browser | `demo/mock-webhook/` → http://localhost:9911 |
| **Email inbox** | Mailpit catches the HTML emails | http://localhost:8025 |
| **Live simulators** | Scripts to inject app errors, fix the data, and reset it | `demo/scripts/` |

### Planted problems in the test data

| Rule | Planted rows | Severity | Expected alert channels |
|---|---|---|---|
| Invoices posted without GL entry | 2 (`INV-90001`, `INV-90002`) | Critical | Teams, Zoom, WhatsApp, Email |
| Negative stock on hand | 2 (`SKU-1004 @ KHI-01`, `SKU-1005 @ LHE-01`) | Warning | Teams, Zoom |
| Orders stuck in PROCESSING > 2 hours | 3 (one 20-minute order is correctly **not** flagged) | Critical | all |
| Duplicate payments against same invoice | 1 (`BANK-DUP-7781`) | Critical | all |
| Shipped orders without invoice | **0** — shows a healthy rule (green `ok`) | Warning | none |

| Infrastructure check | State | Result |
|---|---|---|
| Finance SQL Server (offline) | unreachable IP | DB Outage |
| IIS web02 – OrderPortal | returns HTTP 503 | Endpoint Outage (IIS App Pool guidance) |
| Legacy SQL Server (DR) :1433 | port refused | Endpoint Outage |
| Notification gateway health | HTTP 200 | healthy — no alert |

---

## 2. Start the demo

Prerequisites: Docker Desktop, Node.js 18+.

```bash
git clone https://github.com/arsalankhanahmed/sentinel.git
cd sentinel

# 1. start everything (Sentinel, Core DB, demo ERP DB, mock inbox, Mailpit)
docker compose -f docker-compose.demo.yml up --build -d

# 2. load the demo configuration (rules, watcher, probes, channels)
node demo/seed/seed.mjs --api http://localhost:8080 --reset
```

Open three browser tabs:

| Tab | URL |
|---|---|
| Sentinel dashboard | http://localhost:8080 |
| Mock Teams / Zoom / WhatsApp inbox | http://localhost:9911 |
| Email inbox | http://localhost:8025 |

Within ~30 seconds incidents appear on the dashboard and messages appear in the inboxes.

> Tip: for a faster-paced demo use `--fast` (rules every 5 s instead of 30 s).

---

## 3. Suggested presentation script (15–20 min)

### Act 1 — The problem (2 min)
"Our business problems hide in data: invoices that never reached the ledger, payments taken twice, orders stuck. Today we find out from angry customers or month-end. Traditional APM tools need SDKs inside every application — not possible for legacy WebForms / third-party systems."

### Act 2 — Zero-code setup (4 min)
1. **Data Sources** → show *ERP Production (demo)*, click **Test** → `✓ 3 ms`. Point out the connection string is encrypted and never shown again.
2. **Exception Rules** → open *Invoices posted without GL entry*. "This is just a SELECT that returns the bad records. No code in the ERP."
3. Show the **Schedule** selector (Minutes / Hourly / Daily / Weekly / Cron).

### Act 3 — Detection and alert (4 min)
1. Switch to the **mock inbox** tab: Teams Adaptive Card, Zoom message, WhatsApp template.
2. "Notice: no invoice numbers, no amounts — only a secure link. Compliance-safe."
3. Open **Mailpit** → show the mobile-friendly HTML email.

### Act 4 — The deep link (4 min)
1. Click **Open Incident** on the Teams card.
2. Show the captured rows grid → filter `90002`, sort by amount, **Export CSV**.
3. Show the **Recommended Corrective Action Plan**, tick a step, click **Acknowledge**.
4. Open the same link on your phone (or narrow the browser) — fully usable on mobile.

### Act 5 — Runtime errors without an SDK (2 min)
```bash
./demo/scripts/simulate-errors.sh
```
Within ~10 s a *Runtime Exception* incident appears with `NullReferenceException` / deadlock rows read straight from the app's own log table. WARN rows are filtered out.

### Act 6 — Infrastructure (1 min)
Show *Database unreachable: Finance SQL Server* and *IIS web02* (HTTP 503 → action plan suggests recycling the App Pool).

### Act 7 — Self-healing (2 min)
```bash
./demo/scripts/fix-data.sh
```
At the next run the business incidents switch to **resolved** by themselves and **RESOLVED** messages arrive in the inbox. "No one has to remember to close tickets."

### Close
"No agents. No code changes. Read-only. Lightning fast (2 ms deep-link response at p95). Plugs into Teams, Zoom, WhatsApp and email."

---

## 4. Useful commands during a demo

| Goal | Command |
|---|---|
| Inject 3 app errors | `./demo/scripts/simulate-errors.sh` |
| Continuous errors every 20 s | `./demo/scripts/simulate-errors.sh --loop 20` |
| Fix all business data (auto-resolve) | `./demo/scripts/fix-data.sh` |
| Re-plant all problems | `./demo/scripts/reset-data.sh` |
| Reload demo configuration from scratch | `node demo/seed/seed.mjs --api http://localhost:8080 --reset` |
| Clear the mock inbox | `curl -X DELETE http://localhost:9911/api/messages` |
| Stop everything | `docker compose -f docker-compose.demo.yml down -v` |

Without Docker (local PostgreSQL), set `DEMO_DB_URL=postgres://demo:demo@localhost:55432/erp` before running the scripts.

---

## 5. Presenting against SQL Server / IIS instead

1. Run `demo/appdb/sqlserver/01_schema_and_seed.sql` on a SQL Server instance (creates `ErpDemo` and the read-only login `sentinel_ro`).
2. In Sentinel add a data source: SQL Server, `sqlserver://sentinel_ro:Sentinel_ro%232026@<host>:1433?database=ErpDemo`.
3. Paste the example rules from the bottom of that SQL file.
4. Add a log watcher on `dbo.AppErrorLog`, id column `LogId`, filter `Level IN ('ERROR','FATAL')`.
5. Add an HTTP probe to one of your IIS sites and a TCP probe to `<sqlhost>:1433`.

---

## 6. Recorded walkthrough video

A narrated 2-minute recording of the full script is in [`docs/media/sentinel-demo.mp4`](media/sentinel-demo.mp4) — use it when a live demo is not possible.

To re-record (e.g. after UI changes), start and seed the demo stack, then:

```bash
pip install playwright && playwright install chromium
python demo/recording/record_demo.py --out sentinel-demo.webm
ffmpeg -i sentinel-demo.webm -c:v libx264 -crf 23 -pix_fmt yuv420p -movflags +faststart sentinel-demo.mp4
```

The script drives the browser with a visible cursor and on-screen captions, injects application errors, fixes the data and shows auto-resolution.

## 7. Troubleshooting the demo

| Symptom | Fix |
|---|---|
| No incidents after 1 minute | `docker compose -f docker-compose.demo.yml logs sentinel` — check the seed ran against the right `--api` URL |
| Inbox empty | Check **Channels** → *Send test*. Inside Docker the webhook host is `mock-webhook`, which is the seeder default. |
| Emails missing | Mailpit must be running; email channel only receives **Critical** |
| Data already "fixed" from a previous demo | `./demo/scripts/reset-data.sh` |
