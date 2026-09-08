#!/usr/bin/env bash
set -euo pipefail

# Exercises the opt-in subscription-active path in an isolated project. The
# deterministic local model avoids sending smoke data or credentials off-host.

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
project_name="${COMPOSE_PROJECT_NAME:-dipole-agent-subscription-active-${RANDOM}-$$}"
scratch_dir=$(mktemp -d "${TMPDIR:-/tmp}/dipole-agent-subscription-active.XXXXXX")
owner_telephone="13900000004"
agent_uuid="UAI000000000000000001"
grant_uuid="PROMOTION-SUBSCRIPTION-ACTIVE-${RANDOM}-$$"
model_source="${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_MODEL_SOURCE:-stub}"
promotion_mode="${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_PROMOTION_MODE:-fixture}"

command -v docker >/dev/null 2>&1 || { printf 'Docker is required\n' >&2; exit 2; }
command -v openssl >/dev/null 2>&1 || { printf 'openssl is required\n' >&2; exit 2; }
[[ "${model_source}" == "stub" || "${model_source}" == "provider" ]] || { printf 'DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_MODEL_SOURCE must be stub or provider\n' >&2; exit 2; }
[[ "${promotion_mode}" == "fixture" || "${promotion_mode}" == "control" || "${promotion_mode}" == "publication" ]] || { printf 'DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_PROMOTION_MODE must be fixture, control, or publication\n' >&2; exit 2; }
if [[ "${model_source}" == "provider" ]]; then
  : "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_MODEL_ENV_FILE:?DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_MODEL_ENV_FILE is required for provider mode}"
  [[ -f "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_MODEL_ENV_FILE}" ]] || { printf 'DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_MODEL_ENV_FILE must name a file\n' >&2; exit 2; }
fi

if [[ "${BUILD_IMAGE:-0}" == "1" ]]; then
  "${root_dir}/scripts/docker-build.sh" backend
  DIPOLE_MICROSERVICE_IMAGE_SERVICES="migrate,core,gateway,message,sync" \
    "${root_dir}/scripts/docker-build-microservice-images.sh"
fi

: "${DIPOLE_MIGRATE_IMAGE:=dipole-migrate:latest}"
: "${DIPOLE_CORE_IMAGE:=dipole-core:latest}"
: "${DIPOLE_GATEWAY_IMAGE:=dipole-gateway:latest}"
: "${DIPOLE_MESSAGE_IMAGE:=dipole-message:latest}"
: "${DIPOLE_SYNC_IMAGE:=dipole-sync:latest}"
: "${DIPOLE_AGENT_IMAGE:=dipole-agent:latest}"
: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
# Candidate versions pass through the public Gateway control API in control
# mode, so keep the default within its public ID character contract.
: "${DIPOLE_AGENT_CANDIDATE_VERSION:=agent-runtime.subscription-active-compose-smoke}"
: "${DIPOLE_AGENT_ACTIVE_KAFKA_GROUP_ID:=dipole-agent-active-subscription-smoke-${RANDOM}-$$}"
: "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_KAFKA_GROUP_ID:=dipole-agent-subscription-active-smoke-${RANDOM}-$$}"
: "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_TASK_QUEUE:=dipole-agent-subscription-smoke-${RANDOM}-$$}"
: "${DIPOLE_AGENT_TEMPORAL_ADDRESS:=temporal:7233}"
: "${DIPOLE_AGENT_TEMPORAL_NAMESPACE:=default}"
: "${DIPOLE_AGENT_TEMPORAL_TASK_QUEUE:=${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_TASK_QUEUE}}"
: "${DIPOLE_GATEWAY_BIND_ADDRESS:=127.0.0.1}"
: "${DIPOLE_GATEWAY_PORT:=$((18000 + RANDOM % 2000))}"
: "${DIPOLE_MYSQL_AIO_COMPAT:=0}"
: "${DIPOLE_AGENT_SUBSCRIPTION_AUTOREPLY:=0}"
[[ "${DIPOLE_MYSQL_AIO_COMPAT}" == "0" || "${DIPOLE_MYSQL_AIO_COMPAT}" == "1" ]] || { printf 'DIPOLE_MYSQL_AIO_COMPAT must be 0 or 1\n' >&2; exit 2; }
[[ "${DIPOLE_AGENT_SUBSCRIPTION_AUTOREPLY}" == "0" || "${DIPOLE_AGENT_SUBSCRIPTION_AUTOREPLY}" == "1" ]] || { printf 'DIPOLE_AGENT_SUBSCRIPTION_AUTOREPLY must be 0 or 1\n' >&2; exit 2; }
if [[ "${promotion_mode}" == "control" || "${promotion_mode}" == "publication" ]]; then
  export DIPOLE_GATEWAY_AGENT_PROMOTION_ENABLED=true
