#!/usr/bin/env bash
set -euo pipefail

# Exercises the opt-in subscription-active path in an isolated project. The
# deterministic local model avoids sending smoke data or credentials off-host.

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
project_name="${COMPOSE_PROJECT_NAME:-dipole-agent-subscription-active-${RANDOM}-$$}"
scratch_dir=$(mktemp -d "${TMPDIR:-/tmp}/dipole-agent-subscription-active.XXXXXX")
owner_telephone="13900000004"
agent_uuid="UAI000000000000000001"
# Keep test identifiers within the production columns shared by Message,
# Sync, and the Agent event ledger.
event_id="EA-${RANDOM}-$$"
grant_uuid="PROMOTION-SUBSCRIPTION-ACTIVE-${RANDOM}-$$"

command -v docker >/dev/null 2>&1 || { printf 'Docker is required\n' >&2; exit 2; }
command -v openssl >/dev/null 2>&1 || { printf 'openssl is required\n' >&2; exit 2; }

if [[ "${BUILD_IMAGE:-0}" == "1" ]]; then
  "${root_dir}/scripts/docker-build.sh" backend
  "${root_dir}/scripts/docker-build-microservice-images.sh"
fi

: "${DIPOLE_MIGRATE_IMAGE:=dipole-migrate:latest}"
: "${DIPOLE_CORE_IMAGE:=dipole-core:latest}"
: "${DIPOLE_GATEWAY_IMAGE:=dipole-gateway:latest}"
: "${DIPOLE_MESSAGE_IMAGE:=dipole-message:latest}"
: "${DIPOLE_SYNC_IMAGE:=dipole-sync:latest}"
: "${DIPOLE_AGENT_IMAGE:=dipole-agent:latest}"
: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
: "${DIPOLE_AGENT_CANDIDATE_VERSION:=agent-runtime@subscription-active-compose-smoke}"
: "${DIPOLE_AGENT_ACTIVE_KAFKA_GROUP_ID:=dipole-agent-active-subscription-smoke-${RANDOM}-$$}"
: "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_KAFKA_GROUP_ID:=dipole-agent-subscription-active-smoke-${RANDOM}-$$}"
: "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_TASK_QUEUE:=dipole-agent-subscription-smoke-${RANDOM}-$$}"
: "${DIPOLE_AGENT_TEMPORAL_ADDRESS:=temporal:7233}"
: "${DIPOLE_AGENT_TEMPORAL_NAMESPACE:=default}"
: "${DIPOLE_AGENT_TEMPORAL_TASK_QUEUE:=${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_TASK_QUEUE}}"
: "${DIPOLE_GATEWAY_BIND_ADDRESS:=127.0.0.1}"
: "${DIPOLE_GATEWAY_PORT:=$((18000 + RANDOM % 2000))}"
: "${DIPOLE_MYSQL_AIO_COMPAT:=0}"
[[ "${DIPOLE_MYSQL_AIO_COMPAT}" == "0" || "${DIPOLE_MYSQL_AIO_COMPAT}" == "1" ]] || { printf 'DIPOLE_MYSQL_AIO_COMPAT must be 0 or 1\n' >&2; exit 2; }

