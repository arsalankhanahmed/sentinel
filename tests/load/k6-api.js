// ============================================================================
// k6 load / performance test for the Sentinel API.
//   k6 run tests/load/k6-api.js
//   k6 run -e SENTINEL_URL=https://sentinel.corp -e SENTINEL_TOKEN=... tests/load/k6-api.js
//
// Scenarios:
//   dashboard  — many engineers watching the incidents dashboard (poll every 10 s)
//   deeplink   — burst of people clicking the same Teams deep link at once
//   spike      — short spike to verify the API degrades gracefully
// Thresholds encode the SLOs; k6 exits non-zero if any is breached (CI-friendly).
// ============================================================================
import http from "k6/http";
import { check, sleep } from "k6";
import { Trend } from "k6/metrics";

const BASE = __ENV.SENTINEL_URL || "http://localhost:8080";
const MOCK = __ENV.MOCK_URL || "http://localhost:9911";
const HEADERS = __ENV.SENTINEL_TOKEN ? { Authorization: `Bearer ${__ENV.SENTINEL_TOKEN}` } : {};
const deeplinkLatency = new Trend("deeplink_latency", true);

export const options = {
  scenarios: {
    dashboard: { executor: "constant-vus", vus: 50, duration: "1m", exec: "dashboard" },
    deeplink: { executor: "ramping-arrival-rate", startRate: 5, timeUnit: "1s", preAllocatedVUs: 50,
      stages: [{ target: 100, duration: "30s" }, { target: 100, duration: "30s" }], exec: "deeplink" },
    spike: { executor: "ramping-vus", startTime: "1m", stages: [{ duration: "10s", target: 300 }, { duration: "20s", target: 0 }], exec: "dashboard" },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    "http_req_duration{scenario:dashboard}": ["p(95)<150", "p(99)<400"],
    deeplink_latency: ["p(95)<200"],
  },
};

export function setup() {
  // Take one real deep link token from the mock Teams inbox (demo stack).
  const msgs = http.get(`${MOCK}/api/messages`).json() || [];
  const m = msgs.find((x) => x.channel === "teams" && x.view && x.view.link);
  return { token: m ? m.view.link.split("/").pop() : null };
}

export function dashboard() {
  const s = http.get(`${BASE}/api/v1/stats`, { headers: HEADERS, tags: { name: "stats" } });
  const l = http.get(`${BASE}/api/v1/incidents?status=open&limit=200`, { headers: HEADERS, tags: { name: "incidents" } });
  check(s, { "stats 200": (r) => r.status === 200 });
  check(l, { "incidents 200": (r) => r.status === 200, "no payload in list": (r) => !r.body.includes('"payload"') });
  sleep(10);
}

export function deeplink(data) {
  if (!data.token) return;
  const r = http.get(`${BASE}/api/v1/incidents/link/${data.token}`, { headers: HEADERS, tags: { name: "deeplink" } });
  deeplinkLatency.add(r.timings.duration);
  check(r, { "deeplink 200": (x) => x.status === 200, "has rows": (x) => x.json("payload") !== null });
}
