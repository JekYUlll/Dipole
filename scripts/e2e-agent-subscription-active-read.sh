#!/usr/bin/env bash
# Verifies the public read-only subscription worker without disturbing Route B
# interactive delivery. Shared runs consume a reviewed grant created through
# the Gateway/Core control plane; fixture grants remain isolated-only opt-in.
set -euo pipefail

PROJECT="${PROJECT:-dipole-experience}"
AGENT_UUID="${AGENT_UUID:-UAI000000000000000001}"
GATEWAY="${GATEWAY:-http://127.0.0.1:8080}"
CANDIDATE_VERSION="${DIPOLE_AGENT_CANDIDATE_VERSION:-experience-v1}"
GRANT_MODE="${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_GRANT_MODE:-reviewed}"
# A reviewed grant must bind the Definition created by this run. The helper is
# responsible for the separately approved proposal/review flow and returns
# only its resulting grant UUID on stdout.
REVIEWED_GRANT_COMMAND="${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_COMMAND:-}"
REVIEWED_GRANT_CLEANUP_COMMAND="${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_CLEANUP_COMMAND:-}"
TELEPHONE="186$(printf '%08d' $((10#$(date +%N) % 100000000)))"
PASSWORD="subscription-read-e2e-pass-123"
grant_uuid=""
fixture_grant=0

[[ "${GRANT_MODE}" == "fixture" || "${GRANT_MODE}" == "reviewed" ]] || { printf 'DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_GRANT_MODE must be fixture or reviewed\n' >&2; exit 2; }
if [[ "${GRANT_MODE}" == "fixture" ]]; then
  [[ "${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_ALLOW_FIXTURE_GRANT:-0}" == "1" ]] || { printf 'fixture grant mode requires DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_ALLOW_FIXTURE_GRANT=1\n' >&2; exit 2; }
  fixture_grant=1