export DIPOLE_MIGRATE_IMAGE DIPOLE_CORE_IMAGE DIPOLE_GATEWAY_IMAGE DIPOLE_MESSAGE_IMAGE DIPOLE_SYNC_IMAGE DIPOLE_AGENT_IMAGE
export DIPOLE_INTERNAL_RPC_SHARED_SECRET DIPOLE_AGENT_CANDIDATE_VERSION DIPOLE_AGENT_ACTIVE_KAFKA_GROUP_ID
export DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_KAFKA_GROUP_ID DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_TASK_QUEUE
export DIPOLE_AGENT_TEMPORAL_ADDRESS DIPOLE_AGENT_TEMPORAL_NAMESPACE DIPOLE_AGENT_TEMPORAL_TASK_QUEUE
export DIPOLE_GATEWAY_BIND_ADDRESS DIPOLE_GATEWAY_PORT
export DIPOLE_AGENT_RELEASE_MANIFEST_FILE="${scratch_dir}/release-manifest.json"
export DIPOLE_INTERNAL_CERT_DIR="${scratch_dir}/certs"
export INTERNAL_CERT_DIR="${DIPOLE_INTERNAL_CERT_DIR}"
export DIPOLE_AGENT_MODEL_PROVIDER_NAME="compose-smoke"
export DIPOLE_AGENT_MODEL_BASE_URL="http://127.0.0.1:8089/v1"
export DIPOLE_AGENT_MODEL_API_KEY="compose-smoke-no-network"
export DIPOLE_AGENT_MODEL_ROUTES="compose-smoke/deterministic"
export DIPOLE_AGENT_MODEL_CONTEXT_PROFILES='[{"route":"compose-smoke/deterministic","contextWindowTokens":32768,"utf8BytesPerToken":3,"safetyMarginBps":1500}]'
export DIPOLE_AGENT_MODEL_OUTPUT_MODE="json_text"
export DIPOLE_AGENT_MODEL_MAX_CALLS="1"
export DIPOLE_AGENT_MODEL_TOTAL_TIMEOUT_MS="5000"
export DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS="256"
export DIPOLE_AGENT_SUBSCRIPTION_MODEL_STUB_FILE="${scratch_dir}/model-stub.mjs"

cat >"${DIPOLE_AGENT_RELEASE_MANIFEST_FILE}" <<EOF
{"schemaVersion":"dipole.agent.release-manifest.v1","candidateVersion":"${DIPOLE_AGENT_CANDIDATE_VERSION}","runtimeId":"dipole-agent","stage":"user_gray","components":{"model":{"version":"v1","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"prompt":{"version":"v1","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},"capabilitySchema":{"version":"v1","sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},"memoryPolicy":{"version":"v1","sha256":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"}},"offlineEvalSuiteSha256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","createdAt":"2026-09-02T00:00:00.000Z"}
EOF

cat >"${DIPOLE_AGENT_SUBSCRIPTION_MODEL_STUB_FILE}" <<'NODE'
import http from "node:http";
const body = JSON.stringify({ id: "subscription-active-smoke", object: "chat.completion", choices: [{ index: 0, finish_reason: "stop", message: { role: "assistant", content: '{"summary":"subscription active smoke","steps":[]}' } }], usage: { prompt_tokens: 1, completion_tokens: 1, total_tokens: 2 } });
http.createServer((request, response) => {
  if (request.method !== "POST" || request.url !== "/v1/chat/completions") { response.writeHead(404).end(); return; }
  request.resume(); request.on("end", () => response.writeHead(200, { "content-type": "application/json" }).end(body));
}).listen(8089, "0.0.0.0");
NODE

compose_files=(
  -f "${root_dir}/deploy/compose/docker-compose.microservices.yml"
  -f "${root_dir}/deploy/microservices/agent-temporal-read-shadow.yml"
  -f "${root_dir}/deploy/microservices/agent-subscription-shadow.yml"
  -f "${root_dir}/deploy/microservices/agent-active.yml"
  -f "${root_dir}/deploy/microservices/agent-subscription-active.yml"
  -f "${root_dir}/deploy/microservices/agent-subscription-active-smoke.yml"
)
[[ "${DIPOLE_MYSQL_AIO_COMPAT}" == "0" ]] || compose_files+=( -f "${root_dir}/deploy/microservices/remote-gpu-mysql-aio-compat.yml" )
compose() { docker compose -p "${project_name}" "${compose_files[@]}" "$@"; }

