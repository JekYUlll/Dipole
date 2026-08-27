#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.dist.yml}"
BENCH_SCRIPT="${BENCH_SCRIPT:-scripts/bench/bench.js}"
RESULTS_DIR="${RESULTS_DIR:-scripts/bench/results}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_ID="${RUN_ID:-g0-${TIMESTAMP}}"
SCENARIO="${SCENARIO:-mixed}"
SCENARIO_FILTER="${SCENARIO_FILTER:-}"
BASE_URL="${BASE_URL:-http://127.0.0.1:8081}"
NODE1_WS="${NODE1_WS:-ws://127.0.0.1:8081}"
NODE2_WS="${NODE2_WS:-ws://127.0.0.1:8082}"
USER_COUNT="${USER_COUNT:-50}"
GROUP_SIZE="${GROUP_SIZE:-50}"
PHONE_PREFIX="${PHONE_PREFIX:-138}"
LAG_SAMPLE_SECONDS="${LAG_SAMPLE_SECONDS:-2}"
LAG_SETTLE_TIMEOUT_SECONDS="${LAG_SETTLE_TIMEOUT_SECONDS:-30}"
MINIMUM_DELIVERY_RATE="${MINIMUM_DELIVERY_RATE:-0.90}"
ENFORCE_BASELINE="${ENFORCE_BASELINE:-true}"
SEND_COUNT="${SEND_COUNT:-20}"
DIRECT_SEND_COUNT="${DIRECT_SEND_COUNT:-5}"
CONCURRENT_SEND_COUNT="${CONCURRENT_SEND_COUNT:-8}"
SEND_INTERVAL_MS="${SEND_INTERVAL_MS:-300}"
SENDER_WARMUP_MS="${SENDER_WARMUP_MS:-2000}"
RECEIVER_CONN_MS="${RECEIVER_CONN_MS:-15000}"
SENDER_CONN_MS="${SENDER_CONN_MS:-15000}"
HOT_GROUP_WARMUP_MESSAGES="${HOT_GROUP_WARMUP_MESSAGES:-0}"
HOT_GROUP_ACTIVATION_WAIT_MS="${HOT_GROUP_ACTIVATION_WAIT_MS:-500}"
HOT_GROUP_MEMBER_COUNT_THRESHOLD="${HOT_GROUP_MEMBER_COUNT_THRESHOLD:-}"
HOT_GROUP_MESSAGE_THRESHOLD="${HOT_GROUP_MESSAGE_THRESHOLD:-}"
MYSQL_SERVICE="${MYSQL_SERVICE:-mysql}"
KAFKA_SERVICE="${KAFKA_SERVICE:-kafka}"

SUMMARY_JSON="${RESULTS_DIR}/${RUN_ID}.k6-summary.json"
OPERATIONS_JSON="${RESULTS_DIR}/${RUN_ID}.operations.json"
BASELINE_JSON="${RESULTS_DIR}/${RUN_ID}.baseline.json"
BASELINE_MD="${RESULTS_DIR}/${RUN_ID}.baseline.md"
RUN_LOG="${RESULTS_DIR}/${RUN_ID}.log"
LAG_FILE="${RESULTS_DIR}/${RUN_ID}.lag"

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "required command not found: $1" >&2
    exit 1
  }
}

for command in docker curl k6 jq python3; do
  require_command "${command}"
done

if [[ ! "${RUN_ID}" =~ ^[A-Za-z0-9._-]+$ ]]; then
  echo "RUN_ID may contain only letters, numbers, dot, underscore, and hyphen" >&2
  exit 1
fi
if [[ ! "${PHONE_PREFIX}" =~ ^[0-9]{3}$ ]]; then
  echo "PHONE_PREFIX must contain exactly three digits" >&2
  exit 1
fi

mkdir -p "${RESULTS_DIR}"
: >"${LAG_FILE}"

for port in 8081 8082; do
  curl --fail --silent --show-error "http://127.0.0.1:${port}/health" >/dev/null || {
    echo "Dipole node on port ${port} is not ready; start ${COMPOSE_FILE} first" >&2
    exit 1
  }
done