else
  [[ "${REVIEWED_GRANT_COMMAND}" == /* && -x "${REVIEWED_GRANT_COMMAND}" ]] || {
    printf 'reviewed grant mode requires an executable absolute DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_COMMAND\n' >&2
    exit 2
  }
  [[ -z "${REVIEWED_GRANT_CLEANUP_COMMAND}" || ( "${REVIEWED_GRANT_CLEANUP_COMMAND}" == /* && -x "${REVIEWED_GRANT_CLEANUP_COMMAND}" ) ]] || {
    printf 'reviewed grant cleanup command must be an executable absolute path when provided\n' >&2
    exit 2
  }
fi

# Raw batch output preserves the tab-delimited grant binding used below.
mysql() { docker exec "${PROJECT}-mysql-1" sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot -N -B -r dipole -e "'"$1"'"'; }

cleanup() {
  if (( fixture_grant )) && [[ -n "${grant_uuid}" ]]; then
    mysql "UPDATE agent_runtime_promotion_grants SET revoked_at=COALESCE(revoked_at, UTC_TIMESTAMP(3)) WHERE grant_uuid='${grant_uuid}'" >/dev/null
  fi
  if (( ! fixture_grant )) && [[ -n "${grant_uuid}" && -n "${REVIEWED_GRANT_CLEANUP_COMMAND}" ]]; then
    DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_ACTION=revoke \
      DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_UUID="${grant_uuid}" \
      DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_OWNER_UUID="${owner_uuid:-}" \
      DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_PROJECT="${PROJECT}" \
      "${REVIEWED_GRANT_CLEANUP_COMMAND}" || printf 'reviewed grant cleanup failed for %s\n' "${grant_uuid}" >&2
  fi
}
trap cleanup EXIT

require_identifier() {
  [[ "$2" =~ ^[A-Za-z0-9._:-]+$ && ${#2} -le "$3" ]] || { printf 'invalid %s\n' "$1" >&2; exit 1; }
}

echo "==> register a fresh subscription owner (${TELEPHONE})"
registration=$(curl -sS -X POST "${GATEWAY}/api/v1/auth/register" -H 'content-type: application/json' \
  -d "{\"nickname\":\"Sub E2E\",\"telephone\":\"${TELEPHONE}\",\"password\":\"${PASSWORD}\"}")
owner_uuid=$(printf '%s' "${registration}" | python3 -c 'import json,sys; body=json.load(sys.stdin); print(body.get("data", {}).get("user", {}).get("uuid", ""))')
token=$(printf '%s' "${registration}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("data", {}).get("token", ""))')
[[ -n "${owner_uuid}" && -n "${token}" ]] || { printf 'registration failed: %s\n' "${registration}" >&2; exit 1; }
require_identifier owner "${owner_uuid}" 24

echo "==> create a group with the assistant as a member"
group_response=$(curl -fsS -X POST "${GATEWAY}/api/v1/groups" -H 'content-type: application/json' -H "authorization: Bearer ${token}" \
  -d "{\"name\":\"Subscription Read E2E\",\"member_uuids\":[\"${AGENT_UUID}\"]}")
group_uuid=$(printf '%s' "${group_response}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["uuid"])')
require_identifier group "${group_uuid}" 64

send_group_message() {
  docker exec -i -e TOKEN="${token}" -e GROUP="${group_uuid}" -e LABEL="$1" "${PROJECT}-agent-1" node --input-type=module - <<'NODE'
const [token, group, label] = [process.env.TOKEN, process.env.GROUP, process.env.LABEL];
const socket = new WebSocket(`ws://gateway:8080/api/v1/ws?token=${encodeURIComponent(token)}&device=subscription-read-e2e`);
await new Promise((resolve, reject) => {
  const timer = setTimeout(() => reject(new Error("send timeout")), 15_000);
  socket.addEventListener("open", () => socket.send(JSON.stringify({ type: "chat.send", data: {
    target_uuid: group, content: `subscription read ${label}`, client_message_id: `subscription-read-${label}-${Date.now()}`
  }})));
  socket.addEventListener("message", ({ data }) => {
    const event = JSON.parse(String(data));
    if (event?.type === "chat.sent") { clearTimeout(timer); socket.close(); resolve(); }
    if (event?.type === "error") reject(new Error(`send failed: ${JSON.stringify(event.data)}`));
  });
  socket.addEventListener("error", () => reject(new Error("socket failed")));
});
NODE
}

echo "==> establish owner access to the group conversation"
send_group_message bootstrap

echo "==> create an owner-scoped read-only Definition and Subscription"
definition_response=$(curl -fsS -X POST "${GATEWAY}/api/v1/agent/definitions" -H 'content-type: application/json' -H "authorization: Bearer ${token}" \
  -d '{"profile":"read_only"}')
definition_uuid=$(printf '%s' "${definition_response}" | python3 -c 'import json,sys; body=json.load(sys.stdin); print(body["definitionId"])')
definition_version=$(printf '%s' "${definition_response}" | python3 -c 'import json,sys; body=json.load(sys.stdin); print(body["version"])')
require_identifier definition "${definition_uuid}" 64
[[ "${definition_version}" == "1" ]] || { printf 'unexpected Definition version: %s\n' "${definition_version}" >&2; exit 1; }

conversation_key="group:${group_uuid}"
subscription_response=$(curl -fsS -X POST "${GATEWAY}/api/v1/agent/subscriptions" -H 'content-type: application/json' -H "authorization: Bearer ${token}" \
  -d "{\"definitionId\":\"${definition_uuid}\",\"definitionVersion\":${definition_version},\"conversationKey\":\"${conversation_key}\",\"filterKind\":\"all\",\"filter\":{}}")
subscription_uuid=$(printf '%s' "${subscription_response}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["subscriptionId"])')
require_identifier subscription "${subscription_uuid}" 64

if (( fixture_grant )); then
  echo "==> provision an explicitly approved fixture grant for the read-only Definition"
  grant_uuid=$(openssl rand -hex 32)
  mysql "INSERT INTO agent_runtime_promotion_grants (grant_uuid, tenant_id, runtime_id, candidate_version, definition_uuid, definition_version, policy_version, evidence_sha256, eval_suite_sha256, granted_by_uuid, reviewed_by_uuid, valid_from, expires_at) VALUES ('${grant_uuid}', 'dipole', 'dipole-agent', '${CANDIDATE_VERSION}', '${definition_uuid}', ${definition_version}, 'dipole.agent.shadow-promotion-policy.v2', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 'U-E2E-GRANTOR', 'U-E2E-REVIEWER', DATE_SUB(UTC_TIMESTAMP(3), INTERVAL 1 MINUTE), DATE_ADD(UTC_TIMESTAMP(3), INTERVAL 10 MINUTE))"
else
  echo "==> obtain a reviewed grant bound to this Definition"
  grant_uuid=$(DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_OWNER_UUID="${owner_uuid}" \
    DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_DEFINITION_UUID="${definition_uuid}" \
    DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_DEFINITION_VERSION="${definition_version}" \
    DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_SUBSCRIPTION_UUID="${subscription_uuid}" \
    DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_CONVERSATION_KEY="${conversation_key}" \
    DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_CANDIDATE_VERSION="${CANDIDATE_VERSION}" \
    DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_GATEWAY="${GATEWAY}" \
    DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_PROJECT="${PROJECT}" \
    "${REVIEWED_GRANT_COMMAND}")
  [[ "${grant_uuid}" =~ ^[a-f0-9]{64}$ ]] || { printf 'reviewed grant helper must output one 64-character grant UUID\n' >&2; exit 1; }

  echo "==> verify the reviewed grant is active and bound to this Definition"
  grant_binding=$(mysql "SELECT CONCAT(runtime_id, CHAR(9), candidate_version, CHAR(9), definition_uuid, CHAR(9), definition_version) FROM agent_runtime_promotion_grants WHERE grant_uuid='${grant_uuid}' AND tenant_id='dipole' AND revoked_at IS NULL AND valid_from <= UTC_TIMESTAMP(3) AND expires_at > UTC_TIMESTAMP(3)")
  expected_binding=$(printf 'dipole-agent\t%s\t%s\t%s' "${CANDIDATE_VERSION}" "${definition_uuid}" "${definition_version}")
  [[ "${grant_binding}" == "${expected_binding}" ]] || { printf 'reviewed grant binding mismatch: %q\n' "${grant_binding}" >&2; exit 1; }
fi

echo "==> send a non-mention group message to trigger only the subscription worker"
send_group_message trigger

echo "==> wait for one completed subscription task"
task_uuid=""
state=""
for _ in $(seq 1 120); do
  task_uuid=$(mysql "SELECT task_uuid FROM agent_tasks WHERE trigger_subscription_uuid='${subscription_uuid}' ORDER BY created_at DESC LIMIT 1")
  if [[ -n "${task_uuid}" ]]; then
    state=$(mysql "SELECT CONCAT(status, ':', COALESCE(workflow_status, '')) FROM agent_tasks WHERE task_uuid='${task_uuid}'")
    [[ "${state}" == "completed:completed" ]] && break
  fi
  sleep 1
done
[[ "${state}" == "completed:completed" ]] || { printf 'subscription task did not complete: %s\n' "${state}" >&2; exit 1; }

echo "==> assert the task used the owner Definition, called the model, and wrote no group reply"
pinned=$(mysql "SELECT definition_uuid FROM agent_tasks WHERE task_uuid='${task_uuid}'")
model_calls=$(mysql "SELECT COUNT(*) FROM agent_model_runs WHERE task_uuid='${task_uuid}' AND status='completed'")
agent_messages=$(mysql "SELECT COUNT(*) FROM messages WHERE sender_uuid='${AGENT_UUID}' AND target_uuid='${group_uuid}' AND target_type=1")
[[ "${pinned}" == "${definition_uuid}" ]] || { printf 'Definition binding drifted: %s\n' "${pinned}" >&2; exit 1; }
[[ "${model_calls}" =~ ^[0-9]+$ && "${model_calls}" -ge 1 ]] || { printf 'subscription task has no completed model call: %s\n' "${model_calls}" >&2; exit 1; }
[[ "${agent_messages}" == "0" ]] || { printf 'read-only subscription wrote %s group messages\n' "${agent_messages}" >&2; exit 1; }

echo "==> PASS: subscription active read completed with one owner-scoped task and zero Agent messages"
echo "    owner=${owner_uuid} group=${group_uuid} subscription=${subscription_uuid} task=${task_uuid} model_calls=${model_calls}"