cleanup() {
  local status=$?
  compose exec -T mysql mysql -uroot -proot123 dipole -e "UPDATE agent_runtime_promotion_grants SET revoked_at = COALESCE(revoked_at, UTC_TIMESTAMP(3)) WHERE grant_uuid = '${grant_uuid}'" >/dev/null 2>&1 || true
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    compose down --volumes --remove-orphans >/dev/null 2>&1 || true
    rm -rf "${scratch_dir}"
  else
    printf 'Subscription active Compose stack retained: project=%s scratch=%s\n' "${project_name}" "${scratch_dir}" >&2
  fi
  exit "${status}"
}
trap cleanup EXIT INT TERM

"${root_dir}/scripts/generate-internal-certs.sh"
compose config --quiet
compose up -d --wait

mysql() { compose exec -T mysql mysql -N -B -uroot -proot123 dipole "$@"; }
mysql -e "INSERT IGNORE INTO users (uuid, nickname, telephone, password_hash, status, created_at, updated_at) VALUES ('${agent_uuid}', 'Dipole Agent', '13900000002', 'smoke', 1, NOW(3), NOW(3));"

binding=$(compose exec -T agent node --input-type=module - "${owner_telephone}" "${agent_uuid}" <<'NODE'
const [telephone, agentUuid] = process.argv.slice(2);
const register = await fetch("http://core:8081/api/v1/auth/register", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ nickname: "Subscription Active", telephone, password: "smoke-pass-123" }) });
const ownerUuid = (await register.json())?.data?.user?.uuid;
if (register.status !== 200 || typeof ownerUuid !== "string") throw new Error(`register failed: ${register.status}`);
const login = await fetch("http://core:8081/api/v1/auth/login", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ telephone, password: "smoke-pass-123" }) });
const token = (await login.json())?.data?.token;
if (login.status !== 200 || typeof token !== "string") throw new Error(`login failed: ${login.status}`);
const headers = { authorization: `Bearer ${token}`, "content-type": "application/json" };
const definitionResponse = await fetch("http://gateway:8080/api/v1/agent/definitions", { method: "POST", headers });
const definition = await definitionResponse.json();
if (definitionResponse.status !== 201 || typeof definition?.definitionId !== "string" || definition.version !== 1) throw new Error(`definition failed: ${definitionResponse.status}`);
const socket = new WebSocket(`ws://gateway:8080/api/v1/ws?token=${encodeURIComponent(token)}&device=smoke`);
await new Promise((resolve, reject) => { const timer = setTimeout(() => reject(new Error("bootstrap timeout")), 15000); socket.addEventListener("open", () => socket.send(JSON.stringify({ type: "chat.send", data: { target_uuid: agentUuid, content: "establish subscription scope", client_message_id: `bootstrap-${ownerUuid}` } }))); socket.addEventListener("message", ({ data }) => { const event = JSON.parse(String(data)); if (event?.type === "chat.sent") { clearTimeout(timer); socket.close(); resolve(); } if (event?.type === "error") reject(new Error(`bootstrap failed: ${JSON.stringify(event.data)}`)); }); socket.addEventListener("error", () => reject(new Error("bootstrap socket failed"))); });
const conversationKey = `direct:${[ownerUuid, agentUuid].sort().join(":")}`;
let eligible = false;
for (let attempt = 0; attempt < 60; attempt += 1) { const response = await fetch(`http://gateway:8080/api/v1/agent/subscriptions/options?definitionId=${encodeURIComponent(definition.definitionId)}&definitionVersion=1`, { headers }); const body = await response.json(); eligible = response.status === 200 && body?.conversations?.some(item => item?.conversationKey === conversationKey); if (eligible) break; await new Promise(resolve => setTimeout(resolve, 1000)); }
if (!eligible) throw new Error("subscription conversation did not become eligible");
const response = await fetch("http://gateway:8080/api/v1/agent/subscriptions", { method: "POST", headers, body: JSON.stringify({ definitionId: definition.definitionId, definitionVersion: 1, conversationKey, filterKind: "all", filter: {} }) });
const subscription = await response.json();
if (response.status !== 200 || typeof subscription?.subscriptionId !== "string") throw new Error(`subscription failed: ${response.status}`);
process.stdout.write(`${ownerUuid}\t${definition.definitionId}\t${subscription.subscriptionId}\t${conversationKey}`);
NODE
)
IFS=$'\t' read -r owner_uuid definition_uuid subscription_uuid conversation_key <<<"${binding}"

