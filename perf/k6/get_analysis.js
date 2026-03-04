import http from "k6/http";
import { check, sleep } from "k6";
import { API, ENDPOINTS, loginAndGetToken, authHeaders, getVideoIDs, pick } from "./shared.js";

export const options = {
  scenarios: {
    get_analysis_read: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "10s", target: 20 },
        { duration: "40s", target: 80 },
        { duration: "40s", target: 80 },
        { duration: "10s", target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.02"],
    http_req_duration: ["p(95)<1000", "p(99)<2000"],
  },
};

export function setup() {
  const token = loginAndGetToken();
  const headers = authHeaders(token);
  const ids = getVideoIDs(headers, 200);
  return { headers, ids };
}

export default function (data) {
  if (!data.ids.length) return;

  const id = pick(data.ids);
  const res = http.get(`${API}${ENDPOINTS.analysis(id)}`, { headers: data.headers });

  check(res, {
    "get analysis status ok": (r) => [200, 404].includes(r.status),
  });

  sleep(0.05);
}