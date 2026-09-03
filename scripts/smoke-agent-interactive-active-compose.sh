#!/usr/bin/env bash
set -euo pipefail

# Runs the narrow, explicitly enabled Agent write surface in an isolated Compose
# project. It never uses a real model provider: /send follows the deterministic
# approval path and verifies its durable effects through Core and Message.

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root_dir=$(cd "${script_dir}/.." && pwd)
compose_file="${root_dir}/deploy/compose/docker-compose.microservices.yml"
project_name="${COMPOSE_PROJECT_NAME:-dipole-agent-interactive-active-${RANDOM}-$$}"
scratch_dir=$(mktemp -d "${TMPDIR:-/tmp}/dipole-agent-interactive-active.XXXXXX")
owner_uuid=""
owner_telephone="13900000001"
agent_uuid="UAI000000000000000001"
definition_uuid="DEF-SMOKE-ACTIVE"
grant_uuid="PROMOTION-SMOKE-ACTIVE"

command -v docker >/dev/null 2>&1 || { printf 'Docker is required\n' >&2; exit 2; }
command -v openssl >/dev/null 2>&1 || { printf 'openssl is required\n' >&2; exit 2; }
command -v node >/dev/null 2>&1 || { printf 'Node.js is required\n' >&2; exit 2; }

if [[ "${BUILD_IMAGE:-0}" == "1" ]]; then
  "${script_dir}/docker-build.sh" backend
  DIPOLE_MICROSERVICE_IMAGE_SERVICES="migrate,core,gateway,message,sync" \
    "${script_dir}/docker-build-microservice-images.sh"
fi

: "${DIPOLE_MIGRATE_IMAGE:=dipole-migrate:latest}"
: "${DIPOLE_CORE_IMAGE:=dipole-core:latest}"
: "${DIPOLE_GATEWAY_IMAGE:=dipole-gateway:latest}"
: "${DIPOLE_MESSAGE_IMAGE:=dipole-message:latest}"
: "${DIPOLE_SYNC_IMAGE:=dipole-sync:latest}"
: "${DIPOLE_AGENT_IMAGE:=dipole-agent:latest}"
: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
: "${DIPOLE_AGENT_CONTROL_SECRET:=$(openssl rand -hex 32)}"
: "${DIPOLE_AGENT_CANDIDATE_VERSION:=agent-runtime@interactive-compose-smoke}"
: "${DIPOLE_AGENT_ACTIVE_KAFKA_GROUP_ID:=dipole-agent-active-interactive-${RANDOM}-$$}"
: "${DIPOLE_AGENT_INTERACTIVE_TASK_QUEUE:=dipole-agent-interactive-${RANDOM}-$$}"
: "${DIPOLE_AGENT_TEMPORAL_ADDRESS:=temporal:7233}"
: "${DIPOLE_AGENT_TEMPORAL_NAMESPACE:=default}"
: "${DIPOLE_AGENT_TEMPORAL_TASK_QUEUE:=${DIPOLE_AGENT_INTERACTIVE_TASK_QUEUE}}"
: "${DIPOLE_GATEWAY_BIND_ADDRESS:=127.0.0.1}"
: "${DIPOLE_GATEWAY_PORT:=$((18000 + RANDOM % 2000))}"
: "${DIPOLE_MYSQL_AIO_COMPAT:=0}"
: "${DIPOLE_AGENT_DEFINITION_ONLY:=0}"
[[ "${DIPOLE_MYSQL_AIO_COMPAT}" == "0" || "${DIPOLE_MYSQL_AIO_COMPAT}" == "1" ]] || { printf 'DIPOLE_MYSQL_AIO_COMPAT must be 0 or 1\n' >&2; exit 2; }
[[ "${DIPOLE_AGENT_DEFINITION_ONLY}" == "0" || "${DIPOLE_AGENT_DEFINITION_ONLY}" == "1" ]] || { printf 'DIPOLE_AGENT_DEFINITION_ONLY must be 0 or 1\n' >&2; exit 2; }

