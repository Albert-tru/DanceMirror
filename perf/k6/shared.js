import http from "k6/http";
import { check, fail } from "k6";

export const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";
export const API_PREFIX = __ENV.API_PREFIX || "/api/v1";
export const API = `${BASE_URL}${API_PREFIX}`;

export const PHONE = __ENV.PHONE || "";
export const PASS = __ENV.PASS || "";
export const AUTH_TOKEN = __ENV.AUTH_TOKEN || "";

export const ENDPOINTS = {
  login: "/login",
  listVideos: "/videos",
  searchVideos: "/videos/search",
  analyze: (id) => `/videos/${id}/analyze`,
  analysis: (id) => `/videos/${id}/analysis`,
  crop: (id) => `/videos/${id}/crop`,
  upload: "/videos",
};

export function authHeaders(token) {
  return { Authorization: `Bearer ${token}` };
}

export function parseToken(res) {
  const direct = res.json("token");
  if (direct) return direct;
  const nested = res.json("data.token");
  if (nested) return nested;
  const alt = res.json("access_token");
  if (alt) return alt;
  return "";
}

export function loginAndGetToken() {
  if (AUTH_TOKEN) return AUTH_TOKEN;
  if (!PHONE || !PASS) fail("PHONE/PASS or AUTH_TOKEN is required");

  const res = http.post(
    `${API}${ENDPOINTS.login}`,
    JSON.stringify({ phone: PHONE, password: PASS }),
    { headers: { "Content-Type": "application/json" } }
  );

  check(res, { "login status 200": (r) => r.status === 200 });
  const token = parseToken(res);
  if (!token) fail(`token not found in login response, status=${res.status}`);
  return token;
}

export function getVideoIDs(headers, maxIDs = 100) {
  const res = http.get(`${API}${ENDPOINTS.listVideos}`, { headers });
  check(res, { "list videos 200": (r) => r.status === 200 });

  let items = res.json();
  if (!Array.isArray(items)) {
    const maybeData = res.json("data");
    if (Array.isArray(maybeData)) items = maybeData;
  }

  if (!Array.isArray(items)) return [];
  return items.map((x) => x?.id).filter(Boolean).slice(0, maxIDs);
}

export function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}