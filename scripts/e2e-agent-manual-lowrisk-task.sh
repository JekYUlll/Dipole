#!/usr/bin/env bash
# Verifies that a new user can create a governed interactive task without an
# owner Definition. This is intentionally separate from subscription triggers,
# which still require an owner-reviewed promotion grant.
set -euo pipefail

PROJECT="${PROJECT:-dipole-experience}"
GATEWAY="${GATEWAY:-http://127.0.0.1:8080}"
TELEPHONE="187$(printf '%08d' $((RANDOM % 100000000)))"
PASSWORD="manual-task-e2e-pass-123"

mysql() { docker exec "${PROJECT}-mysql-1" sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -N -B dipole -e "'"$1"'"'; }

echo "==> register + login new user (${TELEPHONE})"
registration=$(curl -sf -X POST "${GATEWAY}/api/v1/auth/register" -H 'content-type: application/json' \
  -d "{\"nickname\":\"Manual Task E2E\",\"telephone\":\"${TELEPHONE}\",\"password\":\"${PASSWORD}\"}")
owner=$(printf '%s' "$registration" | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["user"]["uuid"])')
login=$(curl -sf -X POST "${GATEWAY}/api/v1/auth/login" -H 'content-type: application/json' \
  -d "{\"telephone\":\"${TELEPHONE}\",\"password\":\"${PASSWORD}\"}")
token=$(printf '%s' "$login" | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["token"])')

echo "==> create an interactive task without an owner Definition or grant"
response=$(curl -sS -w '\n%{http_code}' -X POST "${GATEWAY}/api/v1/agent/tasks" \
  -H 'content-type: application/json' -H "authorization: Bearer ${token}" \
  -d '{"client_request_id":"manual-lowrisk-e2e","goal":"请简要说明你可以做什么。"}')
body=${response%$'\n'*}
status=${response##*$'\n'}
[ "$status" = "202" ] || { echo "task create status=${status}: ${body}" >&2; exit 1; }
task_uuid=$(printf '%s' "$body" | python3 -c 'import json,sys;value=json.load(sys.stdin);print(value.get("taskId", ""))')
[ -n "$task_uuid" ] || { echo "task create did not return taskId" >&2; exit 1; }

echo "==> wait for the durable task to complete"
state=""
for _ in $(seq 1 45); do
  state=$(mysql "SELECT CONCAT(status, ':', COALESCE(workflow_status, ''), ':', definition_uuid) FROM agent_tasks WHERE task_uuid='${task_uuid}'")
  [ "${state%%:*}" = "completed" ] && break
  case "$state" in failed:*|cancelled:*) break;; esac
  sleep 1
done
[ "${state%%:*}" = "completed" ] || { echo "task did not complete: ${state}" >&2; exit 1; }

echo "==> assert the task used the shared low-risk Definition"
pinned=$(mysql "SELECT definition_uuid FROM agent_tasks WHERE task_uuid='${task_uuid}'")
[ "$pinned" = "lowrisk-assistant:v1" ] || { echo "task pinned ${pinned}, want lowrisk-assistant:v1" >&2; exit 1; }
owner_definitions=$(mysql "SELECT COUNT(*) FROM agent_definition_versions WHERE owner_uuid='${owner}'")
[ "$owner_definitions" = "0" ] || { echo "new owner unexpectedly has ${owner_definitions} Definition(s)" >&2; exit 1; }

echo "==> PASS: explicit interactive task completed through the platform low-risk Definition"
echo "    task=${task_uuid} state=${state} owner_definitions=${owner_definitions}"
