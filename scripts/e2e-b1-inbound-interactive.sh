#!/usr/bin/env bash
# Route B/B1 E2E against the live experience stack (project dipole-experience).
# A brand-new (unprovisioned) user DMs the assistant. With
# DIPOLE_AGENT_INBOUND_INTERACTIVE_ENABLED on the agent and
# DIPOLE_AI_DIRECT_REPLY_ENABLED=false on core, the message must be answered by
# the governed interactive runtime (auto-enrolled onto the shared low-risk
# Definition + platform grant) — exactly one assistant reply, no legacy
# double-reply.
set -euo pipefail

PROJECT="${PROJECT:-dipole-experience}"
AGENT_UUID="${AGENT_UUID:-UAI000000000000000001}"
GATEWAY="${GATEWAY:-http://127.0.0.1:8080}"
TELEPHONE="188$(printf '%08d' $((RANDOM % 100000000)))"
PASSWORD="b1-e2e-pass-123"

mysql() { docker exec "${PROJECT}-mysql-1" sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -N -B dipole -e "'"$1"'"'; }

echo "==> register + login new user (${TELEPHONE})"
reg=$(curl -s -X POST "${GATEWAY}/api/v1/auth/register" -H 'content-type: application/json' \
  -d "{\"nickname\":\"B1 E2E\",\"telephone\":\"${TELEPHONE}\",\"password\":\"${PASSWORD}\"}")
owner=$(printf '%s' "$reg" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["user"]["uuid"])')
[ -n "$owner" ] || { echo "register failed: $reg" >&2; exit 1; }
login=$(curl -s -X POST "${GATEWAY}/api/v1/auth/login" -H 'content-type: application/json' \
  -d "{\"telephone\":\"${TELEPHONE}\",\"password\":\"${PASSWORD}\"}")
token=$(printf '%s' "$login" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["token"])')
[ -n "$token" ] || { echo "login failed: $login" >&2; exit 1; }
echo "    owner=${owner}"

echo "==> send one DM to the assistant over WS (no prior Definition/grant for this user)"
docker exec -i -e OWNER="$owner" -e TOKEN="$token" -e AGENT="$AGENT_UUID" "${PROJECT}-agent-1" node --input-type=module - <<'NODE'
const [owner, token, agent] = [process.env.OWNER, process.env.TOKEN, process.env.AGENT];
const socket = new WebSocket(`ws://gateway:8080/api/v1/ws?token=${encodeURIComponent(token)}&device=b1-e2e`);
await new Promise((resolve, reject) => {
  const timer = setTimeout(() => reject(new Error("send timeout")), 15000);
  socket.addEventListener("open", () => socket.send(JSON.stringify({ type: "chat.send", data: { target_uuid: agent, content: "B1 inbound interactive e2e", client_message_id: `b1-e2e-${Date.now()}` } })));
  socket.addEventListener("message", ({ data }) => { const e = JSON.parse(String(data)); if (e?.type === "chat.sent") { clearTimeout(timer); socket.close(); resolve(); } if (e?.type === "error") reject(new Error(`send failed: ${JSON.stringify(e.data)}`)); });
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

echo "==> assert exactly one assistant reply, no double reply"
reply_count=$(mysql "SELECT COUNT(*) FROM messages WHERE sender_uuid='${AGENT_UUID}' AND target_uuid='${owner}'")
[ "$reply_count" = "1" ] || { echo "assistant replies=${reply_count}, want exactly 1 (double-reply or none)" >&2; exit 1; }

echo "==> assert one consumed assistant_reply approval for this task"
approvals=$(mysql "SELECT COUNT(*) FROM agent_approvals WHERE task_uuid='${task_uuid}' AND capability_id='message.assistant_reply.send' AND status='consumed'")
[ "$approvals" = "1" ] || { echo "consumed assistant_reply approvals=${approvals}, want 1" >&2; exit 1; }

echo "==> PASS: unprovisioned user's first DM was answered by the governed interactive path (auto-enrolled, single reply, consumed approval)"
echo "    owner=${owner} task=${task_uuid} replies=${reply_count} consumed_approvals=${approvals}"
