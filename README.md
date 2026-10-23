# Sentinel — Zero-Trust Operational Exception & Observability Platform

Plug-and-play monitoring that **never touches your application code**.

📘 [User Manual](docs/USER_MANUAL.md) · 🏗️ [Technical Document](docs/TECHNICAL_DOCUMENT.md) · 🎤 [Demo Guide](docs/DEMO_GUIDE.md) · ✅ [Test Report](docs/TEST_REPORT.md)

![Dashboard](docs/images/01-dashboard.png)

**Quick demo (test data, 2 commands):**

```bash
docker compose -f docker-compose.demo.yml up --build -d
node demo/seed/seed.mjs --api http://localhost:8080 --reset
# dashboard http://localhost:8080 · mock Teams/Zoom/WhatsApp http://localhost:9911 · email http://localhost:8025
```
 Sentinel reads your databases and probes your servers from the outside, captures exceptions as JSON in its own Core DB, and notifies Teams / Zoom / WhatsApp / Email with **only a summary + secure deep link**.

| Module | What it does | How |
|---|---|---|
| **A. Business Data Exceptions** | Scheduled SQL rules; `rows > 0` ⇒ incident, rows captured as JSON | robfig/cron → bounded worker pool → read-only, always-rolled-back tx |
| **B. Runtime App Exceptions (zero-code)** | Watches an app's own error-log table, alerts on *new* rows only | Identity-column watermark (`WHERE id > @cursor`), index seek every ~30 s |
| **C. Infra & DB Outages** | `SELECT 1` heartbeat on every data source; TCP / HTTP probes (IIS 80/443, SQL 1433…) | 1 s supervisor tick, consecutive-failure threshold, auto-resolve on recovery |

## Architecture

```
                 ┌──────────────────────── Sentinel (single Go binary) ────────────────────────┐
 React SPA ────► │ Gin API (JWT, RBAC, audit) ─► Store (pgx) ─► PostgreSQL Core DB              │
 (Vite+TW)       │                                                                              │
                 │ cron (Module A) ─┐                                                           │
                 │ 1s supervisor ───┼─► WorkerPool (bounded, keyed, non-blocking) ─► Engines     │
                 │  (B + C)         ┘        │                                                  │
                 │                          ▼                                                   │
                 │                Incident Service (dedupe by fingerprint, deep-link token)     │
                 │                          ▼                                                   │
                 │                Dispatcher (queue + retry/backoff) ─► Teams│Zoom│WhatsApp│SMTP │
                 └──────────────────────────────────────────────────────────────────────────────┘
                        ▲ read-only, pooled (max 4 conns)          ▲ TCP / HTTP
                 SQL Server · PostgreSQL · MySQL · Oracle     IIS / VMs / any port
```

## Layout

```
sentinel/
├── backend/
│   ├── cmd/server/main.go          # wiring, graceful shutdown
│   ├── cmd/token/main.go           # mint API tokens (dev / service accounts)
│   ├── migrations/001_init.sql     # PostgreSQL schema (embedded, idempotent)
│   └── internal/
│       ├── config/                 # env-based config
│       ├── vault/                  # AES-256-GCM for DSNs & channel secrets
│       ├── store/                  # Core DB repository (pgx)
│       ├── connectors/             # external DB pool, dialect SQL, row→JSON
│       ├── engine/                 # worker pool, cron, query runner, log watcher, heartbeat
│       ├── incident/               # dedupe, deep-link tokens, corrective-action playbooks
│       ├── notify/                 # dispatcher + Teams / Zoom / WhatsApp / Email adapters
│       └── api/                    # Gin router, handlers, zero-trust middleware
├── frontend/src/
│   ├── App.tsx                     # routes (lazy-loaded), auth gate, /i/:token deep link
│   ├── pages/IncidentPage.tsx      # deep-linked grid + corrective action plan
│   ├── pages/{Dashboard,Rules,DataSources,Watchers,Probes,Channels,Login}.tsx
│   └── components/{DataGrid,ScheduleBuilder,CrudPage,Layout,Badges}.tsx
├── demo/                           # test data + demo tooling
│   ├── appdb/postgres|sqlserver/   # demo ERP DB with planted exceptions
│   ├── seed/seed.mjs               # loads demo config through the API
│   ├── mock-webhook/server.py      # fake Teams/Zoom/WhatsApp inbox + UP/DOWN targets
│   └── scripts/                    # simulate-errors, fix-data, reset-data
├── tests/
│   ├── e2e/                        # Playwright: API, UI, mobile, docs screenshots
│   └── load/k6-api.js              # k6 load / spike test with SLO thresholds
├── docs/                           # user manual, technical doc, demo guide, test report
├── .github/workflows/ci.yml        # Go tests → UI build → E2E on demo stack → k6
├── Dockerfile                      # node → go → distroless (non-root)
├── docker-compose.yml              # production-style
├── docker-compose.demo.yml         # full demo stack
└── .env.example
```