export DIPOLE_MIGRATE_IMAGE DIPOLE_CORE_IMAGE DIPOLE_GATEWAY_IMAGE DIPOLE_MESSAGE_IMAGE DIPOLE_SYNC_IMAGE DIPOLE_AGENT_IMAGE
export DIPOLE_INTERNAL_RPC_SHARED_SECRET DIPOLE_AGENT_CONTROL_SECRET DIPOLE_AGENT_CANDIDATE_VERSION
export DIPOLE_AGENT_ACTIVE_KAFKA_GROUP_ID DIPOLE_AGENT_INTERACTIVE_TASK_QUEUE DIPOLE_GATEWAY_BIND_ADDRESS DIPOLE_GATEWAY_PORT
export DIPOLE_AGENT_TEMPORAL_ADDRESS DIPOLE_AGENT_TEMPORAL_NAMESPACE DIPOLE_AGENT_TEMPORAL_TASK_QUEUE
export DIPOLE_AGENT_RELEASE_MANIFEST_FILE="${scratch_dir}/release-manifest.json"
export DIPOLE_INTERNAL_CERT_DIR="${scratch_dir}/certs"
export INTERNAL_CERT_DIR="${DIPOLE_INTERNAL_CERT_DIR}"
export DIPOLE_AGENT_MODEL_PROVIDER_NAME="compose-smoke"
export DIPOLE_AGENT_MODEL_BASE_URL="https://models.invalid/v1"
export DIPOLE_AGENT_MODEL_API_KEY="compose-smoke-no-network"
export DIPOLE_AGENT_MODEL_ROUTES="compose-smoke/deterministic"
export DIPOLE_AGENT_MODEL_CONTEXT_PROFILES='[{"route":"compose-smoke/deterministic","contextWindowTokens":32768,"utf8BytesPerToken":3,"safetyMarginBps":1500}]'
export DIPOLE_AGENT_MODEL_MAX_CALLS="1"
export DIPOLE_AGENT_MODEL_TOTAL_TIMEOUT_MS="1000"
export DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS="256"

cat >"${DIPOLE_AGENT_RELEASE_MANIFEST_FILE}" <<EOF
{
  "schemaVersion": "dipole.agent.release-manifest.v1",
  "candidateVersion": "${DIPOLE_AGENT_CANDIDATE_VERSION}",
  "runtimeId": "dipole-agent",
  "stage": "user_gray",
  "components": {
    "model": { "version": "v1", "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" },
    "prompt": { "version": "v1", "sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" },
    "capabilitySchema": { "version": "v1", "sha256": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc" },
    "memoryPolicy": { "version": "v1", "sha256": "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd" }
  },
  "offlineEvalSuiteSha256": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
  "createdAt": "2026-09-02T00:00:00.000Z"
}
EOF

compose_files=(
  -f "${compose_file}"
  -f "${root_dir}/deploy/microservices/agent-temporal-read-shadow.yml"
  -f "${root_dir}/deploy/microservices/agent-active.yml"
  -f "${root_dir}/deploy/microservices/agent-interactive-active.yml"
)
if [[ "${DIPOLE_MYSQL_AIO_COMPAT}" == "1" ]]; then
  compose_files+=(-f "${root_dir}/deploy/microservices/remote-gpu-mysql-aio-compat.yml")
fi

compose() {
  docker compose -p "${project_name}" "${compose_files[@]}" "$@"
}

cleanup() {
  local status=$?
  compose exec -T mysql mysql -uroot -proot123 dipole \
    -e "UPDATE agent_runtime_promotion_grants SET revoked_at = COALESCE(revoked_at, UTC_TIMESTAMP(3)) WHERE grant_uuid = '${grant_uuid}'" >/dev/null 2>&1 || true
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  else
    printf 'Interactive Agent Compose stack retained: project=%s\n' "${project_name}"
  fi
  rm -rf "${scratch_dir}"
  exit "${status}"
}
trap cleanup EXIT INT TERM

"${script_dir}/generate-internal-certs.sh"
compose config --quiet
compose up -d --wait

register_owner() {
  owner_uuid=$(compose exec -T agent node --input-type=module - "${owner_telephone}" <<'NODE'
const [telephone] = process.argv.slice(2);
const response = await fetch("http://gateway:8080/api/v1/auth/register", {
  method: "POST",
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ nickname: "Active Smoke Owner", telephone, password: "smoke-pass-123" })
});
const body = await response.json();
if (response.status !== 200 || typeof body?.data?.user?.uuid !== "string") {
  throw new Error(`register smoke owner failed: ${response.status} ${JSON.stringify(body)}`);
}
process.stdout.write(body.data.user.uuid);
NODE
)
  [[ -n "${owner_uuid}" ]] || { printf 'Core did not return a smoke owner UUID\n' >&2; return 1; }
}

