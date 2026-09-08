#!/usr/bin/env bash
# Route B/B1 regression against the experience stack. An owner-scoped
# read-only Definition without a reviewed promotion grant must not block an
# inbound assistant DM; it falls back to the shared low-risk Definition.
set -euo pipefail

PROJECT="${PROJECT:-dipole-experience}"
AGENT_UUID="${AGENT_UUID:-UAI000000000000000001}"
GATEWAY="${GATEWAY:-http://127.0.0.1:8080}"
# A timestamp-derived suffix avoids collisions with retained experience users.
TELEPHONE="186$(printf '%08d' $(( $(date +%s) % 100000000 )))"
PASSWORD="b1-owner-definition-e2e-pass-123"

mysql() { docker exec "${PROJECT}-mysql-1" sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -N -B dipole -e "'"$1"'"'; }

echo "==> register + login a fresh owner (${TELEPHONE})"
registration=$(curl -sS -X POST "${GATEWAY}/api/v1/auth/register" -H 'content-type: application/json' \
  -d "{\"nickname\":\"B1 Fallback\",\"telephone\":\"${TELEPHONE}\",\"password\":\"${PASSWORD}\"}")
owner=$(printf '%s' "${registration}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("data", {}).get("user", {}).get("uuid", ""))')
[[ -n "${owner}" ]] || { echo "register failed: ${registration}" >&2; exit 1; }
login=$(curl -sS -X POST "${GATEWAY}/api/v1/auth/login" -H 'content-type: application/json' \
  -d "{\"telephone\":\"${TELEPHONE}\",\"password\":\"${PASSWORD}\"}")
token=$(printf '%s' "${login}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("data", {}).get("token", ""))')
[[ -n "${token}" ]] || { echo "login failed: ${login}" >&2; exit 1; }

echo "==> create the owner's default read-only Definition without a promotion grant"
definition=$(curl -sS -w '\n%{http_code}' -X POST "${GATEWAY}/api/v1/agent/definitions" \
  -H "authorization: Bearer ${token}")
definition_body=${definition%$'\n'*}
definition_status=${definition##*$'\n'}
[[ "${definition_status}" == "201" ]] || { echo "Definition create status=${definition_status}: ${definition_body}" >&2; exit 1; }
definition_uuid=$(printf '%s' "${definition_body}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("definitionId", ""))')
[[ -n "${definition_uuid}" ]] || { echo "Definition create returned no definitionId" >&2; exit 1; }
[[ "$(mysql "SELECT COUNT(*) FROM agent_runtime_promotion_grants WHERE definition_uuid='${definition_uuid}' AND revoked_at IS NULL")" == "0" ]] || {
  echo "owner Definition unexpectedly has an active promotion grant" >&2; exit 1;
}

echo "==> send a new DM to the assistant"
client_message_id="b1-owner-definition-${RANDOM}-$(date +%s)"
docker exec -i -e TOKEN="${token}" -e AGENT="${AGENT_UUID}" -e CLIENT_MESSAGE_ID="${client_message_id}" "${PROJECT}-agent-1" node --input-type=module - <<'NODE'
const [token, agent, clientMessageId] = [process.env.TOKEN, process.env.AGENT, process.env.CLIENT_MESSAGE_ID];
const socket = new WebSocket(`ws://gateway:8080/api/v1/ws?token=${encodeURIComponent(token)}&device=b1-owner-definition-e2e`);
await new Promise((resolve, reject) => {
  const timer = setTimeout(() => reject(new Error("send timeout")), 15000);
  socket.addEventListener("open", () => socket.send(JSON.stringify({ type: "chat.send", data: { target_uuid: agent, content: "B1 owner Definition fallback e2e", client_message_id: clientMessageId } })));
  socket.addEventListener("message", ({ data }) => { const event = JSON.parse(String(data)); if (event?.type === "chat.sent") { clearTimeout(timer); socket.close(); resolve(); } if (event?.type === "error") reject(new Error(`send failed: ${JSON.stringify(event.data)}`)); });
  socket.addEventListener("error", () => reject(new Error("socket failed")));
});
NODE

echo "==> wait for the governed task and assert low-risk fallback"
task_uuid=""
for _ in $(seq 1 90); do
  message_uuid=$(mysql "SELECT uuid FROM messages WHERE sender_uuid='${owner}' AND client_message_id='${client_message_id}' ORDER BY id DESC LIMIT 1")
  task_uuid=$(mysql "SELECT task_uuid FROM agent_tasks WHERE principal_uuid='${owner}' AND trigger_ref='${message_uuid}' LIMIT 1")
  state=$(mysql "SELECT CONCAT(status, ':', COALESCE(workflow_status, '')) FROM agent_tasks WHERE task_uuid='${task_uuid}'")
  [[ "${state}" == "completed:completed" ]] && break
  sleep 1
done
[[ -n "${task_uuid}" && "${state:-}" == "completed:completed" ]] || { echo "task did not complete: ${task_uuid:-none} ${state:-none}" >&2; exit 1; }
[[ "$(mysql "SELECT definition_uuid FROM agent_tasks WHERE task_uuid='${task_uuid}'")" == "lowrisk-assistant:v1" ]] || { echo "task did not use the shared low-risk Definition" >&2; exit 1; }
[[ "$(mysql "SELECT COUNT(*) FROM messages WHERE sender_uuid='${AGENT_UUID}' AND target_uuid='${owner}'")" == "1" ]] || { echo "assistant reply count drifted" >&2; exit 1; }
[[ "$(mysql "SELECT COUNT(*) FROM agent_approvals WHERE task_uuid='${task_uuid}' AND capability_id='message.assistant_reply.send' AND status='consumed'")" == "1" ]] || { echo "assistant approval count drifted" >&2; exit 1; }

echo "==> PASS: ungranted owner Definition fell back to lowrisk-assistant:v1 with one governed reply"
echo "    owner=${owner} owner_definition=${definition_uuid} task=${task_uuid}"
