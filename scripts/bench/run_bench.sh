#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.dist.yml}"
RESULTS_DIR="scripts/bench/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RAW_JSON="${RESULTS_DIR}/raw_${TIMESTAMP}.json"
REPORT_HTML="${RESULTS_DIR}/report_${TIMESTAMP}.html"

BASE_URL="${BASE_URL:-http://localhost:80}"
NODE1_WS="${NODE1_WS:-ws://localhost:8081}"
NODE2_WS="${NODE2_WS:-ws://localhost:8082}"
USER_COUNT="${USER_COUNT:-50}"
GROUP_SIZE="${GROUP_SIZE:-50}"

CONFIG_FILE="configs/config.docker.yaml"

mkdir -p "${RESULTS_DIR}"

# ── 临时关闭限流 ─────────────────────────────────────────────────────────────
echo "==> Disabling rate limiting for benchmark..."
# 用 sed 临时替换，压测结束后恢复
sed -i 's/rate_limit.enabled: true/rate_limit.enabled: false/' "${CONFIG_FILE}"

cleanup() {
  echo "==> Restoring rate limiting..."
  sed -i 's/rate_limit.enabled: false/rate_limit.enabled: true/' "${CONFIG_FILE}"
  docker compose -f "${COMPOSE_FILE}" restart dipole-node1 dipole-node2 > /dev/null 2>&1 || true
  echo "==> Rate limiting restored."
}
trap cleanup EXIT

# ── 重启节点使配置生效 ────────────────────────────────────────────────────────
echo "==> Restarting dipole nodes..."
docker compose -f "${COMPOSE_FILE}" restart dipole-node1 dipole-node2
echo "==> Waiting for nodes to be ready..."
sleep 8

# ── 检查节点健康 ─────────────────────────────────────────────────────────────
for port in 8081 8082; do
  if ! curl -sf "http://localhost:${port}/health" > /dev/null; then
    echo "ERROR: node on port ${port} is not healthy, aborting."
    exit 1
  fi
done
echo "==> Both nodes healthy."

# ── 运行压测 ─────────────────────────────────────────────────────────────────
echo "==> Running benchmark (results -> ${RAW_JSON})..."
k6 run \
  --out "json=${RAW_JSON}" \
  -e BASE_URL="${BASE_URL}" \
  -e NODE1_WS="${NODE1_WS}" \
  -e NODE2_WS="${NODE2_WS}" \
  -e USER_COUNT="${USER_COUNT}" \
  -e GROUP_SIZE="${GROUP_SIZE}" \
  scripts/bench/bench.js 2>&1 | tee "${RESULTS_DIR}/run_${TIMESTAMP}.log"

# ── 生成 HTML 报告 ────────────────────────────────────────────────────────────
echo "==> Generating HTML report -> ${REPORT_HTML}..."
python3 scripts/bench/gen_report.py "${RAW_JSON}" "${REPORT_HTML}"

echo ""
echo "==> Benchmark complete."
echo "    Raw JSON : ${RAW_JSON}"
echo "    Report   : ${REPORT_HTML}"
echo "    Log      : ${RESULTS_DIR}/run_${TIMESTAMP}.log"