register_owner

verify_runtime_status() {
  compose exec -T agent node --input-type=module - "${owner_telephone}" <<'NODE'
const [telephone] = process.argv.slice(2);
const loginResponse = await fetch("http://gateway:8080/api/v1/auth/login", {
  method: "POST",
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ telephone, password: "smoke-pass-123" })
});
const loginBody = await loginResponse.json();
const token = loginBody?.data?.token;
if (loginResponse.status !== 200 || typeof token !== "string" || token.length === 0) {
  throw new Error(`login for runtime status failed: ${loginResponse.status}`);
}
const response = await fetch("http://gateway:8080/api/v1/agent/status", {
  headers: { authorization: `Bearer ${token}` }
});
const body = await response.json();
const expected = {
  schemaVersion: "dipole.agent.runtime_status.v1",
  runtimeMode: "active",
  taskControlEnabled: true,
  interactiveMessageWritesEnabled: true
};
if (response.status !== 200 || !body || Object.entries(expected).some(([key, value]) => body[key] !== value)) {
  throw new Error(`runtime status mismatch: ${response.status}`);
}
NODE
}

verify_runtime_status

verify_definition_catalog() {
  compose exec -T agent node --input-type=module - "${owner_telephone}" <<'NODE'
const [telephone] = process.argv.slice(2);
const loginResponse = await fetch("http://gateway:8080/api/v1/auth/login", {
  method: "POST",
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ telephone, password: "smoke-pass-123" })
});
const loginBody = await loginResponse.json();
const token = loginBody?.data?.token;
if (loginResponse.status !== 200 || typeof token !== "string" || token.length === 0) {
  throw new Error(`login for Definition catalog failed: ${loginResponse.status}`);
}
const headers = { authorization: `Bearer ${token}` };
const created = [];
for (let attempt = 0; attempt < 2; attempt += 1) {
  const response = await fetch("http://gateway:8080/api/v1/agent/definitions", { method: "POST", headers });
  const body = await response.json();
  if (response.status !== 201 || typeof body?.definitionId !== "string" || body.version !== 1 ||
      body.agentId !== "UAI000000000000000001" || JSON.stringify(body.conversationScopes) !== JSON.stringify(["*"])) {
    throw new Error(`Definition create failed: ${response.status} ${JSON.stringify(body)}`);
  }
  created.push(body.definitionId);
}
if (created[0] !== created[1]) {
  throw new Error("Definition create replay returned a different identity");
}
const listedResponse = await fetch("http://gateway:8080/api/v1/agent/definitions?limit=20", { headers });
const listed = await listedResponse.json();
if (listedResponse.status !== 200 || !Array.isArray(listed?.definitions) || listed.definitions.length !== 1 ||
    listed.definitions[0]?.definitionId !== created[0] || JSON.stringify(listed.definitions[0]?.conversationScopes) !== JSON.stringify(["*"])) {
  throw new Error(`Definition list failed: ${listedResponse.status} ${JSON.stringify(listed)}`);
}
process.stdout.write(created[0]);
NODE
}

definition_uuid=$(verify_definition_catalog)

canonical_direct_conversation() {
  local first=$1
  local second=$2
  if [[ "${first}" < "${second}" ]]; then
    printf 'direct:%s:%s' "${first}" "${second}"
  else
    printf 'direct:%s:%s' "${second}" "${first}"
  fi
}

conversation_key=$(canonical_direct_conversation "${owner_uuid}" "${agent_uuid}")

mysql() {
  compose exec -T mysql mysql -N -B -uroot -proot123 dipole "$@"
}

