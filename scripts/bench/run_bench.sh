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
NODE1_HEALTH_URL="${NODE1_HEALTH_URL:-${BASE_URL%/}/health}"
NODE2_HEALTH_URL="${NODE2_HEALTH_URL:-http://127.0.0.1:8082/health}"
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
CONVERSATION_METRICS_SERVICES="${CONVERSATION_METRICS_SERVICES:-dipole-node1 dipole-node2 dipole-node3}"
PROCESS_METRICS_SERVICES="${PROCESS_METRICS_SERVICES:-dipole-node1 dipole-node2 dipole-node3}"

SUMMARY_JSON="${RESULTS_DIR}/${RUN_ID}.k6-summary.json"
OPERATIONS_JSON="${RESULTS_DIR}/${RUN_ID}.operations.json"
BASELINE_JSON="${RESULTS_DIR}/${RUN_ID}.baseline.json"
BASELINE_MD="${RESULTS_DIR}/${RUN_ID}.baseline.md"
RUN_LOG="${RESULTS_DIR}/${RUN_ID}.log"
LAG_FILE="${RESULTS_DIR}/${RUN_ID}.lag"
CONVERSATION_METRICS_JSON="${RESULTS_DIR}/${RUN_ID}.conversation-metrics.json"
PROCESS_SAMPLES_JSONL="${RESULTS_DIR}/${RUN_ID}.process-samples.jsonl"
PROCESS_RESOURCES_JSON="${RESULTS_DIR}/${RUN_ID}.process-resources.json"
RUNTIME_PROVENANCE_JSON="${RESULTS_DIR}/${RUN_ID}.runtime-provenance.json"
CONVERSATION_METRIC_ARGS=()
PROCESS_METRIC_ARGS=()
RUNTIME_PROVENANCE_SERVICE_ARGS=()

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "required command not found: $1" >&2
    exit 1
  }
}

for command in docker curl git k6 jq python3; do
  require_command "${command}"
done

git_commit="$(git rev-parse HEAD)"
if [[ -n "$(git status --porcelain)" ]]; then
  echo "benchmark source tree has tracked changes; commit them before collecting a baseline" >&2
  exit 1
fi

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
: >"${PROCESS_SAMPLES_JSONL}"

for health_url in "${NODE1_HEALTH_URL}" "${NODE2_HEALTH_URL}"; do
  curl --fail --silent --show-error "${health_url}" >/dev/null || {
    echo "Dipole node is not ready at ${health_url}; start ${COMPOSE_FILE} first" >&2
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
    | python3 scripts/bench/kafka_lag.py --group-prefix dipole)"
  printf '%s\n' "${LAST_KAFKA_LAG}" >>"${LAG_FILE}"
}

capture_conversation_metrics() {
  local phase="$1"
  local option="--${phase}"
  local service output
  for service in ${CONVERSATION_METRICS_SERVICES}; do
    output="${RESULTS_DIR}/${RUN_ID}.conversation-${service}.${phase}.prom"
    docker compose -f "${COMPOSE_FILE}" exec -T "${service}" \
      wget -q -O - http://127.0.0.1:9100/metrics >"${output}"
    CONVERSATION_METRIC_ARGS+=("${option}" "${output}")
  done
}