api_status="$(curl --silent --output /dev/null --write-out '%{http_code}' \
  --request POST --header 'Content-Type: application/json' --data '{}' \
  "${BASE_URL}/api/v1/auth/login")"
if [[ "${api_status}" == "301" || "${api_status}" == "302" || "${api_status}" == "404" ]]; then
  echo "API preflight failed: POST /api/v1/auth/login returned ${api_status}" >&2
  exit 1
fi

sample_kafka_lag() {
  LAST_KAFKA_LAG="$(docker compose -f "${COMPOSE_FILE}" exec -T "${KAFKA_SERVICE}" \
    /opt/kafka/bin/kafka-consumer-groups.sh \
    --bootstrap-server 127.0.0.1:9092 --all-groups --describe 2>/dev/null \
    | awk '$1 ~ /^dipole/ && $6 ~ /^[0-9]+$/ { total += $6 } END { print total + 0 }')"
  printf '%s\n' "${LAST_KAFKA_LAG}" >>"${LAG_FILE}"
}

echo "==> Running ${BENCH_SCRIPT} with run_id=${RUN_ID}"
set +e
k6 run \
  --summary-export "${SUMMARY_JSON}" \
  --summary-trend-stats "avg,min,med,max,p(50),p(90),p(95),p(99)" \
  -e RUN_ID="${RUN_ID}" \
  -e BASE_URL="${BASE_URL}" \
  -e NODE1_WS="${NODE1_WS}" \
  -e NODE2_WS="${NODE2_WS}" \
  -e USER_COUNT="${USER_COUNT}" \
  -e GROUP_SIZE="${GROUP_SIZE}" \
  -e PHONE_PREFIX="${PHONE_PREFIX}" \
  -e SCENARIO_FILTER="${SCENARIO_FILTER}" \
  -e SEND_COUNT="${SEND_COUNT}" \
  -e DIRECT_SEND_COUNT="${DIRECT_SEND_COUNT}" \
  -e CONCURRENT_SEND_COUNT="${CONCURRENT_SEND_COUNT}" \
  -e SEND_INTERVAL_MS="${SEND_INTERVAL_MS}" \
  -e SENDER_WARMUP_MS="${SENDER_WARMUP_MS}" \
  -e RECEIVER_CONN_MS="${RECEIVER_CONN_MS}" \
  -e SENDER_CONN_MS="${SENDER_CONN_MS}" \
  -e HOT_GROUP_WARMUP_MESSAGES="${HOT_GROUP_WARMUP_MESSAGES}" \
  -e HOT_GROUP_ACTIVATION_WAIT_MS="${HOT_GROUP_ACTIVATION_WAIT_MS}" \
  "${BENCH_SCRIPT}" >"${RUN_LOG}" 2>&1 &
k6_pid=$!

while kill -0 "${k6_pid}" 2>/dev/null; do
  sample_kafka_lag
  sleep "${LAG_SAMPLE_SECONDS}"
done
wait "${k6_pid}"
k6_status=$?
set -e
sample_kafka_lag
settle_started="$(date +%s)"
while [[ "${LAST_KAFKA_LAG}" != "0" ]] && (( $(date +%s) - settle_started < LAG_SETTLE_TIMEOUT_SECONDS )); do
  sleep "${LAG_SAMPLE_SECONDS}"
  sample_kafka_lag
done
cat "${RUN_LOG}"

if [[ ! -s "${SUMMARY_JSON}" ]]; then
  echo "k6 did not produce ${SUMMARY_JSON}" >&2
  exit "${k6_status}"
fi

read -r direct_messages direct_inbox group_messages group_inbox < <(
  docker compose -f "${COMPOSE_FILE}" exec -T "${MYSQL_SERVICE}" \
    mysql --batch --skip-column-names -uroot -proot123 dipole -e "
      SELECT
        (SELECT COUNT(*) FROM messages WHERE target_type = 0 AND content LIKE 'bench:${RUN_ID}:%'),
        (SELECT COUNT(*) FROM user_sync_inbox i JOIN messages m ON m.uuid = i.message_uuid WHERE m.target_type = 0 AND m.content LIKE 'bench:${RUN_ID}:%'),
        (SELECT COUNT(*) FROM messages WHERE target_type = 1 AND content LIKE 'bench:${RUN_ID}:%'),
        (SELECT COUNT(*) FROM user_sync_inbox i JOIN messages m ON m.uuid = i.message_uuid WHERE m.target_type = 1 AND m.content LIKE 'bench:${RUN_ID}:%');" \
    2>/dev/null
)