fi

export DIPOLE_MIGRATE_IMAGE DIPOLE_CORE_IMAGE DIPOLE_GATEWAY_IMAGE DIPOLE_MESSAGE_IMAGE DIPOLE_SYNC_IMAGE DIPOLE_AGENT_IMAGE
export DIPOLE_INTERNAL_RPC_SHARED_SECRET DIPOLE_AGENT_CANDIDATE_VERSION DIPOLE_AGENT_ACTIVE_KAFKA_GROUP_ID
export DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_KAFKA_GROUP_ID DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_TASK_QUEUE
export DIPOLE_AGENT_TEMPORAL_ADDRESS DIPOLE_AGENT_TEMPORAL_NAMESPACE DIPOLE_AGENT_TEMPORAL_TASK_QUEUE
export DIPOLE_GATEWAY_BIND_ADDRESS DIPOLE_GATEWAY_PORT DIPOLE_AGENT_SUBSCRIPTION_AUTOREPLY
export DIPOLE_AGENT_RELEASE_MANIFEST_FILE="${scratch_dir}/release-manifest.json"
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
  export DIPOLE_AGENT_SUBSCRIPTION_MODEL_STUB_FILE="${scratch_dir}/model-stub.mjs"
else
  model_env_file="${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_MODEL_ENV_FILE}"
fi

model_summary="subscription active smoke"
[[ "${DIPOLE_AGENT_SUBSCRIPTION_AUTOREPLY}" == "0" ]] || model_summary="subscription autonomous reply smoke"

cat >"${DIPOLE_AGENT_RELEASE_MANIFEST_FILE}" <<EOF
{"schemaVersion":"dipole.agent.release-manifest.v1","candidateVersion":"${DIPOLE_AGENT_CANDIDATE_VERSION}","runtimeId":"dipole-agent","stage":"user_gray","components":{"model":{"version":"v1","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"prompt":{"version":"v1","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},"capabilitySchema":{"version":"v1","sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},"memoryPolicy":{"version":"v1","sha256":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"}},"offlineEvalSuiteSha256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","createdAt":"2026-09-02T00:00:00.000Z"}
EOF

if [[ "${model_source}" == "stub" ]]; then
cat >"${DIPOLE_AGENT_SUBSCRIPTION_MODEL_STUB_FILE}" <<NODE
import http from "node:http";
const summary = ${model_summary@Q};
http.createServer((request, response) => {
  if (request.method !== "POST" || request.url !== "/v1/chat/completions") { response.writeHead(404).end(); return; }
  let raw = "";
  request.setEncoding("utf8");
  request.on("data", chunk => { raw += chunk; });
  request.on("end", () => {
    const requestBody = JSON.parse(raw);
    const schema = requestBody?.response_format?.json_schema?.schema;
    // The planner requires a strict Plan shape. Reply and synthesis stages use
    // a distinct strict summary shape, so a shared stub must mirror both.
    const expectsPlan = schema?.properties?.steps !== undefined || raw.includes("steps");
    const content = JSON.stringify(expectsPlan ? { summary, steps: [] } : { summary });
    const body = JSON.stringify({ id: "subscription-active-smoke", object: "chat.completion", choices: [{ index: 0, finish_reason: "stop", message: { role: "assistant", content } }], usage: { prompt_tokens: 1, completion_tokens: 1, total_tokens: 2 } });
    response.writeHead(200, { "content-type": "application/json" }).end(body);
  });
}).listen(8089, "0.0.0.0");
NODE
fi

