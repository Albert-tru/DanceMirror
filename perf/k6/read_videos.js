import http from "k6/http";
import { check, sleep } from "k6";
import { randomItem } from "https://jslib.k6.io/k6-utils/1.4.0/index.js";

export const options = {
  scenarios: {
    mix_read: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "15s", target: 50 },
        { duration: "45s", target: 200 },
        { duration: "60s", target: 200 },
        { duration: "15s", target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<250", "p(99)<600"],
  },
};

const BASE = __ENV.BASE_URL || "http://host.docker.internal:8080";
const API  = `${BASE}/api/v1`;
const PHONE = __ENV.PHONE;
const PASS  = __ENV.PASS;

export function setup() {
  // 只登录一次，避免触发 login 限流
  const loginRes = http.post(`${API}/login`, JSON.stringify({ phone: PHONE, password: PASS }), {
    headers: { "Content-Type": "application/json" },
  });
  check(loginRes, { "login 200": (r) => r.status === 200 });

  const token = loginRes.json("token");
  const headers = { Authorization: `Bearer ${token}` };

  // 拉一次视频列表，拿一批 id 做详情压测
  const listRes = http.get(`${API}/videos`, { headers });
  check(listRes, { "list 200": (r) => r.status === 200 });

  const items = listRes.json();
  const ids = Array.isArray(items) ? items.map((v) => v.id).filter(Boolean) : [];
  return { headers, ids };
}

export default function (data) {
  // 70% list，30% detail（你可调整）
  const r = Math.random();

  if (r < 0.7) {
    const res = http.get(`${API}/videos`, { headers: data.headers });
    check(res, { "GET /videos 200": (x) => x.status === 200 });
  } else if (data.ids.length > 0) {
    const id = randomItem(data.ids);
    const res = http.get(`${API}/videos/${id}`, { headers: data.headers });
    check(res, { "GET /videos/{id} 200": (x) => x.status === 200 });
  }

  sleep(0.05);
}