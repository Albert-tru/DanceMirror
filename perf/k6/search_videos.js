import http from "k6/http";
import { check, sleep } from "k6";
import { API, ENDPOINTS, loginAndGetToken, authHeaders, pick } from "./shared.js";

const KEYWORDS = (__ENV.KEYWORDS || "dance,hiphop,mirror,practice")
  .split(",")
  .map((s) => s.trim())
  .filter(Boolean);

export const options = {
  scenarios: {
    search_read: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "15s", target: 30 },
        { duration: "45s", target: 100 },
        { duration: "45s", target: 100 },
        { duration: "15s", target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<800", "p(99)<1500"],
  },
};

export function setup() {
  const token = loginAndGetToken();
  return { headers: authHeaders(token) };
}

export default function (data) {
  const kw = pick(KEYWORDS);
  const res = http.get(`${API}${ENDPOINTS.searchVideos}?keyword=${encodeURIComponent(kw)}`, {
    headers: data.headers,
  });

  check(res, { "search status ok": (r) => r.status === 200 });
  sleep(0.05);
}