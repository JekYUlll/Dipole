#!/usr/bin/env bash
# Verify Route B/B1 preserves short conversation context across two separately
# durable inbound tasks in the live experience stack.
set -euo pipefail

PROJECT="${PROJECT:-dipole-experience}"
AGENT_UUID="${AGENT_UUID:-UAI000000000000000001}"
GATEWAY="${GATEWAY:-http://127.0.0.1:8080}"
TELEPHONE="186$(printf '%08d' $((RANDOM % 100000000)))"
PASSWORD="b1-context-e2e-pass-123"
CONTEXT_TOKEN="DIPOLECTX$(date +%s)${RANDOM}"

mysql() { docker exec "${PROJECT}-mysql-1" sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -N -B dipole -e "'"$1"'"'; }

send_dm() {
  local content="$1" client_message_id="$2"
  docker exec -i -e TOKEN="$token" -e AGENT="$AGENT_UUID" -e CONTENT="$content" -e CLIENT_MESSAGE_ID="$client_message_id" "${PROJECT}-agent-1" node --input-type=module - <<'NODE'
const [token, agent, content, clientMessageId] = [process.env.TOKEN, process.env.AGENT, process.env.CONTENT, process.env.CLIENT_MESSAGE_ID];
const socket = new WebSocket(`ws://gateway:8080/api/v1/ws?token=${encodeURIComponent(token)}&device=b1-context-e2e`);
await new Promise((resolve, reject) => {
  const timer = setTimeout(() => reject(new Error("send timeout")), 15000);
  socket.addEventListener("open", () => socket.send(JSON.stringify({
    type: "chat.send", data: { target_uuid: agent, content, client_message_id: clientMessageId }
  })));
  socket.addEventListener("message", ({ data }) => {
    const event = JSON.parse(String(data));
    if (event?.type === "chat.sent") { clearTimeout(timer); socket.close(); resolve(); }
    if (event?.type === "error") reject(new Error(`send failed: ${JSON.stringify(event.data)}`));
  });
  socket.addEventListener("error", () => reject(new Error("socket failed")));
});
NODE
}

wait_for_completed_task() {
  local client_message_id="$1" message_uuid="" task_uuid="" state=""
  for _ in $(seq 1 90); do
    message_uuid=$(mysql "SELECT uuid FROM messages WHERE sender_uuid='${owner}' AND client_message_id='${client_message_id}' ORDER BY id DESC LIMIT 1")
    if [[ -n "${message_uuid}" ]]; then
      task_uuid=$(mysql "SELECT task_uuid FROM agent_tasks WHERE principal_uuid='${owner}' AND trigger_type='agent.interactive.requested' AND trigger_ref='${message_uuid}' ORDER BY created_at DESC LIMIT 1")
      if [[ -n "${task_uuid}" ]]; then
        state=$(mysql "SELECT CONCAT(status,':',COALESCE(workflow_status,'')) FROM agent_tasks WHERE task_uuid='${task_uuid}'")
        if [[ "${state%%:*}" == "completed" ]]; then
          printf '%s\n' "${task_uuid}"
          return 0
        fi
      fi
    fi
    sleep 1
  done
  echo "interactive task did not complete for client_message_id=${client_message_id}, task=${task_uuid:-none}, state=${state:-none}" >&2
  return 1
}

echo "==> register + login a fresh user"
registration=$(curl -fsS -X POST "${GATEWAY}/api/v1/auth/register" -H 'content-type: application/json' \
  -d "{\"nickname\":\"B1 Context E2E\",\"telephone\":\"${TELEPHONE}\",\"password\":\"${PASSWORD}\"}")
owner=$(printf '%s' "${registration}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["user"]["uuid"])')
login=$(curl -fsS -X POST "${GATEWAY}/api/v1/auth/login" -H 'content-type: application/json' \
  -d "{\"telephone\":\"${TELEPHONE}\",\"password\":\"${PASSWORD}\"}")
token=$(printf '%s' "${login}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])')

first_client_message_id="b1-context-first-${RANDOM}-$(date +%s)"
echo "==> send the first message with a unique context token"
send_dm "Please remember my temporary code: ${CONTEXT_TOKEN}. Reply only that you remembered it." "${first_client_message_id}"
first_task=$(wait_for_completed_task "${first_client_message_id}")

before_second_reply_id=$(mysql "SELECT COALESCE(MAX(id), 0) FROM messages WHERE sender_uuid='${AGENT_UUID}' AND target_uuid='${owner}'")
second_client_message_id="b1-context-second-${RANDOM}-$(date +%s)"
echo "==> ask for the token in a new inbound task"
send_dm "From our current conversation, tell me the temporary code I just gave you. Include the complete code in your reply." "${second_client_message_id}"
second_task=$(wait_for_completed_task "${second_client_message_id}")

reply_count=$(mysql "SELECT COUNT(*) FROM messages WHERE sender_uuid='${AGENT_UUID}' AND target_uuid='${owner}' AND id > ${before_second_reply_id}")
[[ "${reply_count}" == "1" ]] || { echo "second turn assistant replies=${reply_count}, want exactly 1" >&2; exit 1; }
second_reply=$(mysql "SELECT content FROM messages WHERE sender_uuid='${AGENT_UUID}' AND target_uuid='${owner}' AND id > ${before_second_reply_id} ORDER BY id DESC LIMIT 1")
[[ "${second_reply}" == *"${CONTEXT_TOKEN}"* ]] || { echo "second reply did not preserve the first-turn context token" >&2; exit 1; }

echo "==> PASS: two governed inbound tasks completed with one reply each and the second reply retained short conversation context"
echo "    first_task=${first_task} second_task=${second_task} context_token=${CONTEXT_TOKEN}"