compose_files=(
  -f "${root_dir}/deploy/compose/docker-compose.microservices.yml"
  -f "${root_dir}/deploy/microservices/agent-temporal-read-shadow.yml"
  -f "${root_dir}/deploy/microservices/agent-subscription-shadow.yml"
  -f "${root_dir}/deploy/microservices/agent-active.yml"
  -f "${root_dir}/deploy/microservices/agent-subscription-active.yml"
)
if [[ "${model_source}" == "stub" ]]; then
  compose_files+=( -f "${root_dir}/deploy/microservices/agent-subscription-active-smoke.yml" )
else
  compose_files+=(
    -f "${root_dir}/deploy/microservices/agent-ai-sdk-shadow.yml"
    -f "${root_dir}/deploy/microservices/agent-deepseek-v4-flash-shadow.yml"
  )
fi
[[ "${DIPOLE_AGENT_SUBSCRIPTION_AUTOREPLY}" == "0" ]] || compose_files+=( -f "${root_dir}/deploy/microservices/agent-subscription-autoreply.yml" )
[[ "${DIPOLE_MYSQL_AIO_COMPAT}" == "0" ]] || compose_files+=( -f "${root_dir}/deploy/microservices/remote-gpu-mysql-aio-compat.yml" )
compose() {
  local env_args=()
  [[ -z "${model_env_file}" ]] || env_args=(--env-file "${model_env_file}")
  docker compose "${env_args[@]}" -p "${project_name}" "${compose_files[@]}" "$@"
}

cleanup() {
  local status=$?
  compose exec -T -e MYSQL_PWD=root123 mysql mysql -uroot dipole -e "UPDATE agent_runtime_promotion_grants SET revoked_at = COALESCE(revoked_at, UTC_TIMESTAMP(3)) WHERE grant_uuid = '${grant_uuid}'" >/dev/null 2>&1 || true
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

mysql() { compose exec -T -e MYSQL_PWD=root123 mysql mysql -N -B -uroot dipole "$@"; }
mysql -e "INSERT IGNORE INTO users (uuid, nickname, telephone, password_hash, status, created_at, updated_at) VALUES ('${agent_uuid}', 'Dipole Agent', '13900000002', 'smoke', 1, NOW(3), NOW(3));"

binding=$(compose exec -T agent node --input-type=module - "${owner_telephone}" "${agent_uuid}" "${DIPOLE_AGENT_SUBSCRIPTION_AUTOREPLY}" <<'NODE'
const [telephone, agentUuid, autoReply] = process.argv.slice(2);
const register = await fetch("http://core:8081/api/v1/auth/register", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ nickname: "Subscription Active", telephone, password: "smoke-pass-123" }) });
const ownerUuid = (await register.json())?.data?.user?.uuid;
if (register.status !== 200 || typeof ownerUuid !== "string") throw new Error(`register failed: ${register.status}`);
const login = await fetch("http://core:8081/api/v1/auth/login", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ telephone, password: "smoke-pass-123" }) });
const token = (await login.json())?.data?.token;
if (login.status !== 200 || typeof token !== "string") throw new Error(`login failed: ${login.status}`);
const headers = { authorization: `Bearer ${token}`, "content-type": "application/json" };
const definitionResponse = await fetch("http://gateway:8080/api/v1/agent/definitions", {
  method: "POST", headers,
  ...(autoReply === "1" ? { body: JSON.stringify({ profile: "subscription_autoreply" }) } : {})
});
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
process.stdout.write(`${ownerUuid}\t${definition.definitionId}\t${subscription.subscriptionId}\t${conversationKey}\t${token}`);
NODE
)
IFS=$'\t' read -r owner_uuid definition_uuid subscription_uuid conversation_key owner_token <<<"${binding}"

