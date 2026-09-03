#!/usr/bin/env bash
set -euo pipefail

# Exercises the public, read-only interactive Agent Task controls in an isolated
# Compose project. The local model stub keeps prompts and credentials on-host.

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
project_name="${COMPOSE_PROJECT_NAME:-dipole-agent-interactive-shadow-${RANDOM}-$$}"
scratch_dir=$(mktemp -d "${TMPDIR:-/tmp}/dipole-agent-interactive-shadow.XXXXXX")
owner_telephone="13900000005"
foreign_telephone="13900000006"
agent_uuid="UAI000000000000000001"

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
: "${DIPOLE_AGENT_CONTROL_SECRET:=$(openssl rand -hex 32)}"
: "${DIPOLE_AGENT_KAFKA_GROUP_ID:=dipole-agent-shadow-interactive-${RANDOM}-$$}"
: "${DIPOLE_AGENT_INTERACTIVE_SHADOW_TASK_QUEUE:=dipole-agent-interactive-shadow-${RANDOM}-$$}"
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
const body = JSON.stringify({ id: "interactive-shadow-smoke", object: "chat.completion", choices: [{ index: 0, finish_reason: "stop", message: { role: "assistant", content: '{"summary":"interactive shadow smoke","steps":[]}' } }], usage: { prompt_tokens: 1, completion_tokens: 1, total_tokens: 2 } });
http.createServer((request, response) => {
  if (request.method !== "POST" || request.url !== "/v1/chat/completions") { response.writeHead(404).end(); return; }
  request.resume(); request.on("end", () => response.writeHead(200, { "content-type": "application/json" }).end(body));
}).listen(8089, "0.0.0.0");
NODE

compose_files=(
  -f "${root_dir}/deploy/compose/docker-compose.microservices.yml"
  -f "${root_dir}/deploy/microservices/agent-temporal-read-shadow.yml"
  -f "${root_dir}/deploy/microservices/agent-interactive-shadow.yml"
  -f "${root_dir}/deploy/microservices/agent-interactive-shadow-smoke.yml"
)
[[ "${DIPOLE_MYSQL_AIO_COMPAT}" == "0" ]] || compose_files+=( -f "${root_dir}/deploy/microservices/remote-gpu-mysql-aio-compat.yml" )
compose() { docker compose -p "${project_name}" "${compose_files[@]}" "$@"; }

cleanup() {
  local status=$?
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

result=$(compose exec -T agent node --input-type=module - "${owner_telephone}" "${foreign_telephone}" "${agent_uuid}" <<'NODE'
const [ownerTelephone, foreignTelephone, agentUuid] = process.argv.slice(2);

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
async function sendBootstrapMessage(token) {
  const socket = new WebSocket(`ws://gateway:8080/api/v1/ws?token=${encodeURIComponent(token)}&device=interactive-shadow-smoke`);
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error("bootstrap message timeout")), 15_000);
    socket.addEventListener("open", () => socket.send(JSON.stringify({ type: "chat.send", data: { target_uuid: agentUuid, content: "establish interactive Agent scope", client_message_id: `interactive-bootstrap-${Date.now()}` } })));
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
await sendBootstrapMessage(owner.token);
const definition = await request("POST", "http://gateway:8080/api/v1/agent/definitions", owner.token);
if (definition.response.status !== 201) throw new Error(`definition failed: ${definition.response.status}`);

const taskBody = { client_request_id: `interactive-shadow-${Date.now()}`, goal: "Summarize my current Agent conversation and ask before reading it." };
const first = await request("POST", "http://gateway:8080/api/v1/agent/tasks", owner.token, taskBody);
const second = await request("POST", "http://gateway:8080/api/v1/agent/tasks", owner.token, taskBody);
if (first.response.status !== 202 || second.response.status !== 202 || typeof first.payload?.taskId !== "string" || first.payload.taskId !== second.payload?.taskId) {
  throw new Error(`duplicate task start diverged: ${first.response.status}/${second.response.status}`);
}
const taskId = first.payload.taskId;

const foreignRead = await request("GET", `http://gateway:8080/api/v1/agent/tasks/${taskId}`, foreign.token);
if (![403, 404].includes(foreignRead.response.status)) throw new Error(`foreign owner read was not rejected: ${foreignRead.response.status}`);

let state;
for (let attempt = 0; attempt < 90; attempt += 1) {
  const read = await request("GET", `http://gateway:8080/api/v1/agent/tasks/${taskId}`, owner.token);
  if (read.response.status === 200 && read.payload?.status === "waiting_input") { state = read.payload; break; }
  await new Promise(resolve => setTimeout(resolve, 1_000));
}
if (state === undefined) throw new Error("interactive task did not enter waiting_input");
const cancelled = await request("POST", `http://gateway:8080/api/v1/agent/tasks/${taskId}/cancel`, owner.token, { reason: "smoke_cancelled" });
if (cancelled.response.status !== 202) throw new Error(`task cancel failed: ${cancelled.response.status}`);
for (let attempt = 0; attempt < 60; attempt += 1) {
  const read = await request("GET", `http://gateway:8080/api/v1/agent/tasks/${taskId}`, owner.token);
  if (read.response.status === 200 && read.payload?.status === "cancelled") { process.stdout.write(`${owner.ownerUuid}\t${taskId}`); process.exit(0); }
  await new Promise(resolve => setTimeout(resolve, 1_000));
}
throw new Error("interactive task did not cancel");
NODE
)
IFS=$'\t' read -r owner_uuid task_uuid <<<"${result}"

effects=$(compose exec -T mysql mysql -N -B -uroot -proot123 dipole -e "SELECT (SELECT COUNT(*) FROM agent_tasks WHERE task_uuid = '${task_uuid}' AND status = 'cancelled' AND workflow_status = 'cancelled'), (SELECT COUNT(*) FROM agent_runs WHERE task_uuid = '${task_uuid}' AND status = 'cancelled'), (SELECT COUNT(*) FROM messages WHERE sender_uuid = '${agent_uuid}' AND target_uuid = '${owner_uuid}')")
[[ "${effects}" == $'1\t1\t0' ]] || { printf 'interactive read task wrote or failed to converge: %q\n' "${effects}" >&2; exit 1; }

printf 'Interactive Agent shadow Compose smoke passed: authenticated Gateway task creation is idempotent, owner-scoped, cancellable, and read-only.\n'