mysql <<SQL
INSERT INTO agent_runtime_promotion_grants (grant_uuid, tenant_id, runtime_id, candidate_version, definition_uuid, definition_version, policy_version, evidence_sha256, eval_suite_sha256, granted_by_uuid, reviewed_by_uuid, valid_from, expires_at) VALUES ('${grant_uuid}', 'dipole', 'dipole-agent', '${DIPOLE_AGENT_CANDIDATE_VERSION}', '${definition_uuid}', 1, 'dipole.agent.shadow-promotion-policy.v2', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 'U-SMOKE-GRANTOR', 'U-SMOKE-REVIEWER', DATE_SUB(UTC_TIMESTAMP(3), INTERVAL 1 MINUTE), DATE_ADD(UTC_TIMESTAMP(3), INTERVAL 15 MINUTE));
SQL

compose exec -T agent node --input-type=module - "${event_id}" "${owner_uuid}" "${agent_uuid}" "${conversation_key}" <<'NODE'
import { Kafka } from "kafkajs";
const [eventId, ownerUuid, agentUuid, conversationKey] = process.argv.slice(2);
const event = { event_id: eventId, event_type: "message.direct.created", version: "v1", source: "dipole", occurred_at: new Date().toISOString(), payload: { mutation_type: "created", revision: 1, actor_uuid: ownerUuid, message_id: `M-${eventId}`, conversation_key: conversationKey, message_seq: 1, sender_uuid: ownerUuid, target_uuid: agentUuid, target_type: 0, message_type: 0, content: "subscription active smoke", sent_at: new Date().toISOString() } };
const producer = new Kafka({ clientId: "subscription-active-smoke", brokers: ["kafka:9092"] }).producer(); await producer.connect(); await producer.send({ topic: "dipole.message.direct.created", messages: [{ key: eventId, value: JSON.stringify(event) }] }); await producer.disconnect();
NODE

task_state=""
for _ in $(seq 1 90); do
  task_state=$(mysql -e "SELECT CONCAT(status, '\\t', workflow_status) FROM agent_tasks WHERE trigger_subscription_uuid = '${subscription_uuid}' AND trigger_ref = 'M-${event_id}'" || true)
  [[ "${task_state}" == $'completed\tcompleted' ]] && break
  sleep 1
done
[[ "${task_state}" == $'completed\tcompleted' ]] || { printf 'subscription task did not complete: %q\n' "${task_state}" >&2; exit 1; }
model_calls=$(mysql -e "SELECT COUNT(*) FROM agent_model_runs AS r JOIN agent_tasks AS t ON t.task_uuid = r.task_uuid WHERE t.trigger_subscription_uuid = '${subscription_uuid}' AND r.status = 'completed'")
[[ "${model_calls}" == "1" ]] || { printf 'expected one completed model call, got %s\n' "${model_calls}" >&2; exit 1; }
messages=$(mysql -e "SELECT COUNT(*) FROM messages WHERE sender_uuid = '${agent_uuid}' AND target_uuid = '${owner_uuid}'")
[[ "${messages}" == "0" ]] || { printf 'subscription read task wrote %s messages\n' "${messages}" >&2; exit 1; }
mysql -e "UPDATE agent_runtime_promotion_grants SET revoked_at = UTC_TIMESTAMP(3) WHERE grant_uuid = '${grant_uuid}' AND revoked_at IS NULL"
printf 'Agent Subscription active Compose smoke passed: one owner-scoped Kafka event completed one durable read Task with one model call and zero messages.\n'
