#!/usr/bin/env bash
# Verifies that a new authenticated user can create a low-risk explicit Agent
# Task through Gateway and recover its durable outcome from the public stack.
set -euo pipefail

PROJECT="${PROJECT:-dipole-experience}"
AGENT_UUID="${AGENT_UUID:-UAI000000000000000001}"
GATEWAY="${GATEWAY:-http://127.0.0.1:8080}"
TELEPHONE="187$(printf '%08d' $((RANDOM % 100000000)))"
PASSWORD="explicit-task-e2e-pass-123"

mysql() { docker exec "${PROJECT}-mysql-1" sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -N -B dipole -e "'"$1"'"'; }

echo "==> register + login a new explicit-task owner (${TELEPHONE})"
registration=$(curl -fsS -X POST "${GATEWAY}/api/v1/auth/register" -H 'content-type: application/json' \
  -d "{\"nickname\":\"Explicit Task E2E\",\"telephone\":\"${TELEPHONE}\",\"password\":\"${PASSWORD}\"}")
owner_uuid=$(printf '%s' "${registration}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["user"]["uuid"])')
token=$(printf '%s' "${registration}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])')
[[ -n "${owner_uuid}" && -n "${token}" ]] || { printf 'register did not return an owner session\n' >&2; exit 1; }

echo "==> create one low-risk explicit task through Gateway"
request_id="explicit-e2e-$(date +%s)-${RANDOM}"
response=$(curl -fsS -X POST "${GATEWAY}/api/v1/agent/tasks" \
  -H 'content-type: application/json' -H "authorization: Bearer ${token}" \
  -d "{\"client_request_id\":\"${request_id}\",\"goal\":\"Summarize my recent conversations\"}")
task_uuid=$(printf '%s' "${response}" | python3 -c 'import json,sys; body=json.load(sys.stdin); print(body.get("taskId", ""))')
status=$(printf '%s' "${response}" | python3 -c 'import json,sys; body=json.load(sys.stdin); print(body.get("status", ""))')
[[ -n "${task_uuid}" && "${status}" == "accepted" ]] || { printf 'task start was not accepted: %s\n' "${response}" >&2; exit 1; }

echo "==> wait for the durable task and its Timeline projection"
state=""
for _ in $(seq 1 90); do
  state=$(mysql "SELECT status FROM agent_tasks WHERE task_uuid='${task_uuid}'")
  [[ "${state}" == "completed" ]] && break
  sleep 1
done
[[ "${state}" == "completed" ]] || { printf 'explicit task did not complete: %s\n' "${state}" >&2; exit 1; }
definition_uuid=$(mysql "SELECT definition_uuid FROM agent_tasks WHERE task_uuid='${task_uuid}'")
[[ "${definition_uuid}" == "lowrisk-assistant:v1" ]] || { printf 'explicit task did not use low-risk Definition: %s\n' "${definition_uuid}" >&2; exit 1; }

timeline_count=$(mysql "SELECT COUNT(*) FROM agent_task_timeline_events WHERE task_uuid='${task_uuid}'")
reply_count=$(mysql "SELECT COUNT(*) FROM messages WHERE sender_uuid='${AGENT_UUID}' AND target_uuid='${owner_uuid}'")
[[ "${timeline_count}" -gt 0 ]] || { printf 'explicit task did not publish Timeline events\n' >&2; exit 1; }
[[ "${reply_count}" == "1" ]] || { printf 'explicit task replies=%s, want exactly 1\n' "${reply_count}" >&2; exit 1; }

echo "==> PASS: explicit task completed through low-risk authority and has a durable Timeline"
echo "    owner=${owner_uuid} task=${task_uuid} timeline_events=${timeline_count} replies=${reply_count}"
