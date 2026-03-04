import http from "k6/http";
import { check, fail } from "k6";

http.setResponseCallback(http.expectedStatuses({ min: 200, max: 299 }, 404));
export const options = {
  scenarios: {
    cache_test: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "10s", target: 50 },
        { duration: "30s", target: 300 },
        { duration: "30s", target: 300 },
        { duration: "10s", target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<100"],
  },
};

const BASE = __ENV.BASE_URL || "http://localhost:8080";
const API = `${BASE}/api/v1`;
const PHONE = __ENV.PHONE || "";
const PASS = __ENV.PASS || "";
const HOT_VIDEO_ID_FROM_ENV = __ENV.HOT_VIDEO_ID || "";

function tokenFrom(res) {
  return res.json("token") || res.json("data.token") || res.json("access_token") || "";
}

function firstVideoIdFrom(res) {
  let arr = res.json();
  if (!Array.isArray(arr)) {
    const data = res.json("data");
    if (Array.isArray(data)) arr = data;
  }
  if (!Array.isArray(arr) || arr.length === 0) return null;
  return arr[0]?.id ?? null;
}

export function setup() {
  if (!PHONE || !PASS) fail("PHONE and PASS are required");

  const loginRes = http.post(
    `${API}/login`,
    JSON.stringify({ phone: PHONE, password: PASS }),
    { headers: { "Content-Type": "application/json" } }
  );
  check(loginRes, { "login 200": (r) => r.status === 200 });

  const token = tokenFrom(loginRes);
  if (!token) fail("token not found in login response");

  const headers = { Authorization: `Bearer ${token}` };

  let hotId = HOT_VIDEO_ID_FROM_ENV ? Number(HOT_VIDEO_ID_FROM_ENV) : null;
  if (!hotId || Number.isNaN(hotId)) {
    const listRes = http.get(`${API}/videos`, { headers });
    check(listRes, { "list 200": (r) => r.status === 200 });
    hotId = firstVideoIdFrom(listRes);
  }

  if (!hotId) fail("No HOT_VIDEO_ID and /videos returned empty");

  return { headers, hotId };
}

export default function (data) {
  if (Math.random() < 0.8) {
    const res = http.get(`${API}/videos/${data.hotId}`, { headers: data.headers });
    check(res, { "hot key 200": (r) => r.status === 200 });
  } else {
    const fakeId = Math.floor(Math.random() * 100000) + 90000000;
    const res = http.get(`${API}/videos/${fakeId}`, { headers: data.headers });
    check(res, { "fake key 404": (r) => r.status === 404 });
  }
}