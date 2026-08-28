#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/deploy/realtime/cutover-fault-drill.compose.yml"
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-dipole-c3-cutover-fault-$RANDOM}"
REPORT_FILE="${DIPOLE_CUTOVER_DRILL_REPORT:-/tmp/dipole-c3-cutover-fault-report.json}"
if [[ "${REPORT_FILE}" != /* ]]; then
  REPORT_FILE="${ROOT_DIR}/${REPORT_FILE}"
fi
CONTROLLER_REPORT_FILE="${DIPOLE_CUTOVER_CONTROLLER_DRILL_REPORT:-${REPORT_FILE%.json}-controller.json}"
if [[ "${CONTROLLER_REPORT_FILE}" != /* ]]; then
  CONTROLLER_REPORT_FILE="${ROOT_DIR}/${CONTROLLER_REPORT_FILE}"
fi
CPP_COMPILER="${CXX:-/usr/bin/g++}"
CPP_COMPILER_ID="$(basename "${CPP_COMPILER}" | tr -cd '[:alnum:]_.+-')"
CPP_BUILD_DIR="${DIPOLE_CPP_BUILD_DIR:-/tmp/dipole-cpp-realtime-build-${CPP_COMPILER_ID}}"

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
mkdir -p "$(dirname "${CONTROLLER_REPORT_FILE}")"
if [[ "${DIPOLE_CUTOVER_DRILL_BUILD_CPP:-1}" == "1" ]]; then
  CXX="${CPP_COMPILER}" DIPOLE_CPP_BUILD_DIR="${CPP_BUILD_DIR}" \
    "${ROOT_DIR}/scripts/check-cpp-realtime.sh"
fi
CPP_BINARY="${DIPOLE_CUTOVER_DRILL_CPP_BINARY:-${CPP_BUILD_DIR}/dipole-realtime-delivery}"
if [[ ! -x "${CPP_BINARY}" ]]; then
  echo "C++ realtime primary binary is missing: ${CPP_BINARY}" >&2
  exit 1
fi
compose config --quiet
compose up -d --wait

export DIPOLE_CUTOVER_DRILL_REDIS_ADDR="127.0.0.1:${DIPOLE_CUTOVER_DRILL_REDIS_PORT}"
export DIPOLE_CUTOVER_DRILL_KAFKA_ADDR="127.0.0.1:${DIPOLE_CUTOVER_DRILL_KAFKA_PORT}"
export DIPOLE_CUTOVER_DRILL_REPORT="${REPORT_FILE}"
export DIPOLE_CUTOVER_CONTROLLER_DRILL_REPORT="${CONTROLLER_REPORT_FILE}"
export DIPOLE_CUTOVER_CONTROLLER_DRILL_REDIS_ADDR="${DIPOLE_CUTOVER_DRILL_REDIS_ADDR}"
export DIPOLE_CUTOVER_DRILL_CPP_BINARY="${CPP_BINARY}"
export DIPOLE_CUTOVER_DRILL_GOLDEN_DIR="${ROOT_DIR}/api/proto/dipole/delivery/v1/testdata"
export DIPOLE_CUTOVER_DRILL_REVISION="$(git -C "${ROOT_DIR}" rev-parse HEAD)"
export DIPOLE_CUTOVER_DRILL_REDIS_IMAGE="$(docker inspect --format '{{.Image}}' "$(compose ps -q redis)")"
export DIPOLE_CUTOVER_DRILL_KAFKA_IMAGE="$(docker inspect --format '{{.Image}}' "$(compose ps -q kafka)")"

env LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu \
  go test -race -tags=integration ./internal/realtime/delivery \
  -run '^(TestRealtimeCutoverFaultDrill|TestCutoverControllerRealProcessReplacement)$' -count=1 -v

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
  .cpp_primary_ready == true and
  .cpp_primary_stopped_cleanly == true and
  (.cpp_primary_binary_sha256 | test("^[a-f0-9]{64}$")) and
  (.cpp_primary_instance_id | test("^cpp-[a-z0-9-]+$")) and
  (.cpp_primary_group_id | startswith("dipole-realtime-primary-")) and
  (.cpp_primary_observation_key | contains(":observation:realtime-delivery:")) and
  (.cpp_primary_observation_sha256 | test("^[a-f0-9]{64}$")) and
  .expired_freeze_rolled_back == true and
  .rollback_final_sequence == 7 and
  (.rollback_journal_head_sha256 | test("^[a-f0-9]{64}$")) and
  .final_state == "completed"
' "${REPORT_FILE}" >/dev/null

jq -e '
  .schema_version == "dipole.realtime.cutover-controller-drill.v1" and
  (.git_revision | test("^[a-f0-9]{40}$")) and
  .redis_mode == "redis" and
  .process_a_exit_code == 91 and
  .process_a_sequence == 1 and
  .pre_expiry_blocked == true and
  .process_b_resumed == true and
  .final_state == "completed" and
  .final_sequence == 6 and
  (.final_journal_head_sha256 | test("^[a-f0-9]{64}$")) and
  .control_lease_ttl_ms == 5000
' "${CONTROLLER_REPORT_FILE}" >/dev/null

echo "C3 cutover fault drill passed: ${REPORT_FILE}"
echo "C3 cutover controller drill passed: ${CONTROLLER_REPORT_FILE}"