definition_record=$(mysql -e "SELECT owner_uuid, agent_uuid, JSON_UNQUOTE(JSON_EXTRACT(permissions_json, '\$[0]')), JSON_UNQUOTE(JSON_EXTRACT(scopes_json, '\$[0].resource_id')) FROM agent_definition_versions WHERE definition_uuid = '${definition_uuid}' AND version = 1")
expected_definition_record="${owner_uuid}"$'\tUAI000000000000000001\tconversation.read\t*'
[[ "${definition_record}" == "${expected_definition_record}" ]] || { printf 'Definition record diverged: %q\n' "${definition_record}" >&2; exit 1; }
definition_count=$(mysql -e "SELECT COUNT(*) FROM agent_definition_versions WHERE tenant_id = 'dipole' AND owner_uuid = '${owner_uuid}' AND agent_uuid = '${agent_uuid}' AND version = 1")
[[ "${definition_count}" == "1" ]] || { printf 'Definition replay created %s records\n' "${definition_count}" >&2; exit 1; }
if [[ "${DIPOLE_AGENT_DEFINITION_ONLY}" == "1" ]]; then
  printf 'Agent Definition Compose smoke passed: project=%s\n' "${project_name}"
  exit 0
fi

mysql <<SQL
INSERT IGNORE INTO users (uuid, nickname, telephone, password_hash, status, created_at, updated_at) VALUES
  ('${agent_uuid}', 'Dipole Agent', '13900000002', 'smoke', 1, NOW(3), NOW(3));
INSERT INTO agent_definition_versions (
  definition_uuid, version, tenant_id, owner_uuid, agent_uuid, status,
  permissions_json, scopes_json, valid_from
) VALUES (
  '${definition_uuid}', 2, 'dipole', '${owner_uuid}', '${agent_uuid}', 'active',
  JSON_ARRAY('message.write'),
  JSON_ARRAY(JSON_OBJECT('resource_type', 'conversation', 'resource_id', '${conversation_key}', 'actions', JSON_ARRAY('write'))),
  UTC_TIMESTAMP(3)
);
INSERT INTO agent_runtime_promotion_grants (
  grant_uuid, tenant_id, runtime_id, candidate_version, definition_uuid, definition_version,
  policy_version, evidence_sha256, eval_suite_sha256, granted_by_uuid, reviewed_by_uuid, valid_from, expires_at
) VALUES (
  '${grant_uuid}', 'dipole', 'dipole-agent', '${DIPOLE_AGENT_CANDIDATE_VERSION}', '${definition_uuid}', 2,
  'dipole.agent.shadow-promotion-policy.v2',
  'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
  'U-SMOKE-GRANTOR', 'U-SMOKE-REVIEWER',
  DATE_SUB(UTC_TIMESTAMP(3), INTERVAL 1 MINUTE), DATE_ADD(UTC_TIMESTAMP(3), INTERVAL 15 MINUTE)
);
SQL

start_task() {
  local client_request_id=$1
  local goal=$2
  compose exec -T agent node --input-type=module - "${owner_uuid}" "${client_request_id}" "${goal}" <<'NODE'
const [ownerUuid, clientRequestId, goal] = process.argv.slice(2);
const response = await fetch("http://127.0.0.1:8091/internal/v1/agent/tasks", {
  method: "POST",
  headers: {
    "content-type": "application/json",
    "x-dipole-caller-service": "dipole-gateway",
    "x-dipole-service-token": process.env.DIPOLE_AGENT_CONTROL_SECRET,
    "x-dipole-principal-user-id": ownerUuid,
    "x-request-id": `REQ-${clientRequestId}`,
    "x-trace-id": `TRACE-${clientRequestId}`
  },
  body: JSON.stringify({ clientRequestId, goal })
});
const body = await response.json();
if (response.status !== 202 || typeof body.taskId !== "string") {
  throw new Error(`start task failed: ${response.status} ${JSON.stringify(body)}`);
}
process.stdout.write(body.taskId);
NODE
}

