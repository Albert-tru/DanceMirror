//// filepath: /home/dmin/go/DanceMirror/perf/k6/crop_video.js
import http from "k6/http";
http.setResponseCallback(
  http.expectedStatuses({ min: 200, max: 299 }, 429) // 把 429 当成 expected_response
);
import { Counter } from "k6/metrics";
import { check, sleep } from "k6";
import { API, ENDPOINTS, loginAndGetToken, authHeaders, getVideoIDs, pick } from "./shared.js";


const cropSuccess = new Counter("crop_success");
const DEBUG = __ENV.DEBUG === "1";


export const options = {
  scenarios: {
    crop_write: {
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
  const ids = getVideoIDs(authHeaders(token), 50); // 从 /videos 拉 50 个可用 ID
  return { headers, ids };
}

export default function (data) {
  const id = pick(data.ids);

  const payload = { startTime: 1.0, endTime: 3.0 };

  const res = http.post(
    `${API}${ENDPOINTS.crop(id)}`,
    JSON.stringify(payload),
    { headers: data.headers }
  );

  if (DEBUG) {
    console.log(`crop id=${id}, status=${res.status}, body=${res.body}`);
  }

  if ([200, 202].includes(res.status)) {
    cropSuccess.add(1);  // 真正排队成功的裁剪任务
  }

  check(res, {
  "crop accepted": (r) => [200, 202, 429].includes(r.status),
});

  sleep(0.1);
}

