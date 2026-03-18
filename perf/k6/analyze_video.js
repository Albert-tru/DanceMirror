import http from "k6/http";
import { check, sleep } from "k6";
import { Counter } from "k6/metrics";
import { API, ENDPOINTS, loginAndGetToken, authHeaders, getVideoIDs, pick } from "./shared.js";

const analyzeSuccess = new Counter("analyze_success");
const DEBUG = __ENV.DEBUG === "1";

export const options = {
  scenarios: {
    analyze_write: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "20s", target: 5 },
        { duration: "20s", target: 10 },
        { duration: "20s", target: 20 },
        { duration: "20s", target: 30 },
        { duration: "20s", target: 40 },
        { duration: "20s", target: 0 },
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
  const ids = getVideoIDs(authHeaders(token), 500); // 获取 500 个视频 ID
  return { headers, ids };
}

export default function (data) {
  if (!data.ids.length) return;

  // 随机选择一个视频 ID
  const id = pick(data.ids);

  const res = http.post(
    `${API}${ENDPOINTS.analyze(id)}`,
    JSON.stringify({}),
    { headers: data.headers }
  );

  if (DEBUG) {
    console.log(`analyze id=${id}, status=${res.status}, body=${res.body}`);
  }

  if ([200, 202].includes(res.status)) {
    analyzeSuccess.add(1); // 统计成功的分析请求
  }

  check(res, {
    "analyze accepted": (r) => [200, 202, 429].includes(r.status),
  });

  sleep(0.1);
}