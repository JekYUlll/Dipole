#!/usr/bin/env bash
set -euo pipefail

# Exercises the public, read-only interactive Agent Task controls in an isolated
# Compose project. The local model stub keeps prompts and credentials on-host.

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
project_name="${COMPOSE_PROJECT_NAME:-dipole-agent-interactive-shadow-${RANDOM}-$$}"
model_source="${DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_SOURCE:-stub}"
execution_profile="${DIPOLE_AGENT_INTERACTIVE_READ_PROFILE:-shadow}"
owner_telephone="13900000005"
foreign_telephone="13900000006"
agent_uuid="UAI000000000000000001"
grant_uuid="PROMOTION-READ-ACTIVE-${RANDOM}-$$"

command -v docker >/dev/null 2>&1 || { printf 'Docker is required\n' >&2; exit 2; }
command -v openssl >/dev/null 2>&1 || { printf 'openssl is required\n' >&2; exit 2; }
[[ "${model_source}" == "stub" || "${model_source}" == "provider" ]] || { printf 'DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_SOURCE must be stub or provider\n' >&2; exit 2; }
[[ "${execution_profile}" == "shadow" || "${execution_profile}" == "active" ]] || { printf 'DIPOLE_AGENT_INTERACTIVE_READ_PROFILE must be shadow or active\n' >&2; exit 2; }
if [[ "${model_source}" == "provider" ]]; then
  : "${DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_ENV_FILE:?DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_ENV_FILE is required for provider mode}"
  [[ -f "${DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_ENV_FILE}" ]] || { printf 'DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_ENV_FILE must name a file\n' >&2; exit 2; }
fi
scratch_dir=$(mktemp -d "${TMPDIR:-/tmp}/dipole-agent-interactive-shadow.XXXXXX")

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
: "${DIPOLE_AGENT_CONTROL_SECRET:=$(openssl rand -hex 32)}"
if [[ "${execution_profile}" == "active" ]]; then
  : "${DIPOLE_AGENT_KAFKA_GROUP_ID:=dipole-agent-active-read-${RANDOM}-$$}"
  : "${DIPOLE_AGENT_INTERACTIVE_SHADOW_TASK_QUEUE:=dipole-agent-active-read-${RANDOM}-$$}"
else
  : "${DIPOLE_AGENT_KAFKA_GROUP_ID:=dipole-agent-shadow-interactive-${RANDOM}-$$}"
  : "${DIPOLE_AGENT_INTERACTIVE_SHADOW_TASK_QUEUE:=dipole-agent-interactive-shadow-${RANDOM}-$$}"
fi
: "${DIPOLE_AGENT_TEMPORAL_ADDRESS:=temporal:7233}"
: "${DIPOLE_AGENT_TEMPORAL_NAMESPACE:=default}"
: "${DIPOLE_GATEWAY_BIND_ADDRESS:=127.0.0.1}"
: "${DIPOLE_GATEWAY_PORT:=$((18000 + RANDOM % 2000))}"
: "${DIPOLE_MYSQL_AIO_COMPAT:=0}"
[[ "${DIPOLE_MYSQL_AIO_COMPAT}" == "0" || "${DIPOLE_MYSQL_AIO_COMPAT}" == "1" ]] || { printf 'DIPOLE_MYSQL_AIO_COMPAT must be 0 or 1\n' >&2; exit 2; }

export DIPOLE_MIGRATE_IMAGE DIPOLE_CORE_IMAGE DIPOLE_GATEWAY_IMAGE DIPOLE_MESSAGE_IMAGE DIPOLE_SYNC_IMAGE DIPOLE_AGENT_IMAGE
export DIPOLE_INTERNAL_RPC_SHARED_SECRET DIPOLE_AGENT_CONTROL_SECRET DIPOLE_AGENT_KAFKA_GROUP_ID
export DIPOLE_AGENT_INTERACTIVE_SHADOW_TASK_QUEUE DIPOLE_AGENT_TEMPORAL_ADDRESS DIPOLE_AGENT_TEMPORAL_NAMESPACE
export DIPOLE_GATEWAY_BIND_ADDRESS DIPOLE_GATEWAY_PORT
export DIPOLE_INTERNAL_CERT_DIR="${scratch_dir}/certs"
export INTERNAL_CERT_DIR="${DIPOLE_INTERNAL_CERT_DIR}"
model_env_file=""
if [[ "${model_source}" == "stub" ]]; then
  export DIPOLE_AGENT_MODEL_PROVIDER_NAME="compose-smoke"
  export DIPOLE_AGENT_MODEL_BASE_URL="http://127.0.0.1:8089/v1"
  export DIPOLE_AGENT_MODEL_API_KEY="compose-smoke-no-network"
  export DIPOLE_AGENT_MODEL_ROUTES="compose-smoke/deterministic"
  export DIPOLE_AGENT_MODEL_CONTEXT_PROFILES='[{"route":"compose-smoke/deterministic","contextWindowTokens":32768,"utf8BytesPerToken":3,"safetyMarginBps":1500}]'
  export DIPOLE_AGENT_MODEL_OUTPUT_MODE="json_text"
  export DIPOLE_AGENT_MODEL_MAX_CALLS="1"
  export DIPOLE_AGENT_MODEL_TOTAL_TIMEOUT_MS="5000"
  export DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS="256"
  export DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_STUB_FILE="${scratch_dir}/model-stub.mjs"

  cat >"${DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_STUB_FILE}" <<'NODE'