## Run

```bash
cp .env.example .env
# set MASTER_KEY (openssl rand -base64 32) and JWT_SECRET
docker compose up --build            # http://localhost:8080
```

Local dev:

```bash
# backend
cd backend && go mod tidy
AUTH_DISABLED=true MASTER_KEY=dev go run ./cmd/server
# frontend (proxies /api → :8080)
cd frontend && npm install && npm run dev
# real tokens
JWT_SECRET=... go run ./cmd/token -sub you@corp.com -role admin
```

## Tests

| Suite | Command |
|---|---|
| Go unit (race) | `cd backend && go test -race -cover ./...` |
| Playwright API + UI + mobile | `cd tests/e2e && npm install && npx playwright install chromium && npm test` |
| Regenerate doc screenshots | `cd tests/e2e && npm run docs:screenshots` |
| k6 load | `k6 run tests/load/k6-api.js` |

97 functional tests pass; k6 deep-link p95 2.1 ms with 0 % errors — see the [Test Report](docs/TEST_REPORT.md).

## Zero-trust & compliance design

- **Every** API call (including deep-link resolution) needs a valid JWT; config endpoints need `role=admin`. Swap the HS256 keyfunc in `api/middleware.go` for your IdP's JWKS (Entra ID / Okta / Keycloak).
- Deep links carry a 256-bit random token; only its **SHA-256** is stored, links expire (`LINK_TTL`), unknown/expired return an identical 404, `Referrer-Policy: no-referrer` everywhere.
- Notifications contain metadata + link only. Captured rows live only in the Core DB.
- DSNs and channel secrets encrypted with AES-256-GCM; never serialised by the API.
- Rule SQL runs in a read-only transaction (where supported) that is **always rolled back**. Still give Sentinel a least-privilege, read-only login (`db_datareader` / `SELECT` grants).
- Every config change and incident view is written to `audit_log`.

## Behaviour notes

- **No alert storms:** one active incident per fingerprint (partial unique index). A persisting condition increments `occurrences`; notifications fire only when an incident opens and when it auto-resolves.
- **Log watcher** starts from the table's current max id (no history replay); `cursor_value` only moves forward. The id column must be monotonically increasing (IDENTITY / sequence).
- **Probes** alert after `PROBE_FAIL_THRESHOLD` consecutive failures; DB connections are dropped after a failed heartbeat so the next check dials fresh.
- **Schedules:** standard 5-field cron, optional seconds, or `@every 30s` / `@hourly` (UTC).

## Scaling

The binary is stateless except for the scheduler. For HA run 2 replicas with leader election (e.g. `pg_try_advisory_lock` around `engine.Start`) so only the leader schedules; all replicas serve the API. For very high incident volume, partition `incidents` by month on `opened_at`.

## Teams / WhatsApp setup

- **Teams:** create a *Workflows* "Post to a channel when a webhook request is received" flow; use its URL as `webhook_url`.
- **WhatsApp (Meta):** create an approved template with 5 body params: `{{1}}` severity, `{{2}}` title, `{{3}}` source, `{{4}}` time, `{{5}}` link.