if [[ "${promotion_mode}" == "fixture" ]]; then
mysql <<SQL
INSERT INTO agent_runtime_promotion_grants (grant_uuid, tenant_id, runtime_id, candidate_version, definition_uuid, definition_version, policy_version, evidence_sha256, eval_suite_sha256, granted_by_uuid, reviewed_by_uuid, valid_from, expires_at) VALUES ('${grant_uuid}', 'dipole', 'dipole-agent', '${DIPOLE_AGENT_CANDIDATE_VERSION}', '${definition_uuid}', 1, 'dipole.agent.shadow-promotion-policy.v2', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 'U-SMOKE-GRANTOR', 'U-SMOKE-REVIEWER', DATE_SUB(UTC_TIMESTAMP(3), INTERVAL 1 MINUTE), DATE_ADD(UTC_TIMESTAMP(3), INTERVAL 15 MINUTE));
SQL
else
  # The evidence fixture is limited to the isolated project. The grant itself
  # must be created by the Gateway/Core proposal and second-review path.
  # Task and Run persistence accepts at most 64 characters, so use the stable
  # hash directly rather than prefixing it with a display namespace.
  evidence_task=$(printf '%s' "${project_name}:evidence-task" | sha256sum | awk '{print $1}')
  evidence_run=$(printf '%s' "${project_name}:evidence-run" | sha256sum | awk '{print $1}')
  evidence_artifact=$(printf '%s' "${project_name}:promotion-evidence" | sha256sum | awk '{print $1}')
  evidence_sha=$(printf '%s' "${project_name}:promotion-content" | sha256sum | awk '{print $1}')
  eval_sha="eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
  operators=$(compose exec -T agent node --input-type=module - <<'NODE'
const makeOperator = async (telephone, nickname) => {
  const register = await fetch("http://gateway:8080/api/v1/auth/register", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ nickname, telephone, password: "promotion-operator-smoke" }) });
  const owner = (await register.json())?.data?.user?.uuid;
  if (register.status !== 200 || typeof owner !== "string") throw new Error(`operator register failed: ${register.status}`);
  const login = await fetch("http://gateway:8080/api/v1/auth/login", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ telephone, password: "promotion-operator-smoke" }) });
  const token = (await login.json())?.data?.token;
  if (login.status !== 200 || typeof token !== "string") throw new Error(`operator login failed: ${login.status}`);
  return [owner, token];
};
const suffix = String(Date.now()).slice(-8);
const [proposer, reviewer] = await Promise.all([makeOperator(`188${suffix}`, "Proposer"), makeOperator(`189${suffix}`, "Reviewer")]);
process.stdout.write(`${proposer[0]}\t${proposer[1]}\t${reviewer[0]}\t${reviewer[1]}`);
NODE
)
  IFS=$'\t' read -r proposer_uuid proposer_token reviewer_uuid reviewer_token <<<"${operators}"
  mysql <<SQL
INSERT INTO agent_tasks (task_uuid, definition_uuid, definition_version, tenant_id, principal_uuid, agent_uuid, status, trigger_type, trigger_ref, goal) VALUES ('${evidence_task}', '${definition_uuid}', 1, 'dipole', '${owner_uuid}', '${agent_uuid}', 'completed', 'promotion.evaluation', 'subscription-active-smoke', 'isolated promotion evidence');
INSERT INTO agent_runs (run_uuid, task_uuid, runtime_id, candidate_version, mode, status, started_at, completed_at) VALUES ('${evidence_run}', '${evidence_task}', 'dipole-agent', NULL, 'shadow', 'completed', UTC_TIMESTAMP(3), UTC_TIMESTAMP(3));
INSERT INTO agent_runtime_promotion_operator_grants (tenant_id, user_uuid, can_propose, can_review, can_revoke, granted_by_uuid, valid_from) VALUES ('dipole', '${proposer_uuid}', TRUE, FALSE, FALSE, 'U-SMOKE-ROOT', DATE_SUB(UTC_TIMESTAMP(3), INTERVAL 1 MINUTE)), ('dipole', '${reviewer_uuid}', FALSE, TRUE, FALSE, 'U-SMOKE-ROOT', DATE_SUB(UTC_TIMESTAMP(3), INTERVAL 1 MINUTE));
SQL
  if [[ "${promotion_mode}" == "control" ]]; then
    mysql <<SQL