resolve_process_metric_bindings() {
  local service container_id host_pid image_id revision created source_dirty service_json
  for service in ${PROCESS_METRICS_SERVICES}; do
    container_id="$(docker compose -f "${COMPOSE_FILE}" ps -q "${service}")"
    if [[ -z "${container_id}" ]]; then
      echo "process metrics service is not running: ${service}" >&2
      exit 1
    fi
    host_pid="$(docker inspect --format '{{.State.Pid}}' "${container_id}")"
    if [[ ! "${host_pid}" =~ ^[1-9][0-9]*$ ]]; then
      echo "process metrics service has no host pid: ${service}" >&2
      exit 1
    fi
    PROCESS_METRIC_ARGS+=(--service "${service}=${host_pid}")

    image_id="$(docker inspect --format '{{.Image}}' "${container_id}")"
    revision="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "${image_id}")"
    created="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.created"}}' "${image_id}")"
    source_dirty="$(docker image inspect --format '{{index .Config.Labels "io.dipole.source.dirty"}}' "${image_id}")"
    if [[ "${source_dirty}" != "true" && "${source_dirty}" != "false" ]]; then
      echo "runtime image has no valid source dirty label: ${service} (${image_id})" >&2
      exit 1
    fi
    service_json="$(jq -cn \
      --arg name "${service}" \
      --arg container_id "${container_id}" \
      --arg image_id "${image_id}" \
      --arg revision "${revision}" \
      --arg created "${created}" \
      --argjson source_dirty "${source_dirty}" \
      '{name: $name, container_id: $container_id, image_id: $image_id, revision: $revision, created: $created, source_dirty: $source_dirty}')"
    RUNTIME_PROVENANCE_SERVICE_ARGS+=(--service-json "${service_json}")
  done

  python3 scripts/bench/runtime_provenance.py \
    --expected-revision "${git_commit}" \
    "${RUNTIME_PROVENANCE_SERVICE_ARGS[@]}" \
    --output "${RUNTIME_PROVENANCE_JSON}"
}

capture_process_metrics() {
  python3 scripts/bench/process_metrics.py capture \
    "${PROCESS_METRIC_ARGS[@]}" \
    --output "${PROCESS_SAMPLES_JSONL}"
}

capture_conversation_metrics before
resolve_process_metric_bindings
capture_process_metrics

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
  capture_process_metrics
  sleep "${LAG_SAMPLE_SECONDS}"
done
wait "${k6_pid}"
k6_status=$?
set -e
sample_kafka_lag
capture_process_metrics
settle_started="$(date +%s)"
while [[ "${LAST_KAFKA_LAG}" != "0" ]] && (( $(date +%s) - settle_started < LAG_SETTLE_TIMEOUT_SECONDS )); do
  sleep "${LAG_SAMPLE_SECONDS}"
  sample_kafka_lag
done
cat "${RUN_LOG}"

capture_conversation_metrics after
python3 scripts/bench/conversation_metrics.py \
  "${CONVERSATION_METRIC_ARGS[@]}" \
  --output "${CONVERSATION_METRICS_JSON}"
conversation_metrics="$(cat "${CONVERSATION_METRICS_JSON}")"
python3 scripts/bench/process_metrics.py summarize \
  --input "${PROCESS_SAMPLES_JSONL}" \
  --output "${PROCESS_RESOURCES_JSON}"
process_resources="$(cat "${PROCESS_RESOURCES_JSON}")"
runtime_provenance="$(cat "${RUNTIME_PROVENANCE_JSON}")"

if [[ ! -s "${SUMMARY_JSON}" ]]; then
  echo "k6 did not produce ${SUMMARY_JSON}" >&2
  exit "${k6_status}"
fi

read -r direct_messages direct_inbox group_messages group_inbox conversation_messages conversation_rows < <(
  docker compose -f "${COMPOSE_FILE}" exec -T "${MYSQL_SERVICE}" \
    mysql --batch --skip-column-names -uroot -proot123 dipole -e "
      SELECT
        (SELECT COUNT(*) FROM messages WHERE target_type = 0 AND content LIKE 'bench:${RUN_ID}:%'),
        (SELECT COUNT(*) FROM user_sync_inbox i JOIN messages m ON m.uuid = i.message_uuid WHERE m.target_type = 0 AND m.content LIKE 'bench:${RUN_ID}:%'),
        (SELECT COUNT(*) FROM messages WHERE target_type = 1 AND content LIKE 'bench:${RUN_ID}:%'),
        (SELECT COUNT(*) FROM user_sync_inbox i JOIN messages m ON m.uuid = i.message_uuid WHERE m.target_type = 1 AND m.content LIKE 'bench:${RUN_ID}:%'),
        (SELECT COUNT(*) FROM messages WHERE LEFT(client_message_id, CHAR_LENGTH('${RUN_ID}') + 1) = CONCAT('${RUN_ID}', '-')),
        (SELECT COUNT(*) FROM conversations c JOIN messages m ON m.uuid = c.last_message_uuid WHERE LEFT(m.client_message_id, CHAR_LENGTH('${RUN_ID}') + 1) = CONCAT('${RUN_ID}', '-'));" \
    2>/dev/null
)

