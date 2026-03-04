import http from "k6/http";
import { check, sleep } from "k6";
import { API, ENDPOINTS, loginAndGetToken, authHeaders, getVideoIDs, pick } from "./shared.js";

export const options = {
  scenarios: {
    analyze_write: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "10s", target: 10 },
        { duration: "30s", target: 30 },
        { duration: "30s", target: 30 },
        { duration: "10s", target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.05"],
    http_req_duration: ["p(95)<1200"],
  },
};

export function setup() {
  const token = loginAndGetToken();
  const headers = { ...authHeaders(token), "Content-Type": "application/json" };
  const ids = getVideoIDs(authHeaders(token), 200);
  return { headers, ids };
}

export default function (data) {
  if (!data.ids.length) return;

  const id = pick(data.ids);
  const res = http.post(
    `${API}${ENDPOINTS.analyze(id)}`,
    JSON.stringify({}),
    { headers: data.headers }
  );

  // 常见: 200/202；若限流可能 429
  check(res, {
    "analyze accepted": (r) => [200, 202, 429].includes(r.status),
  });

  sleep(0.1);
}