INSERT INTO agent_artifacts (artifact_uuid, schema_version, task_uuid, run_uuid, artifact_type, version, title, media_type, object_bucket, object_key, content_sha256, size_bytes, metadata_json) VALUES ('${evidence_artifact}', 'dipole.agent.artifact.v1', '${evidence_task}', '${evidence_run}', 'promotion_evaluation', 1, 'Subscription promotion smoke', 'application/json', 'agent', 'smoke/${evidence_artifact}', '${evidence_sha}', 2, JSON_OBJECT('runtimeId', 'dipole-agent', 'candidateVersion', '${DIPOLE_AGENT_CANDIDATE_VERSION}', 'definitionId', '${definition_uuid}', 'definitionVersion', 1, 'evalSuiteSHA256', '${eval_sha}'));
SQL
  else
    # Publish the eligible evidence through the Runtime's mTLS Artifact RPC.
    # The resulting receipt, rather than a database fixture, binds the review.
    publication_receipt=$(compose exec -T -e DIPOLE_AGENT_CAPABILITY_RPC_ENABLED=true agent node --input-type=module - "${evidence_task}" "${evidence_run}" "${definition_uuid}" "${DIPOLE_AGENT_CANDIDATE_VERSION}" <<'NODE'
import { createAgentCapabilityRPC, loadShadowRuntimeConfig } from "./dist/runtime/shadow-runtime.js";
import { PromotionEvidencePublisher } from "./dist/promotion/promotion-evidence-publisher.js";
import { evaluateOfflineEvalSuite, parseOfflineEvalSuite } from "./dist/evals/offline-evaluator.js";

const [taskId, runId, definitionId, candidateVersion] = process.argv.slice(2);
const suite = parseOfflineEvalSuite({
  schemaVersion: "dipole.agent.offline-eval-suite.v1", candidateVersion,
  cases: [
    { id: "outcome.case", category: "outcome", expected: { requiredOutputIds: ["output.ok"], forbiddenOutputIds: [] }, observed: { outputIds: ["output.ok"] } },
    { id: "trajectory.case", category: "trajectory", expected: { steps: ["step.ok"], forbiddenSteps: [] }, observed: { steps: ["step.ok"] } },
    { id: "permission.case", category: "permission", expected: { decisions: [] }, observed: { decisions: [] } },
    { id: "retrieval.case", category: "retrieval", expected: { relevantEvidenceIds: ["evidence.ok"], minimumRecall: 1, minimumPrecision: 1 }, observed: { retrievedEvidenceIds: ["evidence.ok"] } },
    { id: "cost.case", category: "cost", expected: { maximums: { modelCalls: 1, toolCalls: 1, totalTokens: 10, totalCostMicrousd: 10, latencyMs: 10 } }, observed: { modelCalls: 1, toolCalls: 1, totalTokens: 10, totalCostMicrousd: 10, latencyMs: 10 } }
  ]
});
const started = Date.now() - 24 * 60 * 60 * 1000;
const evidence = {
  schemaVersion: "dipole.agent.shadow-promotion-evidence.v2", candidateVersion,
  windowStartedAt: new Date(started).toISOString(), windowEndedAt: new Date(started + 24 * 60 * 60 * 1000).toISOString(),
  observations: Array.from({ length: 25 }, (_, index) => ({
    candidateVersion, observedAt: new Date(started + index * 60 * 60 * 1000).toISOString(),
    report: { schemaVersion: "dipole.agent.projection-reconcile.v1", consistent: true, scanned: 5, outcomes: { match: 5, missing: 0, stale: 0, ahead: 0, conflict: 0, unavailable: 0 }, examples: [] }
  })),
  projectionEvals: { passed: 6, total: 6 }, offlineEvalReport: evaluateOfflineEvalSuite(suite)
};
const rpc = createAgentCapabilityRPC(loadShadowRuntimeConfig(process.env));
try {
  const publisher = new PromotionEvidencePublisher(rpc.client);
  const publication = {
    schemaVersion: "dipole.agent.promotion-evidence-publication.v1", tenantId: "dipole", taskId, runId,
    runtimeId: "dipole-agent", definitionId, definitionVersion: 1, evidence
  };
  const receipt = await publisher.publish(publication);
  const replay = await publisher.publish(publication);
  if (replay.artifactId !== receipt.artifactId || replay.evidenceSHA256 !== receipt.evidenceSHA256 ||
      replay.evalSuiteSHA256 !== receipt.evalSuiteSHA256) {
    throw new Error("promotion evidence replay returned a conflicting receipt");
  }
  process.stdout.write(`${receipt.artifactId}\t${receipt.evidenceSHA256}\t${receipt.evalSuiteSHA256}`);
} finally {
  rpc.close();
}
NODE
)
    IFS=$'\t' read -r evidence_artifact evidence_sha eval_sha <<<"${publication_receipt}"
    [[ "${evidence_artifact}" =~ ^[a-f0-9]{64}$ && "${evidence_sha}" =~ ^[a-f0-9]{64}$ && "${eval_sha}" =~ ^[a-f0-9]{64}$ ]] || { printf 'promotion publication returned an invalid receipt: %q\n' "${publication_receipt}" >&2; exit 1; }
  fi
  grant_uuid=$(compose exec -T agent node --input-type=module - "${proposer_token}" "${reviewer_token}" "${definition_uuid}" "${evidence_artifact}" "${evidence_sha}" "${eval_sha}" "${DIPOLE_AGENT_CANDIDATE_VERSION}" <<'NODE'
const [proposerToken, reviewerToken, definitionId, artifactId, evidenceSha256, evalSuiteSha256, candidateVersion] = process.argv.slice(2);
const headers = token => ({ authorization: `Bearer ${token}`, "content-type": "application/json" });
const now = Date.now();
// Gateway owns proposedAt. Keep the grant shortly in the future so validFrom
// cannot precede the server timestamp, then wait before emitting the event.
const grantValidFromUnixMs = now + 2000;
const proposalResponse = await fetch("http://gateway:8080/api/v1/agent/runtime-promotions", { method: "POST", headers: headers(proposerToken), body: JSON.stringify({ runtimeId: "dipole-agent", candidateVersion, definitionId, definitionVersion: 1, evidenceArtifactId: artifactId, evidenceSha256, evalSuiteSha256, ticketRef: "SMOKE-1", reason: "isolated subscription admission", expiresAtUnixMs: now + 300000, grantValidFromUnixMs, grantExpiresAtUnixMs: now + 600000 }) });
const proposal = await proposalResponse.json();
if (proposalResponse.status !== 200 || proposal?.status !== "proposed" || typeof proposal?.proposalId !== "string") throw new Error(`promotion propose failed: ${proposalResponse.status} ${JSON.stringify(proposal)}`);
const reviewResponse = await fetch(`http://gateway:8080/api/v1/agent/runtime-promotions/${proposal.proposalId}/review`, { method: "POST", headers: headers(reviewerToken), body: JSON.stringify({ decision: "approved" }) });
const reviewed = await reviewResponse.json();
if (reviewResponse.status !== 200 || reviewed?.status !== "approved" || typeof reviewed?.grantId !== "string") throw new Error(`promotion review failed: ${reviewResponse.status} ${JSON.stringify(reviewed)}`);
await new Promise(resolve => setTimeout(resolve, Math.max(0, grantValidFromUnixMs - Date.now() + 100)));
process.stdout.write(reviewed.grantId);
NODE
)
  [[ "${grant_uuid}" =~ ^[a-f0-9]{64}$ ]] || { printf 'promotion control did not return a grant ID: %q\n' "${grant_uuid}" >&2; exit 1; }