import http from "node:http";
const body = JSON.stringify({ id: "interactive-shadow-smoke", object: "chat.completion", choices: [{ index: 0, finish_reason: "stop", message: { role: "assistant", content: '{"summary":"select a conversation before reading it","steps":[{"capabilityId":"conversation.list","input":{"limit":20}},{"capabilityId":"conversation.read","input":{"conversationId":"$discovered.previous","limit":20}}]}' } }], usage: { prompt_tokens: 1, completion_tokens: 1, total_tokens: 2 } });
http.createServer((request, response) => {
  if (request.method !== "POST" || request.url !== "/v1/chat/completions") { response.writeHead(404).end(); return; }
  request.resume(); request.on("end", () => response.writeHead(200, { "content-type": "application/json" }).end(body));
}).listen(8089, "0.0.0.0");
NODE
else
  model_env_file="${DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_ENV_FILE}"
fi

compose_files=(-f "${root_dir}/deploy/compose/docker-compose.microservices.yml")
if [[ "${execution_profile}" == "shadow" ]]; then
  compose_files+=(
    -f "${root_dir}/deploy/microservices/agent-temporal-read-shadow.yml"
    -f "${root_dir}/deploy/microservices/agent-interactive-shadow.yml"
  )
else
  compose_files+=(
    -f "${root_dir}/deploy/microservices/agent-active.yml"
    -f "${root_dir}/deploy/microservices/agent-interactive-read-active.yml"
  )
fi
if [[ "${model_source}" == "stub" ]]; then
  compose_files+=( -f "${root_dir}/deploy/microservices/agent-interactive-shadow-smoke.yml" )
else
  compose_files+=(
    -f "${root_dir}/deploy/microservices/agent-ai-sdk-shadow.yml"
    -f "${root_dir}/deploy/microservices/agent-deepseek-v4-flash-shadow.yml"
  )
fi
[[ "${DIPOLE_MYSQL_AIO_COMPAT}" == "0" ]] || compose_files+=( -f "${root_dir}/deploy/microservices/remote-gpu-mysql-aio-compat.yml" )
compose() {
  local env_args=()
  [[ -z "${model_env_file}" ]] || env_args=(--env-file "${model_env_file}")
  docker compose "${env_args[@]}" -p "${project_name}" "${compose_files[@]}" "$@"
}

cleanup() {
  local status=$?
  if [[ "${status}" != "0" ]]; then
    compose logs --no-color agent >&2 || true
  fi
  if [[ "${execution_profile}" == "active" ]]; then
    compose exec -T mysql mysql -uroot -proot123 dipole \
      -e "UPDATE agent_runtime_promotion_grants SET revoked_at = COALESCE(revoked_at, UTC_TIMESTAMP(3)) WHERE grant_uuid = '${grant_uuid}'" >/dev/null 2>&1 || true
  fi
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    compose down --volumes --remove-orphans >/dev/null 2>&1 || true
    rm -rf "${scratch_dir}"
  else
    printf 'Interactive shadow Compose stack retained: project=%s scratch=%s\n' "${project_name}" "${scratch_dir}" >&2
  fi
  exit "${status}"
}
trap cleanup EXIT INT TERM

"${root_dir}/scripts/generate-internal-certs.sh"
compose config --quiet
compose up -d --wait

compose exec -T mysql mysql -uroot -proot123 dipole -e "INSERT IGNORE INTO users (uuid, nickname, telephone, password_hash, status, created_at, updated_at) VALUES ('${agent_uuid}', 'Dipole Agent', '13900000002', 'smoke', 1, NOW(3), NOW(3));"

