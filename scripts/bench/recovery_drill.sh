#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.dist.yml}"
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-dipole-c1}"
TARGET_SERVICE="${TARGET_SERVICE:-dipole-node2}"
RECOVERY_TIMEOUT_SECONDS="${RECOVERY_TIMEOUT_SECONDS:-60}"
CONSUMER_STABLE_SECONDS="${CONSUMER_STABLE_SECONDS:-5}"
KAFKA_SERVICE="${KAFKA_SERVICE:-kafka}"
KAFKA_CONSUMER_GROUP="${KAFKA_CONSUMER_GROUP:-dipole-consumer}"
RESULTS_DIR="${RESULTS_DIR:-scripts/bench/results}"
RUN_ID="${RUN_ID:-c1-recovery-$(date -u +%Y%m%dT%H%M%SZ)}"
POST_RUN_ID="${RUN_ID}-post"
BASE_URL="${BASE_URL:-http://127.0.0.1:18081}"
NODE1_WS="${NODE1_WS:-ws://127.0.0.1:18081}"
NODE2_WS="${NODE2_WS:-ws://127.0.0.1:18082}"
NODE1_HEALTH_URL="${NODE1_HEALTH_URL:-http://127.0.0.1:18081/health}"
NODE2_HEALTH_URL="${NODE2_HEALTH_URL:-http://127.0.0.1:18082/health}"
NODE3_HEALTH_URL="${NODE3_HEALTH_URL:-http://127.0.0.1:18083/health}"
EVIDENCE_JSON="${RESULTS_DIR}/${RUN_ID}.recovery-evidence.json"
REPORT_JSON="${RESULTS_DIR}/${RUN_ID}.recovery-report.json"
POST_BASELINE_JSON="${RESULTS_DIR}/${POST_RUN_ID}.baseline.json"
recovery_required=false

compose() {
  docker compose \
    --project-directory "${ROOT_DIR}" \
    --project-name "${COMPOSE_PROJECT_NAME}" \
    -f "${ROOT_DIR}/${COMPOSE_FILE}" \
    "$@"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "required command not found: $1" >&2
    exit 1
  }
}

now_rfc3339() {
  date -u +%Y-%m-%dT%H:%M:%S.%6NZ
}

wait_ready() {
  local deadline=$((SECONDS + RECOVERY_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    if curl --fail --silent "${TARGET_HEALTH_URL}" >/dev/null; then
      return 0
    fi
    sleep 0.1
  done
  echo "${TARGET_SERVICE} did not recover within ${RECOVERY_TIMEOUT_SECONDS}s" >&2
  compose ps >&2
  return 1
}

wait_consumer_group_ready() {
  local expected_members="${1:-}"
  local deadline=$((SECONDS + RECOVERY_TIMEOUT_SECONDS))
  local state members current previous_members="" stable_seconds=0
  while (( SECONDS < deadline )); do
    current="$(compose exec -T "${KAFKA_SERVICE}" \
      /opt/kafka/bin/kafka-consumer-groups.sh \
      --bootstrap-server 127.0.0.1:9092 \
      --group "${KAFKA_CONSUMER_GROUP}" \
      --describe --state 2>/dev/null \
      | awk -v group="${KAFKA_CONSUMER_GROUP}" '$1 == group {print $(NF-1), $NF}')"
    read -r state members <<<"${current}"
    if [[ "${state:-}" == "Stable" && "${members:-}" =~ ^[1-9][0-9]*$ \
      && ( -z "${expected_members}" || "${members}" == "${expected_members}" ) ]]; then
      if [[ "${members}" == "${previous_members}" ]]; then
        stable_seconds=$((stable_seconds + 1))
      else
        stable_seconds=1
        previous_members="${members}"
      fi
      if (( stable_seconds >= CONSUMER_STABLE_SECONDS )); then
        printf '%s\n' "${members}"
        return 0
      fi
    else
      stable_seconds=0
      previous_members=""
    fi
    sleep 1
  done
  echo "consumer group ${KAFKA_CONSUMER_GROUP} did not become stable" >&2
  return 1
}

snapshot_target() {
  local container_id pid image_id revision source_dirty
  container_id="$(compose ps -q "${TARGET_SERVICE}")"
  if [[ ! "${container_id}" =~ ^[0-9a-f]{64}$ ]]; then
    echo "${TARGET_SERVICE} has no unique full container ID" >&2
    return 1
  fi
  pid="$(docker inspect --format '{{.State.Pid}}' "${container_id}")"
  image_id="$(docker inspect --format '{{.Image}}' "${container_id}")"
  revision="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "${image_id}")"
  source_dirty="$(docker image inspect --format '{{index .Config.Labels "io.dipole.source.dirty"}}' "${image_id}")"
  if [[ ! "${pid}" =~ ^[1-9][0-9]*$ ]]; then
    echo "${TARGET_SERVICE} has no running host PID" >&2
    return 1
  fi
  if [[ "${source_dirty}" != "false" ]]; then
    echo "${TARGET_SERVICE} image source must be clean" >&2
    return 1
  fi
  jq -cn \
    --arg container_id "${container_id}" \
    --arg image_id "${image_id}" \
    --arg revision "${revision}" \
    --argjson pid "${pid}" \
    '{container_id: $container_id, image_id: $image_id, revision: $revision, source_dirty: false, pid: $pid}'
}

recover_target() {
  local status=$?
  trap - EXIT
  if [[ "${recovery_required}" == "true" ]]; then
    echo "attempting emergency recovery for ${TARGET_SERVICE}" >&2
    compose start "${TARGET_SERVICE}" >/dev/null 2>&1 || true
  fi
  exit "${status}"
}

for command in curl date docker git jq python3; do
  require_command "${command}"
done

case "${TARGET_SERVICE}" in
  dipole-node1|dipole-node2|dipole-node3) ;;
  *)
    echo "unsupported TARGET_SERVICE: ${TARGET_SERVICE}" >&2
    exit 1
    ;;
