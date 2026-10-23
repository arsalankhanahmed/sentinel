# Sentinel — Test Report

**Build:** v1.0 · **Date:** 23 October 2026 · **Environment:** Linux x86-64, Go 1.24.7, Node 22, PostgreSQL 16 (Core DB + demo ERP DB), Chromium (Playwright 1.63), k6 1.3.0

## Summary

| Suite | Tool | Tests | Result |
|---|---|---|---|
| Backend unit tests (race detector on) | Go `testing` | 48 | ✅ 48 passed |
| API / integration | Playwright | 31 | ✅ 31 passed |
| UI end-to-end (desktop 1440×900) | Playwright + Chromium | 16 | ✅ 16 passed |
| Mobile (Pixel 7 emulation) | Playwright + Chromium | 2 | ✅ 2 passed |
| **Total functional** |  | **97** | ✅ **97 passed, 0 failed** |
| Load & spike | k6 | 3 scenarios, 6 375 requests | ✅ all thresholds met |

The Playwright suite was executed twice back-to-back against the same live stack to prove it is re-runnable (49 / 49 both times).

## 1. Backend unit tests

| Package | Focus | Coverage* |
|---|---|---|
| `internal/vault` | AES-GCM round trip, random nonce, tamper detection, wrong key, base64 key, short ciphertext | 85.7 % |
| `internal/notify` | Teams Adaptive Card contract, RESOLVED card, Zoom markdown + token, WhatsApp Meta template (per recipient), Twilio form, HTTP error → retry, config validation, HTML email, **no raw data in payloads** | 50.8 % |
| `internal/incident` | 1 000 deep-link tokens: length, URL-safety, uniqueness, SHA-256 hash; playbooks per kind; context-aware hints (timeout, auth, 503) | 48.9 % |
| `internal/connectors` | Watcher SQL per dialect (SQL Server / PostgreSQL / MySQL / Oracle), injection rejection, identifier rules, value normalisation, watermark coercion | 37.5 % |
| `internal/engine` | Cron validation, worker-pool throughput, **in-flight de-duplication**, panic survival, non-blocking submit, quoted column lookup | 13.1 % |
| `internal/api` | JWT matrix (none / garbage / expired / wrong key / `alg:none` / viewer / admin), actor extraction, security headers, dev mode | 7.8 % |

\*Line coverage of unit tests only. Store, handlers and engine I/O paths are exercised end-to-end by the Playwright API suite against a real database rather than mocked.

Run: `cd backend && go test -race -cover ./...`

## 2. Playwright API / integration suite (`tests/e2e/specs/api`)

| Spec | Verified |
|---|---|
| `contract.spec.ts` (12) | `/healthz`, `/readyz`; security headers; DSNs and channel secrets never returned; invalid cron / engine rejected; **SQL-injection attempts in watcher identifiers and filters rejected**; probe validation; incident list never contains payload; unknown deep link → 404; invalid ids; connection test latency |
| `detection.spec.ts` (11) | Each planted business exception detected with the **exact row count** (2 / 2 / 3 / 1) and row contents reachable only via deep link; healthy rule raises nothing; persisting problem → one incident with growing occurrences; historical log rows not alerted; DB outage; HTTP 503 outage with IIS guidance; refused TCP port; healthy endpoint raises nothing |
| `lifecycle.spec.ts` (3) | Open → several re-detections with **exactly one** notification → rule fixed → **auto-resolved** → RESOLVED message without link; manual acknowledge/resolve state machine; Run-now queueing |
| `notifications.spec.ts` (5) | Teams Adaptive Card 1.4 with `Action.OpenUrl` deep link; Zoom markdown + verification token; WhatsApp template with 5 parameters; **severity routing** (WhatsApp critical only, Zoom warning+); **compliance: none of the captured business values appear in any notification** |

## 3. Playwright UI suite (`tests/e2e/specs/ui`)

| Spec | Verified |
|---|---|
| `deeplink.spec.ts` (5) | Header, facts, exact rows in grid; filter, sort asc/desc, CSV export content; corrective action checklist; acknowledge; invalid link error page |
| `dashboard.spec.ts` (5) | KPI cards; list with severity/type/source; status tabs; acknowledge from list; sidebar navigation to every page |
| `config.spec.ts` (6) | Add data source via UI + **Test ✓**, DSN never rendered, delete; schedule builder → cron (minutes/daily/weekly/hourly); create rule in UI → Run now → incident with 2 rows → edit/disable → delete; invalid cron error; channel **Send test**; watchers/probes lists |
| `mobile.spec.ts` (2) | Deep link on a phone: rows, Resolve, action plan visible, **no horizontal scroll**; dashboard on mobile |

Run: see [tests/e2e](../tests/e2e) — `npm test` (HTML report in `playwright-report/`, JUnit in `results/junit.xml`).

## 4. Load test (k6)

Scenarios: 50 constant dashboard users (poll every 10 s) · deep-link arrival ramp to 100 req/s · spike to 300 virtual users.

| Metric | Result | Threshold | |
|---|---|---|---|
| Deep-link latency p95 | **2.13 ms** | < 200 ms | ✅ |
| Deep-link latency average | 1.48 ms | — | |
| Dashboard p95 | **41.2 ms** | < 150 ms | ✅ |
| Dashboard p99 | **53.8 ms** | < 400 ms | ✅ |
| All requests p95 | 4.07 ms | — | |
| Failed requests | **0 %** (0 / 6 375) | < 1 % | ✅ |
| Checks | 11 848 / 11 848 passed | — | ✅ |
| Peak virtual users | 300 | — | |

Run: `k6 run tests/load/k6-api.js`

## 5. Defects found and fixed during testing

| # | Finding | Fix |
|---|---|---|
| 1 | A data source or probe deleted while its health check was in flight could raise one "ghost" outage incident after deletion | Engine now re-checks existence before raising, and configuration changes trigger an immediate supervisor reload (`Engine.ConfigChanged`) |
| 2 | WhatsApp adapter could not be pointed at a sandbox, proxy or mock | Added optional `base_url` to WhatsApp config |
| 3 | Watcher/probe/channel deletions were not written to the audit log | Audit entries added |

## 6. Not covered in this run

| Item | Reason / how to cover |
|---|---|
| `docker-compose.demo.yml` end-to-end | No Docker daemon in the test environment; the same steps run in CI (`.github/workflows/ci.yml`). Compose files were validated syntactically. |
| Real Teams / Zoom / Meta / Twilio endpoints | Contract-tested against recorded payload shapes and the mock receiver; verify with **Send test** after configuring real webhooks. |
| SQL Server, MySQL, Oracle as monitored sources | Query generation unit-tested per dialect; live run only against PostgreSQL. Use `demo/appdb/sqlserver` to validate on SQL Server. |
| SMTP delivery | Template unit-tested; use Mailpit in the demo stack to view real emails. |