result=$(compose exec -T agent node --input-type=module - "${owner_telephone}" "${foreign_telephone}" "${agent_uuid}" "${execution_profile}" <<'NODE'
const [ownerTelephone, foreignTelephone, agentUuid, executionProfile] = process.argv.slice(2);

async function request(method, url, token, body) {
  const response = await fetch(url, { method, headers: { authorization: `Bearer ${token}`, ...(body === undefined ? {} : { "content-type": "application/json" }) }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
  const payload = await response.json().catch(() => undefined);
  return { response, payload };
}
async function registerAndLogin(telephone, nickname) {
  const register = await fetch("http://core:8081/api/v1/auth/register", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ telephone, nickname, password: "smoke-pass-123" }) });
  const ownerUuid = (await register.json())?.data?.user?.uuid;
  if (register.status !== 200 || typeof ownerUuid !== "string") throw new Error(`register failed: ${register.status}`);
  const login = await fetch("http://core:8081/api/v1/auth/login", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ telephone, password: "smoke-pass-123" }) });
  const token = (await login.json())?.data?.token;
  if (login.status !== 200 || typeof token !== "string") throw new Error(`login failed: ${login.status}`);
  return { ownerUuid, token };
}
async function sendBootstrapMessage(token, targetUuid, label) {
  const socket = new WebSocket(`ws://gateway:8080/api/v1/ws?token=${encodeURIComponent(token)}&device=interactive-shadow-smoke`);
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error("bootstrap message timeout")), 15_000);
    socket.addEventListener("open", () => socket.send(JSON.stringify({ type: "chat.send", data: { target_uuid: targetUuid, content: "establish interactive Agent scope", client_message_id: `interactive-bootstrap-${label}-${Date.now()}` } })));
    socket.addEventListener("message", ({ data }) => {
      const event = JSON.parse(String(data));
      if (event?.type === "chat.sent") { clearTimeout(timer); socket.close(); resolve(); }
      if (event?.type === "error") reject(new Error(`bootstrap message failed: ${JSON.stringify(event.data)}`));
    });
    socket.addEventListener("error", () => reject(new Error("bootstrap websocket failed")));
  });
}

const owner = await registerAndLogin(ownerTelephone, "Shadow Owner");
const foreign = await registerAndLogin(foreignTelephone, "Shadow Foreign");
await sendBootstrapMessage(owner.token, agentUuid, "agent");
const definition = await request("POST", "http://gateway:8080/api/v1/agent/definitions", owner.token);
if (definition.response.status !== 201) throw new Error(`definition failed: ${definition.response.status}`);
if (executionProfile === "active") {
  if (typeof definition.payload?.definitionId !== "string") throw new Error("active Definition did not return an ID");
  process.stdout.write(`${owner.ownerUuid}\t${definition.payload.definitionId}`);
  process.exit(0);
}

const taskBody = { client_request_id: `interactive-shadow-${Date.now()}`, goal: "List my conversations and read the available Agent conversation for a summary." };
const first = await request("POST", "http://gateway:8080/api/v1/agent/tasks", owner.token, taskBody);
const second = await request("POST", "http://gateway:8080/api/v1/agent/tasks", owner.token, taskBody);
if (first.response.status !== 202 || second.response.status !== 202 || typeof first.payload?.taskId !== "string" || first.payload.taskId !== second.payload?.taskId) {
  throw new Error(`duplicate task start diverged: ${first.response.status}/${second.response.status}`);
}
const taskId = first.payload.taskId;

const foreignRead = await request("GET", `http://gateway:8080/api/v1/agent/tasks/${taskId}`, foreign.token);
if (![403, 404].includes(foreignRead.response.status)) throw new Error(`foreign owner read was not rejected: ${foreignRead.response.status}`);

let lastTaskStatus = "unavailable";
for (let attempt = 0; attempt < 90; attempt += 1) {
  const read = await request("GET", `http://gateway:8080/api/v1/agent/tasks/${taskId}`, owner.token);
  if (read.response.status === 200 && typeof read.payload?.status === "string") lastTaskStatus = read.payload.status;
  if (read.response.status === 200 && read.payload?.status === "completed") { process.stdout.write(`${owner.ownerUuid}\t${taskId}`); process.exit(0); }
  await new Promise(resolve => setTimeout(resolve, 1_000));
}
throw new Error(`interactive task did not complete (last status: ${lastTaskStatus})`);
NODE
)
IFS=$'\t' read -r owner_uuid task_uuid <<<"${result}"

if [[ "${execution_profile}" == "active" ]]; then
  IFS=$'\t' read -r owner_uuid definition_uuid <<<"${result}"
  compose exec -T mysql mysql -uroot -proot123 dipole <<SQL
