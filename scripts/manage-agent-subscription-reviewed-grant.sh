#!/usr/bin/env bash
set -euo pipefail

# Creates a short-lived, owner-bound subscription grant through the public
# Gateway/Core review path. It deliberately requires explicit opt-in because
# it opens the narrowly scoped promotion Gateway overlay in a shared project.

die() { printf '%s\n' "$*" >&2; exit 2; }
log() { printf '%s\n' "$*" >&2; }
require_id() { [[ "$2" =~ ^[A-Za-z0-9._:-]+$ && ${#2} -le "$3" ]] || die "invalid $1"; }
require_sha() { [[ "$2" =~ ^[a-f0-9]{64}$ ]] || die "invalid $1"; }

action="${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_ACTION:-grant}"
[[ "$action" == grant || "$action" == revoke ]] || die "reviewed grant action must be grant or revoke"
[[ "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_ALLOW_REVIEWED_GRANT_HELPER:-0}" == 1 ]] || die "set DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_ALLOW_REVIEWED_GRANT_HELPER=1 to use this helper"
: "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_CONFIG:?set DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_CONFIG}"
: "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_STATE_FILE:?set DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_STATE_FILE}"
config="${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_CONFIG}"
state_file="${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_STATE_FILE}"
[[ "$config" == /* && -r "$config" ]] || die "reviewed grant config must be an absolute readable path"
[[ "$state_file" == /* ]] || die "reviewed grant state file must be an absolute path"

mapfile -t config_values < <(python3 - "$config" <<'PY'
import json, os, re, sys
try:
    with open(sys.argv[1], encoding="utf-8") as handle: config = json.load(handle)
except (OSError, json.JSONDecodeError) as exc: raise SystemExit(f"invalid reviewed grant config: {exc}")
required = {"compose_project", "env_file", "compose_files", "promotion_overlay", "certificate_dir", "gateway_service", "tenant_id"}
if set(config) != required: raise SystemExit("reviewed grant config must match promotion window config")
if not isinstance(config["compose_files"], list) or not config["compose_files"]: raise SystemExit("compose_files must be non-empty")
for key in ("env_file", "promotion_overlay", "certificate_dir"):
    if not isinstance(config[key], str) or not config[key].startswith("/") or not os.path.exists(config[key]): raise SystemExit(f"invalid {key}")
for path in config["compose_files"]:
    if not isinstance(path, str) or not path.startswith("/") or not os.path.isfile(path): raise SystemExit("invalid compose file")
for key in ("compose_project", "gateway_service", "tenant_id"):
    if not isinstance(config[key], str) or not re.fullmatch(r"[A-Za-z0-9._:-]{1,96}", config[key]): raise SystemExit(f"invalid {key}")
for value in [config["compose_project"], config["env_file"], config["promotion_overlay"], config["certificate_dir"], config["gateway_service"], config["tenant_id"], *config["compose_files"]]:
    if any(ch in value for ch in "\n\r\t"): raise SystemExit("control character in config")
for value in (config["compose_project"], config["env_file"], config["promotion_overlay"], config["certificate_dir"], config["gateway_service"], config["tenant_id"], *config["compose_files"]): print(value)
PY
)
project="${config_values[0]}"; env_file="${config_values[1]}"; gateway_service="${config_values[4]}"; tenant_id="${config_values[5]}"
compose_files=("${config_values[@]:6}")
require_id project "$project" 96; require_id tenant "$tenant_id" 96
internal_gateway="http://${gateway_service}:8080"

compose_args=(--env-file "$env_file" -p "$project")
for file in "${compose_files[@]}"; do compose_args+=(-f "$file"); done
mysql() { docker exec "${project}-mysql-1" sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot -N -B dipole -e "'"$1"'"'; }
operator() {
  local subaction=$1 user=$2 actor=$3 roles=${4:-} expiry=${5:-}
  local args=("$(dirname "${BASH_SOURCE[0]}")/manage-agent-promotion-operator-grant.sh" "$subaction" --compose-project "$project" --env-file "$env_file" --user "$user" --granted-by "$actor" --ticket "SUB-E2E-${state_key}" --reason "subscription active reviewed grant")
  for file in "${compose_files[@]}"; do args+=(--compose-file "$file"); done
  [[ "$subaction" != grant ]] || args+=(--roles "$roles" --expires-at "$expiry")
  DIPOLE_AGENT_PROMOTION_MYSQL_ROOT_PASSWORD="${DIPOLE_AGENT_PROMOTION_MYSQL_ROOT_PASSWORD:?set DIPOLE_AGENT_PROMOTION_MYSQL_ROOT_PASSWORD}" "${args[@]}" --apply >/dev/null
}

state_key=$(printf '%s' "$state_file" | sha256sum | awk '{print substr($1,1,12)}')
if [[ "$action" == revoke ]]; then
  : "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_UUID:?set DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_UUID}"
  grant_uuid="$DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_UUID"; require_sha grant "$grant_uuid"
  [[ -r "$state_file" ]] || die "reviewed grant state file is missing"
  mapfile -t state < <(python3 - "$state_file" "$grant_uuid" <<'PY'
import json, os, stat, sys
path, grant = sys.argv[1:]
info = os.stat(path)
if stat.S_IMODE(info.st_mode) & 0o077: raise SystemExit("state file must be owner-only")
with open(path, encoding="utf-8") as handle: state = json.load(handle)
required = {"grant_uuid", "owner_uuid", "proposer_uuid", "reviewer_uuid", "config"}
if set(state) != required or state["grant_uuid"] != grant: raise SystemExit("state does not match reviewed grant")
if state["config"] != os.path.abspath(os.environ["DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_CONFIG"]): raise SystemExit("state config mismatch")
for key in ("owner_uuid", "proposer_uuid", "reviewer_uuid"):
    print(state[key])
PY
)
  owner_uuid="${state[0]}"; proposer_uuid="${state[1]}"; reviewer_uuid="${state[2]}"
  require_id owner "$owner_uuid" 24; require_id proposer "$proposer_uuid" 24; require_id reviewer "$reviewer_uuid" 24
  mysql "UPDATE agent_runtime_promotion_grants SET revoked_at=COALESCE(revoked_at, UTC_TIMESTAMP(3)) WHERE grant_uuid='${grant_uuid}' AND tenant_id='${tenant_id}'" >/dev/null
  operator revoke "$proposer_uuid" "$owner_uuid"
  operator revoke "$reviewer_uuid" "$owner_uuid"
  "$(dirname "${BASH_SOURCE[0]}")/run-agent-promotion-window.sh" close --config "$config" --apply >/dev/null
  rm -f "$state_file"
  log "reviewed subscription grant revoked and promotion window closed"
  exit 0
fi

: "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_OWNER_UUID:?missing owner UUID}"
: "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_DEFINITION_UUID:?missing Definition UUID}"
: "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_DEFINITION_VERSION:?missing Definition version}"
: "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_SUBSCRIPTION_UUID:?missing Subscription UUID}"
: "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_CONVERSATION_KEY:?missing conversation key}"
: "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_CANDIDATE_VERSION:?missing candidate version}"
: "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_GATEWAY:?missing Gateway URL}"
owner_uuid="$DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_OWNER_UUID"; definition_uuid="$DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_DEFINITION_UUID"; definition_version="$DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_DEFINITION_VERSION"; subscription_uuid="$DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_SUBSCRIPTION_UUID"; candidate="$DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_CANDIDATE_VERSION"; gateway="$DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_GATEWAY"
require_id owner "$owner_uuid" 24; require_id definition "$definition_uuid" 64; [[ "$definition_version" =~ ^[1-9][0-9]*$ ]] || die "invalid Definition version"; require_id subscription "$subscription_uuid" 64; require_id candidate "$candidate" 128; [[ "$gateway" =~ ^https?://[A-Za-z0-9.:_-]+$ ]] || die "invalid Gateway URL"
[[ "$project" == "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_PROJECT:-}" ]] || die "project differs from reviewed grant config"
[[ ! -e "$state_file" ]] || die "reviewed grant state file already exists"

"$(dirname "${BASH_SOURCE[0]}")/run-agent-promotion-window.sh" open --config "$config" --apply >/dev/null
cleanup_open_window=1
trap 'if (( cleanup_open_window )); then "$(dirname "${BASH_SOURCE[0]}")/run-agent-promotion-window.sh" close --config "$config" --apply >/dev/null 2>&1 || true; fi' EXIT

# Registration requires an eleven-digit telephone. Keep the unique suffix
# numeric so the temporary operator accounts pass the public validation.
suffix=$(printf '%08d' $((10#$(date +%N) % 100000000)))
operator_registration=$(docker exec -e GATEWAY="$internal_gateway" -e SUFFIX="$suffix" "${project}-agent-1" node --input-type=module - <<'NODE'
const register = async (prefix, nickname) => {
  const telephone = `${prefix}${process.env.SUFFIX}`.slice(0, 11);
  const password = "reviewed-grant-temporary";
  const response = await fetch(`${process.env.GATEWAY}/api/v1/auth/register`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ nickname, telephone, password }) });
  const body = await response.json();
  if (response.status !== 200 || typeof body?.data?.user?.uuid !== "string" || typeof body?.data?.token !== "string") throw new Error(`operator registration failed: ${response.status}`);
  return [body.data.user.uuid, body.data.token];
};
const [proposer, reviewer] = await Promise.all([register("187", "Promotion Proposer"), register("185", "Promotion Reviewer")]);
process.stdout.write(`${proposer[0]}\t${proposer[1]}\t${reviewer[0]}\t${reviewer[1]}`);
NODE
)
IFS=$'\t' read -r proposer_uuid proposer_token reviewer_uuid reviewer_token <<<"$operator_registration"
require_id proposer "$proposer_uuid" 24; require_id reviewer "$reviewer_uuid" 24
expiry=$(date -u -d '+10 minutes' '+%Y-%m-%dT%H:%M:%SZ')
operator grant "$proposer_uuid" "$owner_uuid" propose "$expiry"
operator grant "$reviewer_uuid" "$owner_uuid" review,revoke "$expiry"

evidence_task=$(printf '%s' "${definition_uuid}:${subscription_uuid}:evidence-task" | sha256sum | awk '{print $1}')
evidence_run=$(printf '%s' "${definition_uuid}:${subscription_uuid}:evidence-run" | sha256sum | awk '{print $1}')
mysql "INSERT INTO agent_tasks (task_uuid, definition_uuid, definition_version, tenant_id, principal_uuid, agent_uuid, status, trigger_type, trigger_ref, goal) VALUES ('${evidence_task}', '${definition_uuid}', ${definition_version}, '${tenant_id}', '${owner_uuid}', 'UAI000000000000000001', 'completed', 'promotion.evaluation', 'subscription-reviewed-grant', 'reviewed subscription evidence'); INSERT INTO agent_runs (run_uuid, task_uuid, runtime_id, candidate_version, mode, status, started_at, completed_at) VALUES ('${evidence_run}', '${evidence_task}', 'dipole-agent', NULL, 'shadow', 'completed', UTC_TIMESTAMP(3), UTC_TIMESTAMP(3));" >/dev/null
receipt=$(docker exec -e DIPOLE_AGENT_CAPABILITY_RPC_ENABLED=true "${project}-agent-1" node --input-type=module - "$evidence_task" "$evidence_run" "$definition_uuid" "$definition_version" "$candidate" <<'NODE'
import { createAgentCapabilityRPC, loadShadowRuntimeConfig } from "./dist/runtime/shadow-runtime.js";
import { PromotionEvidencePublisher } from "./dist/promotion/promotion-evidence-publisher.js";
import { evaluateOfflineEvalSuite, parseOfflineEvalSuite } from "./dist/evals/offline-evaluator.js";
const [taskId, runId, definitionId, definitionVersion, candidateVersion] = process.argv.slice(2);
const suite = parseOfflineEvalSuite({ schemaVersion: "dipole.agent.offline-eval-suite.v1", candidateVersion, cases: [{ id: "outcome", category: "outcome", expected: { requiredOutputIds: ["ok"], forbiddenOutputIds: [] }, observed: { outputIds: ["ok"] } }, { id: "trajectory", category: "trajectory", expected: { steps: ["ok"], forbiddenSteps: [] }, observed: { steps: ["ok"] } }, { id: "permission", category: "permission", expected: { decisions: [] }, observed: { decisions: [] } }, { id: "retrieval", category: "retrieval", expected: { relevantEvidenceIds: ["ok"], minimumRecall: 1, minimumPrecision: 1 }, observed: { retrievedEvidenceIds: ["ok"] } }, { id: "cost", category: "cost", expected: { maximums: { modelCalls: 1, toolCalls: 1, totalTokens: 10, totalCostMicrousd: 10, latencyMs: 10 } }, observed: { modelCalls: 1, toolCalls: 1, totalTokens: 10, totalCostMicrousd: 10, latencyMs: 10 } }] });
const started = Date.now() - 24 * 60 * 60 * 1000;
const evidence = { schemaVersion: "dipole.agent.shadow-promotion-evidence.v2", candidateVersion, windowStartedAt: new Date(started).toISOString(), windowEndedAt: new Date(started + 24 * 60 * 60 * 1000).toISOString(), observations: Array.from({ length: 25 }, (_, index) => ({ candidateVersion, observedAt: new Date(started + index * 3600000).toISOString(), report: { schemaVersion: "dipole.agent.projection-reconcile.v1", consistent: true, scanned: 5, outcomes: { match: 5, missing: 0, stale: 0, ahead: 0, conflict: 0, unavailable: 0 }, examples: [] } })), projectionEvals: { passed: 6, total: 6 }, offlineEvalReport: evaluateOfflineEvalSuite(suite) };
const rpc = createAgentCapabilityRPC(loadShadowRuntimeConfig(process.env));
try { const receipt = await new PromotionEvidencePublisher(rpc.client).publish({ schemaVersion: "dipole.agent.promotion-evidence-publication.v1", tenantId: "dipole", taskId, runId, runtimeId: "dipole-agent", definitionId, definitionVersion: Number(definitionVersion), evidence }); process.stdout.write(`${receipt.artifactId}\t${receipt.evidenceSHA256}\t${receipt.evalSuiteSHA256}`); } finally { rpc.close(); }
NODE
)
IFS=$'\t' read -r artifact_uuid evidence_sha eval_sha <<<"$receipt"; require_sha artifact "$artifact_uuid"; require_sha evidence "$evidence_sha"; require_sha eval "$eval_sha"
grant_uuid=$(docker exec -e GATEWAY="$internal_gateway" -e PROPOSER_TOKEN="$proposer_token" -e REVIEWER_TOKEN="$reviewer_token" "${project}-agent-1" node --input-type=module - "$definition_uuid" "$definition_version" "$candidate" "$artifact_uuid" "$evidence_sha" "$eval_sha" <<'NODE'
const [definitionId, definitionVersion, candidateVersion, artifactId, evidenceSha256, evalSuiteSha256] = process.argv.slice(2); const headers = token => ({ authorization: `Bearer ${token}`, "content-type": "application/json" }); const now = Date.now(); const validFrom = now + 2000;
const proposal = await fetch(`${process.env.GATEWAY}/api/v1/agent/runtime-promotions`, { method: "POST", headers: headers(process.env.PROPOSER_TOKEN), body: JSON.stringify({ runtimeId: "dipole-agent", candidateVersion, definitionId, definitionVersion: Number(definitionVersion), evidenceArtifactId: artifactId, evidenceSha256, evalSuiteSha256, ticketRef: `SUB-E2E-${Date.now()}`, reason: "reviewed subscription active read", expiresAtUnixMs: now + 300000, grantValidFromUnixMs: validFrom, grantExpiresAtUnixMs: now + 600000 }) }).then(async response => ({ status: response.status, body: await response.json() }));
if (proposal.status !== 200 || proposal.body?.status !== "proposed") throw new Error(`proposal failed: ${proposal.status}`); const reviewed = await fetch(`${process.env.GATEWAY}/api/v1/agent/runtime-promotions/${proposal.body.proposalId}/review`, { method: "POST", headers: headers(process.env.REVIEWER_TOKEN), body: JSON.stringify({ decision: "approved" }) }).then(async response => ({ status: response.status, body: await response.json() })); if (reviewed.status !== 200 || reviewed.body?.status !== "approved" || typeof reviewed.body?.grantId !== "string") throw new Error(`review failed: ${reviewed.status}`); await new Promise(resolve => setTimeout(resolve, Math.max(0, validFrom - Date.now() + 100))); process.stdout.write(reviewed.body.grantId);
NODE
)
require_sha grant "$grant_uuid"
umask 077
tmp_state=$(mktemp "${state_file}.tmp.XXXXXX")
python3 - "$tmp_state" "$grant_uuid" "$owner_uuid" "$proposer_uuid" "$reviewer_uuid" "$config" <<'PY'
import json, os, sys
path, grant, owner, proposer, reviewer, config = sys.argv[1:]
with open(path, "w", encoding="utf-8") as handle: json.dump({"grant_uuid": grant, "owner_uuid": owner, "proposer_uuid": proposer, "reviewer_uuid": reviewer, "config": os.path.abspath(config)}, handle, separators=(",", ":")); handle.write("\n")
PY
mv "$tmp_state" "$state_file"
chmod 600 "$state_file"
cleanup_open_window=0
printf '%s\n' "$grant_uuid"
