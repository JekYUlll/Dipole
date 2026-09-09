#!/usr/bin/env bash
set -euo pipefail

# Generates one ordinary direct-message Sync page without advancing a device
# cursor, so a shadow window can compare MySQL and Cassandra hydration.
project="${PROJECT:-dipole-experience}"
gateway="${GATEWAY:-http://127.0.0.1:8080}"
password="sync-shadow-window-pass-123"
sender_phone="186$(printf '%08d' $((RANDOM % 100000000)))"
receiver_phone="185$(printf '%08d' $((RANDOM % 100000000)))"

register() {
  curl -fsS -X POST "${gateway}/api/v1/auth/register" -H 'content-type: application/json' \
    -d "{\"nickname\":\"Sync Shadow Window\",\"telephone\":\"$1\",\"password\":\"${password}\"}"
}
field() { python3 -c "import json,sys; print(json.load(sys.stdin)$1)"; }
mysql() { docker exec "${project}-mysql-1" sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot -N -B dipole -e "'"$1"'"'; }

sender_registration=$(register "${sender_phone}")
receiver_registration=$(register "${receiver_phone}")
sender_uuid=$(printf '%s' "${sender_registration}" | field '["data"]["user"]["uuid"]')
receiver_uuid=$(printf '%s' "${receiver_registration}" | field '["data"]["user"]["uuid"]')
receiver_token=$(printf '%s' "${receiver_registration}" | field '["data"]["token"]')

docker exec -i -e TOKEN="$(printf '%s' "${sender_registration}" | field '["data"]["token"]')" -e TARGET="${receiver_uuid}" "${project}-agent-1" node --input-type=module - <<'NODE'
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

for _ in $(seq 1 30); do
  inbox_count=$(mysql "SELECT COUNT(*) FROM user_sync_inbox WHERE user_uuid='${receiver_uuid}'")
  [[ "${inbox_count}" -gt 0 ]] && break
  sleep 1
done
[[ "${inbox_count:-0}" -gt 0 ]] || { echo "receiver Inbox projection was not observed" >&2; exit 1; }

for _ in $(seq 1 5); do
  page=$(curl -fsS "${gateway}/api/v1/sync?after_seq=0&limit=20" -H "authorization: Bearer ${receiver_token}" -H 'X-Device-ID: sync-shadow-window')
  count=$(printf '%s' "${page}" | python3 -c 'import json,sys; print(len(json.load(sys.stdin).get("data", {}).get("items", [])))')
  [[ "${count}" -gt 0 ]] || { echo "Sync page was empty" >&2; exit 1; }
done

printf 'Sync shadow exercise passed: sender=%s receiver=%s inbox=%s sync_pages=5\n' "${sender_uuid}" "${receiver_uuid}" "${inbox_count}"