start_task_and_wait_for_locator() {
  local client_request_id=$1
  local goal=$2
  compose exec -T agent node --input-type=module - "${owner_telephone}" "${owner_uuid}" "${client_request_id}" "${goal}" <<'NODE'
const [telephone, ownerUuid, clientRequestId, goal] = process.argv.slice(2);

const loginResponse = await fetch("http://gateway:8080/api/v1/auth/login", {
  method: "POST",
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ telephone, password: "smoke-pass-123" })
});
const loginBody = await loginResponse.json();
const token = loginBody?.data?.token;
if (loginResponse.status !== 200 || typeof token !== "string" || token.length === 0) {
  throw new Error(`login for WebSocket locator failed: ${loginResponse.status}`);
}

const socket = new WebSocket(`ws://gateway:8080/api/v1/ws?token=${encodeURIComponent(token)}&device=smoke`);
let taskId = "";
let connected = false;
let resolveWaiting;
let rejectWaiting;
const waiting = new Promise((resolve, reject) => {
  resolveWaiting = resolve;
  rejectWaiting = reject;
});

socket.onmessage = event => {
  const packet = JSON.parse(String(event.data));
  if (packet?.type === "connected") {
    connected = true;
    return;
  }
  if (packet?.type !== "agent_task_waiting" || !taskId) return;
  const data = packet.data;
  if (data?.task_uuid === taskId && data.pending_kind === "approval" && Number.isInteger(data.revision) && data.revision > 0) {
    resolveWaiting(data);
  }
};
socket.onerror = () => rejectWaiting(new Error("WebSocket locator connection failed"));

for (let attempt = 0; attempt < 50 && !connected; attempt += 1) {
  await new Promise(resolve => setTimeout(resolve, 100));
}
if (!connected) {
  socket.close();
  throw new Error("WebSocket locator did not receive connected event");
}

const response = await fetch("http://127.0.0.1:8091/internal/v1/agent/tasks", {
  method: "POST",
  headers: {
    "content-type": "application/json",
    "x-dipole-caller-service": "dipole-gateway",
    "x-dipole-service-token": process.env.DIPOLE_AGENT_CONTROL_SECRET,
    "x-dipole-principal-user-id": ownerUuid,
    "x-request-id": `REQ-${clientRequestId}`,
    "x-trace-id": `TRACE-${clientRequestId}`
  },
  body: JSON.stringify({ clientRequestId, goal })
});
const body = await response.json();
if (response.status !== 202 || typeof body.taskId !== "string") {
  socket.close();
  throw new Error(`start task for WebSocket locator failed: ${response.status} ${JSON.stringify(body)}`);
}
taskId = body.taskId;

try {
  await Promise.race([
    waiting,
    new Promise((_, reject) => setTimeout(() => reject(new Error(`timed out waiting for WebSocket locator: ${taskId}`)), 30_000))
  ]);
} finally {
  socket.close();
}
process.stdout.write(taskId);
NODE
}

wait_for_approval() {
  local task_id=$1
  local approval_id=""
  for _ in $(seq 1 90); do
    approval_id=$(mysql -e "SELECT approval_uuid FROM agent_approvals WHERE task_uuid = '${task_id}' AND status = 'pending'" || true)
    [[ -n "${approval_id}" ]] && break
    sleep 1
  done
  [[ -n "${approval_id}" ]] || { printf 'Timed out waiting for approval: task=%s\n' "${task_id}" >&2; return 1; }
  printf '%s' "${approval_id}"
}

wait_for_agent_ready() {
  for _ in $(seq 1 30); do
    if compose exec -T agent node --input-type=module <<'NODE' >/dev/null 2>&1
const response = await fetch("http://127.0.0.1:8091/readyz");
process.exit(response.ok ? 0 : 1);
NODE
    then
      return 0
    fi
    sleep 1
  done
  printf 'Agent worker did not become ready after restart\n' >&2
  return 1
}

restart_agent_worker() {
  compose restart agent
  wait_for_agent_ready
}

