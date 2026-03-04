#!/usr/bin/env bash
#! filepath: /home/dmin/go/DanceMirror/perf/k6/run_ab_compare.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
PHONE="${PHONE:-}"
PASS="${PASS:-}"

if [[ -z "$PHONE" || -z "$PASS" ]]; then
  echo "Usage: PHONE=xxx PASS=xxx bash perf/k6/run_ab_compare.sh"
  exit 1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

OUT_DIR="perf/results/ab_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$OUT_DIR"

db_select() {
  docker exec -i dancemirror-db mariadb -uroot -pMySQL666 -Nse "SHOW GLOBAL STATUS LIKE 'Com_select';" dancemirror | awk '{print $2}'
}
redis_hits() {
  docker exec -i dancemirror-redis redis-cli INFO stats | awk -F: '/keyspace_hits/{gsub("\r","",$2);print $2}'
}
redis_miss() {
  docker exec -i dancemirror-redis redis-cli INFO stats | awk -F: '/keyspace_misses/{gsub("\r","",$2);print $2}'
}

run_group() {
  local group="$1"   # A or B
  local bypass="$2"  # 1 or 0
  local warmup="$3"  # yes or no

  echo "=== Group ${group} (CACHE_BYPASS=${bypass}) ==="
  CACHE_BYPASS="$bypass" docker compose up -d --build app >/dev/null
  sleep 5

  # 可选：清redis，保证组间可比
  docker exec -i dancemirror-redis redis-cli FLUSHALL >/dev/null

  local db_before hit_before miss_before db_after hit_after miss_after
  db_before="$(db_select)"
  hit_before="$(redis_hits)"
  miss_before="$(redis_miss)"

  if [[ "$warmup" == "yes" ]]; then
    k6 run perf/k6/test_cache.js \
      -e BASE_URL="$BASE_URL" \
      -e PHONE="$PHONE" \
      -e PASS="$PASS" > /dev/null
  fi

  k6 run perf/k6/test_cache.js \
    -e BASE_URL="$BASE_URL" \
    -e PHONE="$PHONE" \
    -e PASS="$PASS" | tee "$OUT_DIR/cache_${group}.txt"

  db_after="$(db_select)"
  hit_after="$(redis_hits)"
  miss_after="$(redis_miss)"

  {
    echo "group=${group}"
    echo "cache_bypass=${bypass}"
    echo "db_before=${db_before}"
    echo "db_after=${db_after}"
    echo "db_delta=$((db_after-db_before))"
    echo "redis_hit_before=${hit_before}"
    echo "redis_hit_after=${hit_after}"
    echo "redis_hit_delta=$((hit_after-hit_before))"
    echo "redis_miss_before=${miss_before}"
    echo "redis_miss_after=${miss_after}"
    echo "redis_miss_delta=$((miss_after-miss_before))"
  } | tee "$OUT_DIR/metrics_${group}.txt"
}

# A: 优化前（不走缓存）
run_group "A_before" "1" "no"

# B: 优化后（走缓存，预热）
run_group "B_after" "0" "yes"

echo "Done. Output: $OUT_DIR"