# Sentinel — User Manual

**Zero-Trust Operational Exception & Observability Platform**
Version 1.0 · October 2026

---

## Contents

1. [What Sentinel does](#1-what-sentinel-does)
2. [Who uses it](#2-who-uses-it)
3. [Signing in](#3-signing-in)
4. [The Incidents dashboard](#4-the-incidents-dashboard)
5. [Opening an incident from a notification (deep link)](#5-opening-an-incident-from-a-notification-deep-link)
6. [Working an incident](#6-working-an-incident)
7. [Configuring data sources](#7-configuring-data-sources)
8. [Business exception rules (Module A)](#8-business-exception-rules-module-a)
9. [Runtime log watchers (Module B)](#9-runtime-log-watchers-module-b)
10. [Infrastructure probes (Module C)](#10-infrastructure-probes-module-c)
11. [Notification channels](#11-notification-channels)
12. [Writing good rules — examples](#12-writing-good-rules--examples)
13. [Frequently asked questions](#13-frequently-asked-questions)
14. [Glossary](#14-glossary)

---

## 1. What Sentinel does

Sentinel watches your business systems **from the outside** and tells the right people when something goes wrong — without installing any agent, SDK or code inside your applications.

It finds three kinds of problems:

| Type | Example | How Sentinel finds it |
|---|---|---|
| **Business Data Exception** | Invoices posted without a GL entry; negative stock; duplicate payments | Runs your SQL query on a schedule. If it returns rows, those rows are the exception. |
| **Runtime Application Exception** | `NullReferenceException`, SQL deadlock, bank gateway timeout | Reads new rows from the application's own error-log table. |
| **Infrastructure Outage** | SQL Server unreachable, IIS site returns 503, port 1433 closed | Checks every database with `SELECT 1` and pings servers/ports. |

When a problem is found, Sentinel:

1. Saves the exact rows involved (securely, inside Sentinel only).
2. Sends a short alert to **Microsoft Teams, Zoom, WhatsApp and/or Email** with a secure **Open Incident** link.
3. Shows the captured rows and a **Recommended Corrective Action Plan** when you open that link.
4. **Closes the incident automatically** when the problem goes away, and tells the channel it is resolved.

> **Why the alert has no data in it:** For security and compliance, business data (invoice numbers, amounts, customer names) never travels inside Teams/WhatsApp/email. You must sign in to Sentinel to see it.

---

## 2. Who uses it

| Role | Typical tasks | Access |
|---|---|---|
| **Viewer** (support engineer, finance user, on-call) | Open incidents from alerts, review captured rows, acknowledge, resolve | Incidents only |
| **Admin** (DBA, platform owner) | Add data sources, write rules, configure watchers, probes and channels | Everything |

---

## 3. Signing in

Open your Sentinel address (for example `https://sentinel.yourcompany.com`).

- If your company uses single sign-on, you are redirected to your normal company login.
- In the standalone setup, paste the **access token** you were given and press **Sign in**.

If you clicked an incident link before signing in, Sentinel brings you straight back to that incident after you sign in.

---

## 4. The Incidents dashboard

![Incidents dashboard](images/01-dashboard.png)

**Summary cards (top).** Count of *currently open* incidents per type, plus everything opened in the last 24 hours. A red number means attention is needed.

**Status tabs.** `open` (default) · `acknowledged` · `resolved` · `all`.

**Incident list.** Each row shows:

| Column | Meaning |
|---|---|
| Severity | `CRITICAL` (act now) · `WARNING` (act today) · `INFO` |
| Incident | Rule or check name. "seen 12× · last 3s ago" means the problem is still present and Sentinel has re-checked it 12 times — **without** sending 12 alerts. |
| Type | Business Exception · Runtime Exception · DB Outage · Endpoint Outage |
| Source | The database or server where it was detected |
| Rows | Number of exception rows captured |
| Opened | When it was first detected |
| Status | open / acknowledged / resolved |

**Ack** tells your team "I'm on it". **Resolve** closes it manually.
The page refreshes itself every 10 seconds.

---

## 5. Opening an incident from a notification (deep link)

Every alert contains an **Open Incident** button or link, for example:

`https://sentinel.yourcompany.com/i/kg5VX21fIJ5lZGOFFQa8NbEd3kk6rDbjYQD9hRz2ZXI`

![Incident page opened from a deep link](images/02-incident-deeplink.png)

The page shows:

- **Header** — severity, status, type, short ID, title and a one-line summary.
- **Facts** — source, when it opened, when last seen, rows captured, occurrences.
- **Captured exception rows** — the *exact* rows your query (or log table) returned at the time.
- **Recommended corrective action plan** — step-by-step guidance. Tick steps off as you go (the ticks are for your own tracking on that screen).

The links are long and random on purpose — they cannot be guessed. They expire after 30 days (configurable). A wrong or expired link shows *"Incident link not found"*.

The page works on a phone, so you can act directly from Teams or WhatsApp mobile:

<img src="images/10-mobile-deeplink.png" width="300" alt="Mobile view">

---

## 6. Working an incident

### 6.1 Using the captured rows grid

| Action | How |
|---|---|
| Search | Type in **Filter rows…** — searches every column |
| Sort | Click a column header (click again to reverse) |
| Export | **Export CSV** downloads the currently filtered rows |
| See long values | Hover over a cell to see the full value |

### 6.2 Recommended workflow

1. Open the link from the alert.
2. Click **Acknowledge** so colleagues know someone is working on it.
3. Follow the **corrective action plan**.
4. Fix the data or system **in the owning application**.
5. Wait — Sentinel re-checks automatically. When the query returns no rows (or the server responds again) the incident becomes **resolved** by itself and a *RESOLVED* message is posted to the channel.
6. Use **Resolve** only if you want to close it manually (for example a false positive while you correct the rule).

### 6.3 Outage incidents

![Endpoint outage](images/03-incident-outage.png)

For outages, the captured row contains the target, HTTP status or error, latency and check time. The action plan adapts to the error — for example a **503** first suggests checking the IIS Application Pool; a **timeout** suggests checking for blocking/deadlocks or firewall drops; a **login failure** suggests checking the monitoring account.

---

## 7. Configuring data sources

*Admin only.* **Data Sources** are the databases Sentinel can read.

![Data sources](images/04-data-sources.png)

1. Enter a **Name** people will recognise (e.g. *ERP Production*).
2. Choose the **Engine**: SQL Server, PostgreSQL, MySQL or Oracle.
3. Paste the **Connection string**. Examples:

| Engine | Connection string |
|---|---|
| SQL Server | `sqlserver://sentinel_ro:PASSWORD@db01:1433?database=ERP&encrypt=true` |
| PostgreSQL | `postgres://sentinel_ro:PASSWORD@pg01:5432/app?sslmode=require` |
| MySQL | `sentinel_ro:PASSWORD@tcp(mysql01:3306)/app?parseTime=true&tls=true` |
| Oracle | `oracle://sentinel_ro:PASSWORD@ora01:1521/ORCLPDB1` |

4. Leave **Outage heartbeat** ticked to get a *DB Outage* alert if this database becomes unreachable.
5. Click **Add**, then **Test** — you should see `✓ 4 ms`.

> **Security.** The connection string is encrypted immediately and is never shown again — not in the screen, not in the API. To change it, delete and re-add the source.
> Always use a **read-only** database login (`db_datareader` on SQL Server, `SELECT` grants elsewhere).

---

## 8. Business exception rules (Module A)

*Admin only.* A rule is a SQL query that returns the **bad records**. No rows = healthy. Any rows = exception.

![Rules](images/05-rules.png)

| Field | What to enter |
|---|---|
| Rule name | A business-friendly title. This becomes the alert title. |
| Data source | Where to run the query |
| SQL script | A `SELECT` that returns only the problem rows. Include the columns a person needs to act (IDs, amounts, dates). |
| Schedule | **Minutes** (every N minutes) · **Hourly** (at minute M) · **Daily** / **Weekly** (at HH:MM UTC) · **Custom cron** (e.g. `*/10 * * * *`, `0 7 * * 1-5`, `@every 30s`) |
| Severity | Critical / Warning / Info — controls which channels are notified |
| Max rows captured | Upper limit stored per run (default 500). The incident is marked *truncated* if more rows exist. |
| Corrective action plan | One step per line. Leave empty to use Sentinel's built-in plan. |
| Enabled | Untick to pause the rule without deleting it |

**Rule list columns:** *Last run* shows when it ran and how long the query took. *Result* shows `ok` (green, no rows), `exception: N rows` (amber) or an `error:` (red — usually a SQL mistake or permission problem).

**Run now** runs the rule immediately. **Edit** loads it into the form; **Save changes** applies it straight away — no restart needed.

> Rules run in a read-only transaction that is always rolled back, so even a mistaken `UPDATE` will not change your data. Still, write `SELECT` statements only.

---

## 9. Runtime log watchers (Module B)

*Admin only.* Most applications already write errors to a database table (log4net, NLog, Serilog, Elmah, custom). A **log watcher** reads only the *new* rows of that table and raises an incident for each new batch of errors.

![Log watchers](images/06-log-watchers.png)

| Field | Example |
|---|---|
| Log table | `dbo.ErrorLog` · `app_error_log` |
| Identity column | `Id` / `LogId` — must be an always-increasing number (IDENTITY / sequence) |
| Columns | `*` or `Level, Message, StackTrace, UserName, LoggedAt` |
| Filter (optional) | `Level IN ('ERROR','FATAL')` |
| Poll interval | 30 seconds is typical; minimum 5 |

**Watermark** shows the last identity value processed. When a watcher is first created it shows *initialising* and starts from the **current** end of the table, so old history is not re-alerted.

---

## 10. Infrastructure probes (Module C)

*Admin only.* Probes check that servers and services respond.

![Probes](images/07-probes.png)

| Probe type | Target example | Fails when |
|---|---|---|
| **HTTP(S) status** | `https://web01.corp/health` | No response, timeout, or status ≠ *Expected HTTP status* (default 200) |
| **TCP handshake** | `db01.corp:1433`, `app02:3389` | Connection refused or timed out |

To avoid false alarms from a single network blip, Sentinel alerts only after **two consecutive failures** (configurable by your administrator) and resolves automatically on the first success.

Every data source with *Outage heartbeat* enabled is also checked with `SELECT 1` every 60 seconds.

---

## 11. Notification channels

*Admin only.*

![Channels](images/08-channels.png)

| Channel | What arrives | Config (JSON) |
|---|---|---|
| **Microsoft Teams** | Adaptive Card: type, title, source, timestamp, rows, **Open Incident** button | `{"webhook_url":"https://…"}` (Teams *Workflows* webhook) |
| **Zoom Team Chat** | Formatted message with summary and link | `{"webhook_url":"https://integrations.zoom.us/chat/webhooks/incomingwebhook/…","verification_token":"…"}` |
| **WhatsApp** | Approved template message with link | Meta: `{"provider":"meta","phone_number_id":"…","access_token":"…","template":"incident_alert","recipients":["923001234567"]}` · Twilio: `{"provider":"twilio","account_sid":"AC…","auth_token":"…","from":"whatsapp:+1…","recipients":["whatsapp:+92…"]}` |
| **Email** | Mobile-friendly HTML email with button | `{"to":["oncall@company.com"]}` |

**Minimum severity** routes alerts: e.g. WhatsApp = *Critical* only (wake people up), Teams = *Info* and above.

**Send test** delivers a sample message so you can confirm the setup.

What a channel receives (demo inbox):

![Notifications](images/09-mock-inbox.png)

Each incident is notified **once when it opens** and **once when it resolves** — never on every re-check.

---

## 12. Writing good rules — examples

**Invoices posted without a GL entry (SQL Server)**
```sql
SELECT i.InvoiceNo, i.Amount, i.PostedAt
FROM dbo.Invoices i
WHERE i.Posted = 1
  AND NOT EXISTS (SELECT 1 FROM dbo.GLEntries g WHERE g.InvoiceNo = i.InvoiceNo);
```

**Orders stuck in processing for more than 2 hours**
```sql
SELECT OrderId, CustomerId, TotalAmount, CreatedAt
FROM dbo.Orders
WHERE Status = 'PROCESSING' AND CreatedAt < DATEADD(HOUR, -2, SYSUTCDATETIME());
```

**Duplicate payments**
```sql
SELECT InvoiceNo, COUNT(*) AS Payments, SUM(Amount) AS TotalPaid
FROM dbo.Payments GROUP BY InvoiceNo HAVING COUNT(*) > 1;
```

**Tips**

- Return only what a person needs to fix the problem — IDs, amounts, dates, owner.
- Make the query *self-healing*: once data is corrected it must return 0 rows, so the incident auto-resolves.
- Use indexed columns in `WHERE`; aim for queries under one second.
- Prefer `TOP`/`LIMIT` inside the query if the result can be huge.
- Schedule according to business impact — every 5 minutes for payments, daily for month-end checks.

---

## 13. Frequently asked questions

**I received an alert but the link says "not found".** The link has expired (30 days by default) or was copied incompletely. Open the dashboard and find the incident there.

**The same problem keeps happening — will I get spammed?** No. While the problem persists, the incident stays open and only its *occurrences* counter grows.

**I fixed the data but the incident is still open.** It closes at the next scheduled run. Use **Run now** on the rule to check immediately.

**A rule shows `error:`.** Usually a SQL syntax error, a missing permission for the read-only login, or a timeout (default 60 s). Fix the SQL and **Save changes**.

**Does Sentinel change anything in my databases?** No. It only reads, inside a transaction that is always rolled back.

**Why don't alerts contain the actual rows?** Compliance: chat and email systems are outside your security boundary.

---

## 14. Glossary

| Term | Meaning |
|---|---|
| Incident | One detected problem, from first detection until resolved |
| Deep link | The secure, unguessable URL in an alert that opens one incident |
| Fingerprint | Internal identity of a problem, used to avoid duplicate incidents |
| Occurrences | How many times a still-open problem was detected again |
| Watermark | The last log row id a watcher has processed |
| Heartbeat | The `SELECT 1` connectivity check on each database |
| Auto-resolve | Automatic closure once the condition clears |