lag_samples="$(jq --raw-input --slurp 'split("\n") | map(select(length > 0) | tonumber)' "${LAG_FILE}")"
cpu_model="$(awk -F ': ' '/model name/ { print $2; exit }' /proc/cpuinfo)"
jq -n \
  --arg run_id "${RUN_ID}" \
  --arg scenario "${SCENARIO}" \
  --arg captured_at "${TIMESTAMP}" \
  --arg git_commit "${git_commit}" \
  --arg cpu "${cpu_model}" \
  --arg topology "${COMPOSE_FILE}" \
  --arg api_base_url "${BASE_URL}" \
  --arg node1_ws "${NODE1_WS}" \
  --arg node2_ws "${NODE2_WS}" \
  --arg bench_script "${BENCH_SCRIPT}" \
  --argjson user_count "${USER_COUNT}" \
  --argjson group_size "${GROUP_SIZE}" \
  --arg phone_prefix "${PHONE_PREFIX}" \
  --argjson send_count "${SEND_COUNT}" \
  --argjson direct_send_count "${DIRECT_SEND_COUNT}" \
  --argjson concurrent_send_count "${CONCURRENT_SEND_COUNT}" \
  --argjson receiver_conn_ms "${RECEIVER_CONN_MS}" \
  --argjson sender_conn_ms "${SENDER_CONN_MS}" \
  --argjson hot_group_warmup_messages "${HOT_GROUP_WARMUP_MESSAGES}" \
  --arg hot_group_member_count_threshold "${HOT_GROUP_MEMBER_COUNT_THRESHOLD}" \
  --arg hot_group_message_threshold "${HOT_GROUP_MESSAGE_THRESHOLD}" \
  --argjson direct_messages "${direct_messages}" \
  --argjson direct_inbox "${direct_inbox}" \
  --argjson group_messages "${group_messages}" \
  --argjson group_inbox "${group_inbox}" \
  --argjson kafka_lag_samples "${lag_samples}" \
  --argjson conversation_rows "${conversation_rows}" \
  --argjson conversation_messages "${conversation_messages}" \
  --argjson conversation_metrics "${conversation_metrics}" \
  --argjson process_resources "${process_resources}" \
  --argjson runtime_provenance "${runtime_provenance}" \
  '{
    schema_version: "dipole.performance.operations.v4",
    run_id: $run_id,
    scenario: $scenario,
    captured_at: $captured_at,
    environment: {
      git_commit: $git_commit,
      cpu: $cpu,
      topology: $topology,
      api_base_url: $api_base_url,
      node1_ws: $node1_ws,
      node2_ws: $node2_ws
    },
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
      receiver_conn_ms: $receiver_conn_ms,
      sender_conn_ms: $sender_conn_ms,
      hot_group_warmup_messages: $hot_group_warmup_messages,
      hot_group_member_count_threshold: ($hot_group_member_count_threshold | if length == 0 then null else tonumber end),
      hot_group_message_threshold: ($hot_group_message_threshold | if length == 0 then null else tonumber end)
    },
    storage: {
      direct: {messages: $direct_messages, inbox_rows: $direct_inbox},
      group: {messages: $group_messages, inbox_rows: $group_inbox},
      conversation_state: ($conversation_metrics + {
        rows_touched: $conversation_rows,
        messages_observed: $conversation_messages
      })
    },
    kafka_lag_samples: $kafka_lag_samples,
    process_resources: $process_resources,
    runtime_provenance: $runtime_provenance
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
