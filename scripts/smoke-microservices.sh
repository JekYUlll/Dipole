#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/deploy/compose/docker-compose.microservices.yml"
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-dipole-microservices-smoke}"
GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8080}"
COMPOSE_OVERLAYS="${COMPOSE_OVERLAYS:-}"
COMPOSE_ENV_FILE="${COMPOSE_ENV_FILE:-}"
RESTART_CORE="${RESTART_CORE:-0}"
RESTART_CORE_AFTER_EVENT="${RESTART_CORE_AFTER_EVENT:-0}"
CORE_OUTAGE_BEFORE_EVENT="${CORE_OUTAGE_BEFORE_EVENT:-0}"
EXPECT_READ_SHADOW="${EXPECT_READ_SHADOW:-0}"
AGENT_EVENT_TIMEOUT_SECONDS="${AGENT_EVENT_TIMEOUT_SECONDS:-30}"

for boolean in RESTART_CORE RESTART_CORE_AFTER_EVENT CORE_OUTAGE_BEFORE_EVENT EXPECT_READ_SHADOW; do
  [[ "${!boolean}" == "0" || "${!boolean}" == "1" ]] || { echo "${boolean} must be 0 or 1" >&2; exit 2; }
done
[[ "${AGENT_EVENT_TIMEOUT_SECONDS}" =~ ^[1-9][0-9]*$ && "${AGENT_EVENT_TIMEOUT_SECONDS}" -le 180 ]] || { echo "AGENT_EVENT_TIMEOUT_SECONDS must be an integer from 1 to 180" >&2; exit 2; }
[[ "${CORE_OUTAGE_BEFORE_EVENT}" == "0" || "${EXPECT_READ_SHADOW}" == "1" ]] || { echo "CORE_OUTAGE_BEFORE_EVENT requires EXPECT_READ_SHADOW=1" >&2; exit 2; }

if [[ "${BUILD_IMAGE:-0}" == "1" ]]; then
  "${SCRIPT_DIR}/docker-build.sh" backend
  "${SCRIPT_DIR}/docker-build-microservice-images.sh"
fi

: "${DIPOLE_MIGRATE_IMAGE:=dipole-migrate:latest}"
: "${DIPOLE_CORE_IMAGE:=dipole-core:latest}"
: "${DIPOLE_GATEWAY_IMAGE:=dipole-gateway:latest}"
: "${DIPOLE_MESSAGE_IMAGE:=dipole-message:latest}"
: "${DIPOLE_SYNC_IMAGE:=dipole-sync:latest}"
: "${DIPOLE_SEARCH_IMAGE:=dipole-search:latest}"
: "${DIPOLE_SEARCH_INDEXER_IMAGE:=dipole-search-indexer:latest}"
: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
export DIPOLE_MIGRATE_IMAGE DIPOLE_CORE_IMAGE DIPOLE_GATEWAY_IMAGE DIPOLE_MESSAGE_IMAGE
export DIPOLE_SYNC_IMAGE DIPOLE_SEARCH_IMAGE DIPOLE_SEARCH_INDEXER_IMAGE DIPOLE_INTERNAL_RPC_SHARED_SECRET

compose_args=()
if [[ -n "${COMPOSE_ENV_FILE}" ]]; then
  compose_env_path="${ROOT_DIR}/${COMPOSE_ENV_FILE}"
  [[ -f "${compose_env_path}" ]] || { echo "Compose environment file does not exist: ${COMPOSE_ENV_FILE}" >&2; exit 2; }
  compose_args+=(--env-file "${compose_env_path}")
fi
compose_args+=(-p "${PROJECT_NAME}" -f "${COMPOSE_FILE}")
if [[ -n "${COMPOSE_OVERLAYS}" ]]; then
  IFS=':' read -r -a overlay_files <<<"${COMPOSE_OVERLAYS}"
  for overlay_file in "${overlay_files[@]}"; do
    overlay_path="${ROOT_DIR}/${overlay_file}"
    [[ -f "${overlay_path}" ]] || { echo "Compose overlay does not exist: ${overlay_file}" >&2; exit 2; }
    compose_args+=(-f "${overlay_path}")
  done
fi

compose() {
  docker compose "${compose_args[@]}" "$@"
}