lag_samples="$(jq --raw-input --slurp 'split("\n") | map(select(length > 0) | tonumber)' "${LAG_FILE}")"
cpu_model="$(awk -F ': ' '/model name/ { print $2; exit }' /proc/cpuinfo)"
git_commit="$(git rev-parse HEAD)"

jq -n \
  --arg run_id "${RUN_ID}" \
  --arg scenario "${SCENARIO}" \
  --arg captured_at "${TIMESTAMP}" \
  --arg git_commit "${git_commit}" \
  --arg cpu "${cpu_model}" \
  --arg topology "${COMPOSE_FILE}" \
  --arg bench_script "${BENCH_SCRIPT}" \
  --argjson user_count "${USER_COUNT}" \
  --argjson group_size "${GROUP_SIZE}" \
  --arg phone_prefix "${PHONE_PREFIX}" \
  --argjson send_count "${SEND_COUNT}" \
  --argjson direct_send_count "${DIRECT_SEND_COUNT}" \
  --argjson concurrent_send_count "${CONCURRENT_SEND_COUNT}" \
  --argjson hot_group_warmup_messages "${HOT_GROUP_WARMUP_MESSAGES}" \
  --arg hot_group_member_count_threshold "${HOT_GROUP_MEMBER_COUNT_THRESHOLD}" \
  --arg hot_group_message_threshold "${HOT_GROUP_MESSAGE_THRESHOLD}" \
  --argjson direct_messages "${direct_messages}" \
  --argjson direct_inbox "${direct_inbox}" \
  --argjson group_messages "${group_messages}" \
  --argjson group_inbox "${group_inbox}" \
  --argjson kafka_lag_samples "${lag_samples}" \
  '{
    schema_version: "dipole.performance.operations.v1",
    run_id: $run_id,
    scenario: $scenario,
    captured_at: $captured_at,
    environment: {git_commit: $git_commit, cpu: $cpu, topology: $topology},
    parameters: {
      bench_script: $bench_script,
      user_count: $user_count,
      group_size: $group_size,
      phone_prefix: $phone_prefix,
      sender_count: (
        if $scenario == "direct_msg" then 10
        elif $scenario == "concurrent" then $user_count
        else 1
        end
      ),
      messages_per_sender: (
        if $scenario == "direct_msg" then $direct_send_count
        elif $scenario == "concurrent" then $concurrent_send_count
        else $send_count
        end
      ),
      hot_group_warmup_messages: $hot_group_warmup_messages,
      hot_group_member_count_threshold: ($hot_group_member_count_threshold | if length == 0 then null else tonumber end),
      hot_group_message_threshold: ($hot_group_message_threshold | if length == 0 then null else tonumber end)
    },
    storage: {
      direct: {messages: $direct_messages, inbox_rows: $direct_inbox},
      group: {messages: $group_messages, inbox_rows: $group_inbox}
    },
    kafka_lag_samples: $kafka_lag_samples
  }' >"${OPERATIONS_JSON}"

report_args=(
  "${SUMMARY_JSON}"
  "${OPERATIONS_JSON}"
  "${BASELINE_JSON}"
  "${BASELINE_MD}"
  --minimum-delivery-rate "${MINIMUM_DELIVERY_RATE}"
)
if [[ "${ENFORCE_BASELINE}" == "true" ]]; then
  report_args+=(--enforce)
fi

set +e
python3 scripts/bench/baseline_report.py "${report_args[@]}"
report_status=$?
set -e

echo "==> Baseline JSON: ${BASELINE_JSON}"
echo "==> Baseline Markdown: ${BASELINE_MD}"
echo "==> k6 log: ${RUN_LOG}"
if (( k6_status != 0 )); then
  exit "${k6_status}"
fi
exit "${report_status}"