INSERT INTO agent_runtime_promotion_grants (
  grant_uuid, tenant_id, runtime_id, candidate_version, definition_uuid, definition_version,
  policy_version, evidence_sha256, eval_suite_sha256, granted_by_uuid, reviewed_by_uuid, valid_from, expires_at
) VALUES (
  '${grant_uuid}', 'dipole', 'dipole-agent', '${DIPOLE_AGENT_CANDIDATE_VERSION:-agent-runtime@dev}', '${definition_uuid}', 1,
  'dipole.agent.shadow-promotion-policy.v2',
  'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
  'U-SMOKE-GRANTOR', 'U-SMOKE-REVIEWER',
  DATE_SUB(UTC_TIMESTAMP(3), INTERVAL 1 MINUTE), DATE_ADD(UTC_TIMESTAMP(3), INTERVAL 15 MINUTE)
);
SQL
  task_uuid=$(compose exec -T agent node --input-type=module - "${owner_telephone}" "${foreign_telephone}" <<'NODE'
const [ownerTelephone, foreignTelephone] = process.argv.slice(2);
async function login(telephone) {
  const response = await fetch("http://core:8081/api/v1/auth/login", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ telephone, password: "smoke-pass-123" }) });
  const token = (await response.json())?.data?.token;
  if (response.status !== 200 || typeof token !== "string") throw new Error(`login failed: ${response.status}`);
  return token;
}
async function request(method, url, token, body) {
  const response = await fetch(url, { method, headers: { authorization: `Bearer ${token}`, ...(body === undefined ? {} : { "content-type": "application/json" }) }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
  return { response, payload: await response.json().catch(() => undefined) };
}
const ownerToken = await login(ownerTelephone);
const foreignToken = await login(foreignTelephone);
const body = { client_request_id: `interactive-read-active-${Date.now()}`, goal: "List my conversations and read the available Agent conversation for a summary." };
const first = await request("POST", "http://gateway:8080/api/v1/agent/tasks", ownerToken, body);
const second = await request("POST", "http://gateway:8080/api/v1/agent/tasks", ownerToken, body);
if (first.response.status !== 202 || second.response.status !== 202 || typeof first.payload?.taskId !== "string" || first.payload.taskId !== second.payload?.taskId) throw new Error(`duplicate task start diverged: ${first.response.status}/${second.response.status}`);
const taskId = first.payload.taskId;
const foreignRead = await request("GET", `http://gateway:8080/api/v1/agent/tasks/${taskId}`, foreignToken);
if (![403, 404].includes(foreignRead.response.status)) throw new Error(`foreign owner read was not rejected: ${foreignRead.response.status}`);
let status = "unavailable";
for (let attempt = 0; attempt < 90; attempt += 1) {
  const current = await request("GET", `http://gateway:8080/api/v1/agent/tasks/${taskId}`, ownerToken);
  if (current.response.status === 200 && typeof current.payload?.status === "string") status = current.payload.status;
  if (current.response.status === 200 && current.payload?.status === "completed") { process.stdout.write(taskId); process.exit(0); }
  await new Promise(resolve => setTimeout(resolve, 1_000));
}
throw new Error(`active read task did not complete (last status: ${status})`);
NODE
)
fi

if [[ "${execution_profile}" == "shadow" ]]; then
  effects=$(compose exec -T mysql mysql -N -B -uroot -proot123 dipole -e "SELECT (SELECT COUNT(*) FROM agent_tasks WHERE task_uuid = '${task_uuid}' AND status = 'completed' AND workflow_status = 'completed'), (SELECT COUNT(*) FROM agent_runs WHERE task_uuid = '${task_uuid}' AND status = 'completed'), (SELECT COUNT(*) FROM agent_shadow_steps WHERE task_uuid = '${task_uuid}' AND status = 'completed'), (SELECT COUNT(*) FROM messages WHERE sender_uuid = '${agent_uuid}' AND target_uuid = '${owner_uuid}')")
  [[ "${effects}" == $'1\t1\t2\t0' ]] || { printf 'interactive read task wrote or did not complete the read trajectory: %q\n' "${effects}" >&2; exit 1; }
else
  effects=$(compose exec -T mysql mysql -N -B -uroot -proot123 dipole -e "SELECT (SELECT COUNT(*) FROM agent_tasks WHERE task_uuid = '${task_uuid}' AND status = 'completed' AND workflow_status = 'completed'), (SELECT COUNT(*) FROM agent_runs WHERE task_uuid = '${task_uuid}' AND status = 'completed'), (SELECT COUNT(*) FROM agent_model_runs WHERE task_uuid = '${task_uuid}' AND status = 'completed'), (SELECT COUNT(*) FROM messages WHERE sender_uuid = '${agent_uuid}' AND target_uuid = '${owner_uuid}')")
  IFS=$'\t' read -r task_count run_count model_count message_count <<<"${effects}"
  [[ "${task_count}" == "1" && "${run_count}" == "1" && "${model_count}" =~ ^[1-9][0-9]*$ && "${message_count}" == "0" ]] || { printf 'active read task wrote or did not complete: %q\n' "${effects}" >&2; exit 1; }
fi

printf 'Interactive Agent %s Compose smoke passed: authenticated Gateway task creation is idempotent, owner-scoped, read-only, and completes its two-Step read trajectory.\n' "${execution_profile}"
