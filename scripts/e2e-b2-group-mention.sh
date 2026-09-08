#!/usr/bin/env bash
# Route B/B2 E2E against the live experience stack (project dipole-experience).
# A brand-new (unprovisioned) user creates a group, adds the assistant as a
# member, then @-mentions the assistant. With
# DIPOLE_AGENT_INBOUND_GROUP_INTERACTIVE_ENABLED on the agent and the group
# message consumed by the governed runtime, the mention must be answered by a
# single group assistant reply (auto-enrolled onto the shared low-risk
# Definition + platform grant) — no legacy double-reply.
set -euo pipefail

PROJECT="${PROJECT:-dipole-experience}"
AGENT_UUID="${AGENT_UUID:-UAI000000000000000001}"
GATEWAY="${GATEWAY:-http://127.0.0.1:8080}"
TELEPHONE="189$(printf '%08d' $((RANDOM % 100000000)))"
PASSWORD="b2-e2e-pass-123"

mysql() { docker exec "${PROJECT}-mysql-1" sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -N -B dipole -e "'"$1"'"'; }

echo "==> register + login new user (${TELEPHONE})"
reg=$(curl -s -X POST "${GATEWAY}/api/v1/auth/register" -H 'content-type: application/json' \
  -d "{\"nickname\":\"B2 E2E\",\"telephone\":\"${TELEPHONE}\",\"password\":\"${PASSWORD}\"}")
owner=$(printf '%s' "$reg" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["user"]["uuid"])')
[ -n "$owner" ] || { echo "register failed: $reg" >&2; exit 1; }
login=$(curl -s -X POST "${GATEWAY}/api/v1/auth/login" -H 'content-type: application/json' \
  -d "{\"telephone\":\"${TELEPHONE}\",\"password\":\"${PASSWORD}\"}")
token=$(printf '%s' "$login" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["token"])')
[ -n "$token" ] || { echo "login failed: $login" >&2; exit 1; }
echo "    owner=${owner}"

echo "==> create a group and add the assistant as a member"
group=$(curl -s -X POST "${GATEWAY}/api/v1/groups" -H "content-type: application/json" -H "authorization: Bearer ${token}" \
  -d "{\"name\":\"B2 E2E Room\",\"member_uuids\":[\"${AGENT_UUID}\"]}")
group_uuid=$(printf '%s' "$group" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["uuid"])')
[ -n "$group_uuid" ] || { echo "create group failed: $group" >&2; exit 1; }
echo "    group=${group_uuid}"

echo "==> @-mention the assistant in the group over WS"
docker exec -i -e OWNER="$owner" -e TOKEN="$token" -e GROUP="$group_uuid" "${PROJECT}-agent-1" node --input-type=module - <<'NODE'
const [owner, token, group] = [process.env.OWNER, process.env.TOKEN, process.env.GROUP];
const socket = new WebSocket(`ws://gateway:8080/api/v1/ws?token=${encodeURIComponent(token)}&device=b2-e2e`);
await new Promise((resolve, reject) => {
  const timer = setTimeout(() => reject(new Error("send timeout")), 15000);
  socket.addEventListener("open", () => socket.send(JSON.stringify({
    type: "chat.send", data: { target_uuid: group, content: "@Dipole AI B2 group mention e2e", client_message_id: `b2-e2e-${Date.now()}` }
  })));
  socket.addEventListener("message", ({ data }) => {
    const e = JSON.parse(String(data));
    if (e?.type === "chat.sent") { clearTimeout(timer); socket.close(); resolve(); }
    if (e?.type === "error") reject(new Error(`send failed: ${JSON.stringify(e.data)}`));
  });
  socket.addEventListener("error", () => reject(new Error("socket failed")));
});
NODE

echo "==> wait for the governed interactive task to complete"
task_uuid=""
for _ in $(seq 1 90); do
  task_uuid=$(mysql "SELECT task_uuid FROM agent_tasks WHERE principal_uuid='${owner}' AND trigger_type='agent.interactive.requested' ORDER BY created_at DESC LIMIT 1")
  if [ -n "$task_uuid" ]; then
    state=$(mysql "SELECT CONCAT(status,':',COALESCE(workflow_status,'')) FROM agent_tasks WHERE task_uuid='${task_uuid}'")
    [ "${state%%:*}" = "completed" ] && break
  fi
  sleep 1
done
[ -n "$task_uuid" ] || { echo "no interactive task created for ${owner}" >&2; exit 1; }
echo "    task=${task_uuid} state=$(mysql "SELECT CONCAT(status,':',COALESCE(workflow_status,'')) FROM agent_tasks WHERE task_uuid='${task_uuid}'")"

echo "==> assert it pinned the shared low-risk Definition"
pinned=$(mysql "SELECT definition_uuid FROM agent_tasks WHERE task_uuid='${task_uuid}'")
[ "$pinned" = "lowrisk-assistant:v1" ] || { echo "task pinned '$pinned', want lowrisk-assistant:v1" >&2; exit 1; }

echo "==> assert exactly one assistant group reply, no double reply"
reply_count=$(mysql "SELECT COUNT(*) FROM messages WHERE sender_uuid='${AGENT_UUID}' AND target_uuid='${group_uuid}' AND target_type=1")
[ "$reply_count" = "1" ] || { echo "assistant group replies=${reply_count}, want exactly 1 (double-reply or none)" >&2; exit 1; }

echo "==> assert one consumed group_reply approval for this task"
approvals=$(mysql "SELECT COUNT(*) FROM agent_approvals WHERE task_uuid='${task_uuid}' AND capability_id='message.group_reply.send' AND status='consumed'")
[ "$approvals" = "1" ] || { echo "consumed group_reply approvals=${approvals}, want 1" >&2; exit 1; }

echo "==> PASS: unprovisioned user's group @-mention was answered by the governed interactive path (auto-enrolled, single group reply, consumed approval)"
echo "    owner=${owner} group=${group_uuid} task=${task_uuid} replies=${reply_count} consumed_approvals=${approvals}"