resolve_twice() {
  local task_id=$1
  local approval_id=$2
  local decision=$3
  compose exec -T agent node --input-type=module - "${owner_uuid}" "${task_id}" "${approval_id}" "${decision}" <<'NODE'
const [ownerUuid, taskId, approvalId, decision] = process.argv.slice(2);
const headers = {
  "content-type": "application/json",
  "x-dipole-caller-service": "dipole-gateway",
  "x-dipole-service-token": process.env.DIPOLE_AGENT_CONTROL_SECRET,
  "x-dipole-principal-user-id": ownerUuid,
  "x-request-id": `REQ-${taskId}`,
  "x-trace-id": `TRACE-${taskId}`
};
const results = await Promise.all([0, 1].map(async (attempt) => {
  const response = await fetch(`http://127.0.0.1:8091/internal/v1/agent/tasks/${taskId}/approvals/${approvalId}`, {
    method: "POST", headers: { ...headers, "x-request-id": `${headers["x-request-id"]}-${attempt}` }, body: JSON.stringify({ decision })
  });
  return response.status;
}));
if (!results.includes(202) || results.some(status => status !== 202 && status !== 409)) {
  throw new Error(`approval replay did not serialize safely: ${results.join(",")}`);
}
process.stdout.write(results.join(","));
NODE
}

wait_for_workflow() {
  local task_id=$1
  local wanted_status=$2
  local result=""
  for _ in $(seq 1 90); do
    result=$(mysql -e "SELECT workflow_status FROM agent_tasks WHERE task_uuid = '${task_id}'" || true)
    [[ "${result}" == "${wanted_status}" ]] && break
    sleep 1
  done
  [[ "${result}" == "${wanted_status}" ]] || { printf 'Workflow status did not converge: task=%s got=%s want=%s\n' "${task_id}" "${result}" "${wanted_status}" >&2; return 1; }
}

denied_task=$(start_task_and_wait_for_locator "deny-$(openssl rand -hex 6)" "/send This message must stay uncommitted.")
denied_approval=$(wait_for_approval "${denied_task}")
resolve_twice "${denied_task}" "${denied_approval}" denied >/dev/null
wait_for_workflow "${denied_task}" cancelled
denied_effects=$(mysql -e "SELECT (SELECT COUNT(*) FROM agent_tool_invocations WHERE task_uuid = '${denied_task}'), (SELECT COUNT(*) FROM messages WHERE sender_uuid = '${agent_uuid}' AND target_uuid = '${owner_uuid}')")
[[ "${denied_effects}" == $'0\t0' ]] || { printf 'Denied task produced side effects: %q\n' "${denied_effects}" >&2; exit 1; }

approved_task=$(start_task "approve-$(openssl rand -hex 6)" "/send Compose approval replay committed exactly one message.")
approved_approval=$(wait_for_approval "${approved_task}")
restart_agent_worker
resolve_twice "${approved_task}" "${approved_approval}" approved >/dev/null
wait_for_workflow "${approved_task}" completed

approved_effects=$(mysql -e "SELECT
  (SELECT COUNT(*) FROM agent_tool_invocations WHERE task_uuid = '${approved_task}' AND status = 'completed'),
  (SELECT COUNT(*) FROM agent_approvals WHERE approval_uuid = '${approved_approval}' AND status = 'consumed'),
  (SELECT COUNT(*) FROM messages WHERE sender_uuid = '${agent_uuid}' AND target_uuid = '${owner_uuid}'),
  (SELECT COUNT(DISTINCT client_message_id) FROM messages WHERE sender_uuid = '${agent_uuid}' AND target_uuid = '${owner_uuid}'),
  (SELECT COUNT(*) FROM user_sync_inbox AS inbox JOIN messages AS message ON message.uuid = inbox.message_uuid WHERE message.sender_uuid = '${agent_uuid}' AND message.target_uuid = '${owner_uuid}')")
[[ "${approved_effects}" == $'1\t1\t1\t1\t2' ]] || { printf 'Approved task effects diverged: %q\n' "${approved_effects}" >&2; exit 1; }

revoked=$(mysql -e "UPDATE agent_runtime_promotion_grants SET revoked_at = UTC_TIMESTAMP(3) WHERE grant_uuid = '${grant_uuid}' AND revoked_at IS NULL; SELECT ROW_COUNT();")
[[ "${revoked}" == "1" ]] || { printf 'Temporary promotion grant revocation failed: %q\n' "${revoked}" >&2; exit 1; }

printf 'Interactive Agent active Compose smoke passed: owner WebSocket received the waiting locator; deny has zero effects; duplicate approval converged to one Tool invocation, one message, and two Sync inbox entries.\n'
