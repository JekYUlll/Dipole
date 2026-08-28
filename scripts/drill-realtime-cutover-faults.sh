#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/deploy/realtime/cutover-fault-drill.compose.yml"
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-dipole-c3-cutover-fault-$RANDOM}"
REPORT_FILE="${DIPOLE_CUTOVER_DRILL_REPORT:-/tmp/dipole-c3-cutover-fault-report.json}"

pick_port() {
  local port
  while true; do
    port="$(shuf -i 20000-45000 -n 1)"
    if ! ss -ltnH | awk '{print $4}' | grep -Eq "[:.]${port}$"; then
      printf '%s\n' "${port}"
      return
    fi
  done
}

export DIPOLE_CUTOVER_DRILL_REDIS_PORT="${DIPOLE_CUTOVER_DRILL_REDIS_PORT:-$(pick_port)}"
export DIPOLE_CUTOVER_DRILL_KAFKA_PORT="${DIPOLE_CUTOVER_DRILL_KAFKA_PORT:-$(pick_port)}"

compose() {
  docker compose -p "${PROJECT_NAME}" -f "${COMPOSE_FILE}" "$@"
}

cleanup() {
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

mkdir -p "$(dirname "${REPORT_FILE}")"
compose config --quiet
compose up -d --wait

export DIPOLE_CUTOVER_DRILL_REDIS_ADDR="127.0.0.1:${DIPOLE_CUTOVER_DRILL_REDIS_PORT}"
export DIPOLE_CUTOVER_DRILL_KAFKA_ADDR="127.0.0.1:${DIPOLE_CUTOVER_DRILL_KAFKA_PORT}"
export DIPOLE_CUTOVER_DRILL_REPORT="${REPORT_FILE}"
export DIPOLE_CUTOVER_DRILL_REVISION="$(git -C "${ROOT_DIR}" rev-parse HEAD)"
export DIPOLE_CUTOVER_DRILL_REDIS_IMAGE="$(docker inspect --format '{{.Image}}' "$(compose ps -q redis)")"
export DIPOLE_CUTOVER_DRILL_KAFKA_IMAGE="$(docker inspect --format '{{.Image}}' "$(compose ps -q kafka)")"

env LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu \
  go test -race -tags=integration ./internal/realtime/delivery \
  -run '^TestRealtimeCutoverFaultDrill$' -count=1 -v

jq -e '
  .schema_version == "dipole.realtime.cutover-fault-drill.v1" and
  (.git_revision | test("^[a-f0-9]{40}$")) and
  (.redis_image | test("^sha256:[a-f0-9]{64}$")) and
  (.kafka_image | test("^sha256:[a-f0-9]{64}$")) and
  (.controller_crash_artifact_sha256 | test("^[a-f0-9]{64}$")) and
  (.final_journal_head_sha256 | test("^[a-f0-9]{64}$")) and
  .controller_crash_recovered == true and
  .redis_outage_blocked == true and
  .kafka_rebalance_blocked == true and
  .final_state == "completed"
' "${REPORT_FILE}" >/dev/null

echo "C3 cutover fault drill passed: ${REPORT_FILE}"