cleanup() {
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

"${SCRIPT_DIR}/generate-internal-certs.sh"
compose config --quiet
compose up -d --wait

health=""
for _ in $(seq 1 30); do
  health="$(curl --connect-timeout 2 --max-time 5 -fsS "${GATEWAY_URL}/health" 2>/dev/null || true)"
  if [[ "${health}" == *'"component":"gateway"'* ]]; then
    break
  fi
  sleep 1
done
[[ "${health}" == *'"component":"gateway"'* ]]

proxy_status="$(curl --connect-timeout 2 --max-time 5 -sS -o /dev/null -w '%{http_code}' "${GATEWAY_URL}/api/v1/contacts" || true)"
[[ "${proxy_status}" == "401" ]]

core_ws_status="$(compose exec -T core sh -c \
  'wget -S -O /dev/null http://127.0.0.1:8081/api/v1/ws 2>&1 || true' \
  | sed -n 's/.*HTTP\/1.1 \([0-9][0-9][0-9]\).*/\1/p' \
  | head -n 1)"
[[ "${core_ws_status}" == "404" ]]

restart_core() {
  compose restart core
  core_ready=""
  for _ in $(seq 1 30); do
    core_ready="$(compose exec -T core wget -q -O - http://127.0.0.1:9100/readyz 2>/dev/null || true)"
    if [[ "${core_ready}" == "ready" ]]; then
      break
    fi
    sleep 1
  done
  [[ "${core_ready}" == "ready" ]]
  proxy_status="$(curl --connect-timeout 2 --max-time 5 -sS -o /dev/null -w '%{http_code}' "${GATEWAY_URL}/api/v1/contacts" || true)"
  [[ "${proxy_status}" == "401" ]]
}

if [[ "${RESTART_CORE}" == "1" ]]; then
  restart_core
fi

if [[ "${CORE_OUTAGE_BEFORE_EVENT}" == "1" ]]; then
  compose stop core
fi

for service in core message sync gateway; do
  compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/livez | grep -qx 'alive'
  compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/readyz | grep -qx 'ready'
  compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/metrics | grep -q 'dipole_service_ready{service="dipole-'
done

compose exec -T agent node -e '
const http = require("node:http");
const paths = ["/livez", "/readyz"];
Promise.all(paths.map(path => new Promise((resolve, reject) => {
  const request = http.get(`http://127.0.0.1:8091${path}`, response => {
    response.resume();
    response.on("end", () => response.statusCode === 200 ? resolve() : reject(new Error(`${path}: ${response.statusCode}`)));
  });
  request.on("error", reject);
}))).catch(error => { console.error(error.message); process.exit(1); });
'

agent_event_id="SMOKE-AGENT-EVENT-$(openssl rand -hex 8)"
agent_message_id="SMOKE-AGENT-MESSAGE-$(openssl rand -hex 8)"
compose exec -T agent node --input-type=module - "${agent_event_id}" "${agent_message_id}" <<'NODE'
import { Kafka } from "kafkajs";

const [eventId, messageId] = process.argv.slice(2);
const kafka = new Kafka({ clientId: "dipole-agent-smoke-producer", brokers: ["kafka:9092"] });
const producer = kafka.producer();
const occurredAt = new Date().toISOString();
await producer.connect();
try {
  await producer.send({
    topic: "dipole.message.direct.created",
    messages: [{
      key: messageId,
      value: JSON.stringify({
        event_id: eventId,
        request_id: `REQ-${eventId}`,
        trace_id: `TRACE-${eventId}`,
        event_type: "message.direct.created",
        version: "v1",
        source: "dipole",
        occurred_at: occurredAt,
        payload: {
          mutation_type: "created",
          revision: 1,
          actor_uuid: "U100",
          message_id: messageId,
          conversation_key: "direct:U100:UAI000000000000000001",
          message_seq: 1,
          sender_uuid: "U100",
          target_uuid: "UAI000000000000000001",
          target_type: 0,
          message_type: 0,
          content: "smoke event",
          sent_at: occurredAt
        }
      })
    }, {
      key: messageId,
      value: JSON.stringify({
        event_id: eventId,
        request_id: `REQ-${eventId}`,
        trace_id: `TRACE-${eventId}`,
        event_type: "message.direct.created",
        version: "v1",
        source: "dipole",
        occurred_at: occurredAt,
        payload: {
          mutation_type: "created",
          revision: 1,
          actor_uuid: "U100",
          message_id: messageId,
          conversation_key: "direct:U100:UAI000000000000000001",
          message_seq: 1,
          sender_uuid: "U100",
          target_uuid: "UAI000000000000000001",
          target_type: 0,
          message_type: 0,
          content: "smoke event",
          sent_at: occurredAt
        }
      })
    }]
  });
} finally {
  await producer.disconnect();
}
NODE

if [[ "${CORE_OUTAGE_BEFORE_EVENT}" == "1" ]]; then
  ledger_claimed=""
  for _ in $(seq 1 15); do
    ledger_claimed="$(compose exec -T mysql mysql -uroot -proot123 -Ddipole -N -B -e \
      "SELECT COUNT(*) FROM agent_event_ledger WHERE event_id='${agent_event_id}';" 2>/dev/null || true)"
    [[ "${ledger_claimed}" == "1" ]] && break
    sleep 1
  done
  [[ "${ledger_claimed}" == "1" ]]
  # Hold Core unavailable long enough for Temporal's first admission attempt.
  sleep 2
  compose start core
  core_ready=""
  for _ in $(seq 1 30); do
    core_ready="$(compose exec -T core wget -q -O - http://127.0.0.1:9100/readyz 2>/dev/null || true)"
    [[ "${core_ready}" == "ready" ]] && break
    sleep 1
  done
  [[ "${core_ready}" == "ready" ]]
  proxy_status="$(curl --connect-timeout 2 --max-time 5 -sS -o /dev/null -w '%{http_code}' "${GATEWAY_URL}/api/v1/contacts" || true)"
  [[ "${proxy_status}" == "401" ]]
fi

if [[ "${RESTART_CORE_AFTER_EVENT}" == "1" ]]; then
  restart_core
fi

read_shadow_assertions=""
if [[ "${EXPECT_READ_SHADOW}" == "1" ]]; then
  read_shadow_assertions="
      AND (SELECT COUNT(*) FROM agent_model_runs WHERE task_uuid=(SELECT task_uuid FROM agent_event_ledger WHERE event_id='${agent_event_id}') AND status='completed') >= 1
      AND (SELECT COUNT(*) FROM agent_model_calls WHERE run_uuid=(SELECT run_uuid FROM agent_model_runs WHERE task_uuid=(SELECT task_uuid FROM agent_event_ledger WHERE event_id='${agent_event_id}') ORDER BY created_at DESC LIMIT 1) AND status='completed') >= 1
      AND (SELECT COUNT(*) FROM agent_shadow_steps WHERE task_uuid=(SELECT task_uuid FROM agent_event_ledger WHERE event_id='${agent_event_id}') AND capability_id='conversation.list' AND status='completed') = 1
      AND (SELECT COUNT(*) FROM agent_artifacts WHERE task_uuid=(SELECT task_uuid FROM agent_event_ledger WHERE event_id='${agent_event_id}') AND artifact_type='conversation_digest' AND version=1) = 1"
fi

agent_event_ready=""
for _ in $(seq 1 "${AGENT_EVENT_TIMEOUT_SECONDS}"); do
  agent_event_ready="$(compose exec -T mysql mysql -uroot -proot123 -Ddipole -N -B -e \
    "SELECT IF(
      (SELECT COUNT(*) FROM agent_event_ledger WHERE event_id='${agent_event_id}' AND status='completed') = 1
      AND (SELECT COUNT(*) FROM agent_shadow_plans WHERE event_id='${agent_event_id}') = 1
      AND (SELECT COUNT(*) FROM agent_tasks WHERE trigger_ref='${agent_message_id}' AND status='running') = 1
      AND (SELECT COUNT(*) FROM agent_runs WHERE task_uuid=(SELECT task_uuid FROM agent_event_ledger WHERE event_id='${agent_event_id}') AND mode='shadow' AND status='completed') = 1
      ${read_shadow_assertions},
      1, 0
    );" \
    2>/dev/null || true)"
  if [[ "${agent_event_ready}" == "1" ]]; then
    break
  fi
  sleep 1
done
[[ "${agent_event_ready}" == "1" ]]

echo "Microservices smoke passed: readiness, metrics, Core proxy, mTLS startup, remote WS ownership, Agent event ledger/task/run idempotency, core_restart_before_event=${RESTART_CORE}, core_restart_after_event=${RESTART_CORE_AFTER_EVENT}, core_outage_before_event=${CORE_OUTAGE_BEFORE_EVENT}, read_shadow=${EXPECT_READ_SHADOW}"
