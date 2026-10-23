# Sentinel E2E tests (Playwright)

```bash
npm install
npx playwright install chromium
# against the demo stack (docker-compose.demo.yml):
MOCK_URL_FROM_SENTINEL=http://mock-webhook:9911 npm test
```

| Variable | Default | Meaning |
|---|---|---|
| `SENTINEL_URL` | `http://localhost:8080` | Sentinel base URL |
| `MOCK_URL` | `http://localhost:9911` | Mock inbox as seen from the test runner |
| `MOCK_URL_FROM_SENTINEL` | = `MOCK_URL` | Mock inbox as seen from Sentinel (`http://mock-webhook:9911` in Docker) |
| `DEMO_DSN` | `postgres://sentinel_ro:sentinel_ro@appdb:5432/erp?sslmode=disable` | Demo ERP DB as seen from Sentinel |
| `SENTINEL_TOKEN` | – | Bearer token when auth is enabled |

Projects: `api` · `ui-desktop` · `ui-mobile` · `docs` (screenshots for the user manual).
The global setup resets Sentinel's configuration and loads the demo setup in fast mode, so the suite is safe to re-run.
