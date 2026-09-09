#!/usr/bin/env bash
set -euo pipefail

# Generates a governed B1 direct-message reply and reads its Sync page without
# advancing a device cursor, so a shadow window can compare hydration.
project="${PROJECT:-dipole-experience}"
gateway="${GATEWAY:-http://127.0.0.1:8080}"
agent_uuid="${AGENT_UUID:-UAI000000000000000001}"
password="sync-shadow-window-pass-123"
owner_phone="186$(printf '%08d' $((RANDOM % 100000000)))"

register() {
  curl -fsS -X POST "${gateway}/api/v1/auth/register" -H 'content-type: application/json' \
    -d "{\"nickname\":\"Sync Shadow Window\",\"telephone\":\"$1\",\"password\":\"${password}\"}"
}
field() { python3 -c "import json,sys; print(json.load(sys.stdin)$1)"; }
mysql() { docker exec "${project}-mysql-1" sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot -N -B dipole -e "'"$1"'"'; }

owner_registration=$(register "${owner_phone}")
owner_uuid=$(printf '%s' "${owner_registration}" | field '["data"]["user"]["uuid"]')
owner_token=$(printf '%s' "${owner_registration}" | field '["data"]["token"]')

docker exec -i -e TOKEN="${owner_token}" -e TARGET="${agent_uuid}" "${project}-agent-1" node --input-type=module - <<'NODE'
const socket = new WebSocket(`ws://gateway:8080/api/v1/ws?token=${encodeURIComponent(process.env.TOKEN)}&device=sync-shadow-window`);
await new Promise((resolve, reject) => {
  const timer = setTimeout(() => reject(new Error("direct-message send timed out")), 15_000);
  socket.addEventListener("open", () => socket.send(JSON.stringify({type: "chat.send", data: {target_uuid: process.env.TARGET, content: "sync shadow window exercise", client_message_id: `sync-shadow-${Date.now()}`}})));
  socket.addEventListener("message", ({data}) => {
    const event = JSON.parse(String(data));
    if (event?.type === "chat.sent") { clearTimeout(timer); socket.close(); resolve(); }
    if (event?.type === "error") reject(new Error(JSON.stringify(event.data)));
  });
  socket.addEventListener("error", () => reject(new Error("direct-message socket failed")));
});
NODE

for _ in $(seq 1 90); do
  inbox_count=$(mysql "SELECT COUNT(*) FROM user_sync_inbox WHERE user_uuid='${owner_uuid}'")
  reply_count=$(mysql "SELECT COUNT(*) FROM messages WHERE sender_uuid='${agent_uuid}' AND target_uuid='${owner_uuid}'")
  [[ "${inbox_count}" -gt 0 && "${reply_count}" -eq 1 ]] && break
  sleep 1
done
[[ "${inbox_count:-0}" -gt 0 && "${reply_count:-0}" -eq 1 ]] || { echo "Agent reply Inbox projection was not observed" >&2; exit 1; }

for _ in $(seq 1 5); do
  page=$(curl -fsS "${gateway}/api/v1/sync?after_seq=0&limit=20" -H "authorization: Bearer ${owner_token}" -H 'X-Device-ID: sync-shadow-window')
  count=$(printf '%s' "${page}" | python3 -c 'import json,sys; print(len(json.load(sys.stdin).get("data", {}).get("items", [])))')
  [[ "${count}" -gt 0 ]] || { echo "Sync page was empty" >&2; exit 1; }
done

printf 'Sync shadow exercise passed: owner=%s inbox=%s agent_replies=%s sync_pages=5\n' "${owner_uuid}" "${inbox_count}" "${reply_count}"