fi

compose exec -T agent node --input-type=module - "${owner_token}" "${agent_uuid}" <<'NODE'
const [token, agentUuid] = process.argv.slice(2);
const socket = new WebSocket(`ws://gateway:8080/api/v1/ws?token=${encodeURIComponent(token)}&device=smoke-subscription`);
await new Promise((resolve, reject) => {
  const timer = setTimeout(() => reject(new Error("subscription message timeout")), 15000);
  socket.addEventListener("open", () => socket.send(JSON.stringify({ type: "chat.send", data: { target_uuid: agentUuid, content: "subscription active smoke", client_message_id: `subscription-active-${Date.now()}` } })));
  socket.addEventListener("message", ({ data }) => {
    const event = JSON.parse(String(data));
    if (event?.type === "chat.sent") { clearTimeout(timer); socket.close(); resolve(); }
    if (event?.type === "error") reject(new Error(`subscription message failed: ${JSON.stringify(event.data)}`));
  });
  socket.addEventListener("error", () => reject(new Error("subscription message socket failed")));
});
NODE

task_state=""
for _ in $(seq 1 90); do
  task_state=$(mysql -e "SELECT CONCAT(status, ':', workflow_status) FROM agent_tasks WHERE trigger_subscription_uuid = '${subscription_uuid}'" || true)
  [[ "${task_state}" == "completed:completed" ]] && break
  sleep 1
