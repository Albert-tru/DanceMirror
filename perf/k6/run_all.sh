#!/usr/bin/env bash
# filepath: /home/dmin/go/DanceMirror/perf/k6/run_all.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
PHONE="${PHONE:-}"
PASS="${PASS:-}"
AUTH_TOKEN="${AUTH_TOKEN:-}"
API_PREFIX="${API_PREFIX:-/api/v1}"
SAMPLE_FILE="${SAMPLE_FILE:-./perf/k6/assets/sample.mp4}"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RESULTS_DIR="$ROOT_DIR/perf/results"
TS="$(date +%Y%m%d_%H%M%S)"
OUT_DIR="$RESULTS_DIR/$TS"

mkdir -p "$OUT_DIR"
cd "$ROOT_DIR"

if [[ -z "$PHONE" && -z "$AUTH_TOKEN" ]]; then
  echo "[ERROR] 需要提供 PHONE+PASS 或 AUTH_TOKEN"
  exit 1
fi

resolve_script() {
  local f="$1"
  if [[ -f "./perf/k6/assets/$f" ]]; then
    echo "./perf/k6/assets/$f"
  elif [[ -f "./perf/k6/$f" ]]; then
    echo "./perf/k6/$f"
  else
    echo ""
  fi
}

run_case() {
  local name="$1"
  local file="$2"
  local script
  script="$(resolve_script "$file")"

  if [[ -z "$script" ]]; then
    echo "[WARN] 跳过 $name，未找到脚本: $file"
    return 0
  fi

  echo ""
  echo "==================== $name ===================="
  k6 run "$script" \
    -e BASE_URL="$BASE_URL" \
    -e API_PREFIX="$API_PREFIX" \
    -e PHONE="$PHONE" \
    -e PASS="$PASS" \
    -e AUTH_TOKEN="$AUTH_TOKEN" \
    -e SAMPLE_FILE="$SAMPLE_FILE" \
    | tee "$OUT_DIR/${name}.txt"
}

echo "[INFO] Root: $ROOT_DIR"
echo "[INFO] Output: $OUT_DIR"

run_case "01_read_videos"   "read_videos.js"
run_case "02_search_videos" "search_videos.js"
run_case "03_get_analysis"  "get_analysis.js"
run_case "04_analyze_video" "analyze_video.js"
run_case "05_crop_video"    "crop_video.js"
run_case "06_upload_video"  "upload_video.js"

echo ""
echo "[DONE] 完成: $OUT_DIR"