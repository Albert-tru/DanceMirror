import http from "k6/http";
import { check, sleep } from "k6";
import { API, ENDPOINTS, loginAndGetToken, authHeaders } from "./shared.js";

// k6: open() 必须在 init context
const filePath = __ENV.SAMPLE_FILE || "./perf/k6/assets/sample.mp4";
const bin = open(filePath, "b");

export const options = {
  scenarios: {
    upload_write: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "10s", target: 2 },
        { duration: "20s", target: 5 },
        { duration: "20s", target: 5 },
        { duration: "10s", target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.10"],
    http_req_duration: ["p(95)<5000"],
  },
};

export function setup() {
  const token = loginAndGetToken();
  return { headers: authHeaders(token) };
}

export default function (data) {
  const payload = {
    title: `k6_upload_${Date.now()}`,
    description: "k6 upload test",
    tags: "k6,perf",
    file: http.file(bin, "sample.mp4", "video/mp4"),
  };

  const res = http.post(`${API}${ENDPOINTS.upload}`, payload, {
    headers: data.headers, // multipart header 让 k6 自动处理
    timeout: "120s",
  });

  check(res, {
    "upload status ok": (r) => [200, 201, 202, 429].includes(r.status),
  });

  sleep(0.2);
}