done
[[ "${task_state}" == "completed:completed" ]] || { printf 'subscription task did not complete: %q\n' "${task_state}" >&2; exit 1; }
task_count=$(mysql -e "SELECT COUNT(*) FROM agent_tasks WHERE trigger_subscription_uuid = '${subscription_uuid}'")
[[ "${task_count}" == "1" ]] || { printf 'expected one subscription task, got %s\n' "${task_count}" >&2; exit 1; }
model_calls=$(mysql -e "SELECT COUNT(*) FROM agent_model_runs AS r JOIN agent_tasks AS t ON t.task_uuid = r.task_uuid WHERE t.trigger_subscription_uuid = '${subscription_uuid}' AND r.status = 'completed'")
[[ "${model_calls}" =~ ^[0-9]+$ && "${model_calls}" -ge 1 ]] || { printf 'subscription task completed without a model call: %s\n' "${model_calls}" >&2; exit 1; }
messages=$(mysql -e "SELECT COUNT(*) FROM messages WHERE sender_uuid = '${agent_uuid}' AND target_uuid = '${owner_uuid}'")
if [[ "${DIPOLE_AGENT_SUBSCRIPTION_AUTOREPLY}" == "1" ]]; then
  auto_reply_effects=$(mysql -e "SELECT
  (SELECT COUNT(*) FROM agent_tool_invocations WHERE task_uuid = (SELECT task_uuid FROM agent_tasks WHERE trigger_subscription_uuid = '${subscription_uuid}') AND status = 'completed' AND capability_id = 'message.system.send'),
  (SELECT COUNT(*) FROM agent_approvals WHERE task_uuid = (SELECT task_uuid FROM agent_tasks WHERE trigger_subscription_uuid = '${subscription_uuid}') AND status = 'consumed' AND capability_id = 'message.system.send'),
  (SELECT COUNT(*) FROM messages WHERE sender_uuid = '${agent_uuid}' AND target_uuid = '${owner_uuid}' AND content = '${model_summary}'),
  (SELECT COUNT(DISTINCT client_message_id) FROM messages WHERE sender_uuid = '${agent_uuid}' AND target_uuid = '${owner_uuid}' AND content = '${model_summary}'),
  (SELECT COUNT(*) FROM user_sync_inbox AS inbox JOIN messages AS message ON message.uuid = inbox.message_uuid WHERE message.sender_uuid = '${agent_uuid}' AND message.target_uuid = '${owner_uuid}' AND message.content = '${model_summary}')")
  [[ "${auto_reply_effects}" == $'1\t1\t1\t1\t2' ]] || { printf 'subscription auto-reply side effects drifted: %q\n' "${auto_reply_effects}" >&2; exit 1; }
else
  [[ "${messages}" == "0" ]] || { printf 'subscription read task wrote %s messages\n' "${messages}" >&2; exit 1; }
fi
mysql -e "UPDATE agent_runtime_promotion_grants SET revoked_at = UTC_TIMESTAMP(3) WHERE grant_uuid = '${grant_uuid}' AND revoked_at IS NULL"
if [[ "${DIPOLE_AGENT_SUBSCRIPTION_AUTOREPLY}" == "1" ]]; then
  printf 'Agent Subscription auto-reply Compose smoke passed: one owner-scoped Kafka event completed one durable Task with one or more model calls, one consumed approval, and one owner-Agent reply.\n'
else
  printf 'Agent Subscription active Compose smoke passed: one owner-scoped Kafka event completed one durable read Task with one or more model calls and zero messages.\n'
fi