esac

case "${TARGET_SERVICE}" in
  dipole-node1) TARGET_HEALTH_URL="${TARGET_HEALTH_URL:-${NODE1_HEALTH_URL}}" ;;
  dipole-node2) TARGET_HEALTH_URL="${TARGET_HEALTH_URL:-${NODE2_HEALTH_URL}}" ;;
  dipole-node3) TARGET_HEALTH_URL="${TARGET_HEALTH_URL:-${NODE3_HEALTH_URL}}" ;;
esac

if [[ ! "${RUN_ID}" =~ ^[A-Za-z0-9._-]+$ ]]; then
  echo "RUN_ID may contain only letters, numbers, dot, underscore, and hyphen" >&2
  exit 1
fi
if [[ ! "${CONSUMER_STABLE_SECONDS}" =~ ^[1-9][0-9]*$ ]]; then
  echo "CONSUMER_STABLE_SECONDS must be a positive integer" >&2
  exit 1
fi
if [[ -n "$(git -C "${ROOT_DIR}" status --porcelain)" ]]; then
  echo "recovery source tree has changes; commit them before collecting evidence" >&2
  exit 1
fi

cd "${ROOT_DIR}"
mkdir -p "${RESULTS_DIR}"
expected_revision="$(git rev-parse HEAD)"
curl --fail --silent "${TARGET_HEALTH_URL}" >/dev/null
pre_fault_member_count="$(wait_consumer_group_ready)"
before="$(snapshot_target)"
if [[ "$(jq -r '.revision' <<<"${before}")" != "${expected_revision}" ]]; then
  echo "target image revision does not match recovery source" >&2
  exit 1
fi

trap recover_target EXIT
fault_started_at="$(now_rfc3339)"
recovery_required=true
compose stop "${TARGET_SERVICE}"
if curl --fail --silent "${TARGET_HEALTH_URL}" >/dev/null 2>&1; then
  echo "target health remained available after stop" >&2
  exit 1
fi
unavailable_observed_at="$(now_rfc3339)"
start_requested_at="$(now_rfc3339)"
compose start "${TARGET_SERVICE}"
wait_ready
post_fault_member_count="$(wait_consumer_group_ready "${pre_fault_member_count}")"
ready_observed_at="$(now_rfc3339)"
after="$(snapshot_target)"
recovery_required=false

jq -n \
  --arg run_id "${RUN_ID}" \
  --arg target_service "${TARGET_SERVICE}" \
  --arg expected_revision "${expected_revision}" \
  --arg consumer_group "${KAFKA_CONSUMER_GROUP}" \
  --argjson stable_member_count "${post_fault_member_count}" \
  --arg fault_started_at "${fault_started_at}" \
  --arg unavailable_observed_at "${unavailable_observed_at}" \
  --arg start_requested_at "${start_requested_at}" \
  --arg ready_observed_at "${ready_observed_at}" \
  --argjson before "${before}" \
  --argjson after "${after}" \
  '{
    schema_version: "dipole.performance.recovery-evidence.v1",
    run_id: $run_id,
    target_service: $target_service,
    expected_revision: $expected_revision,
    fault: {
      action: "stop_start",
      consumer_group: $consumer_group,
      stable_member_count: $stable_member_count
    },
    timeline: {
      fault_started_at: $fault_started_at,
      unavailable_observed_at: $unavailable_observed_at,
      start_requested_at: $start_requested_at,
      ready_observed_at: $ready_observed_at
    },
    before: $before,
    after: $after
  }' >"${EVIDENCE_JSON}"

COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME}" \
COMPOSE_FILE="${COMPOSE_FILE}" \
RESULTS_DIR="${RESULTS_DIR}" \
RUN_ID="${POST_RUN_ID}" \
SCENARIO=concurrent \
SCENARIO_FILTER=concurrent \
BASE_URL="${BASE_URL}" \
NODE1_WS="${NODE1_WS}" \
NODE2_WS="${NODE2_WS}" \
NODE1_HEALTH_URL="${NODE1_HEALTH_URL}" \
NODE2_HEALTH_URL="${NODE2_HEALTH_URL}" \
USER_COUNT="${USER_COUNT:-20}" \
CONCURRENT_SEND_COUNT="${CONCURRENT_SEND_COUNT:-2}" \
PHONE_PREFIX="${PHONE_PREFIX:-136}" \
  scripts/bench/run_bench.sh

python3 scripts/bench/recovery_report.py \
  --evidence "${EVIDENCE_JSON}" \
  --baseline "${POST_BASELINE_JSON}" \
  --output "${REPORT_JSON}"

trap - EXIT
cat "${REPORT_JSON}"
