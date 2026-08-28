#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
project="dipole-agent-timeline-repair-${RANDOM}-$$"
network="${project}-network"
mysql_container="${project}-mysql"
work_dir=$(mktemp -d /tmp/dipole-agent-timeline-repair.XXXXXX)
config_file="${work_dir}/config.yaml"
trap 'status=$?; docker rm -f "$mysql_container" >/dev/null 2>&1 || true; docker network rm "$network" >/dev/null 2>&1 || true; rm -rf "$work_dir"; exit "$status"' EXIT INT TERM

docker network create "$network" >/dev/null
docker run -d --name "$mysql_container" --network "$network" --network-alias mysql \
  -e MYSQL_ROOT_PASSWORD=repair-root -e MYSQL_ROOT_HOST=% -e MYSQL_DATABASE=dipole \
  -p 127.0.0.1::3306 mysql:8.4 >/dev/null

for _ in $(seq 1 90); do
  docker exec "$mysql_container" mysqladmin ping -h 127.0.0.1 -uroot -prepair-root --silent >/dev/null 2>&1 && break
  sleep 1
done
docker exec "$mysql_container" mysqladmin ping -h 127.0.0.1 -uroot -prepair-root --silent >/dev/null
mysql_port=$(docker port "$mysql_container" 3306/tcp | sed -n 's/.*:\([0-9][0-9]*\)$/\1/p')
[[ -n "$mysql_port" ]]

cat >"$config_file" <<YAML
mysql:
  host: 127.0.0.1
  port: ${mysql_port}
  user: root
  password: repair-root
  dbname: dipole
YAML

(
  cd "$root_dir"
  CGO_ENABLED=0 go build -o "$work_dir/dipole-migrate" ./cmd/migrate
  CGO_ENABLED=0 go build -o "$work_dir/dipole-agent-task-timeline-repair" ./cmd/agent-task-timeline-repair
)

DIPOLE_CONFIG_FILE="$config_file" DIPOLE_MYSQL_HOST=127.0.0.1 DIPOLE_MYSQL_PORT="$mysql_port" \
  DIPOLE_MYSQL_USER=root DIPOLE_MYSQL_PASSWORD=repair-root DIPOLE_MYSQL_DBNAME=dipole \
  "$work_dir/dipole-migrate" -direction up >/dev/null
docker exec -i "$mysql_container" mysql -uroot -prepair-root dipole <<'SQL'
INSERT INTO agent_definition_versions (
  definition_uuid, version, tenant_id, owner_uuid, agent_uuid, status,
  permissions_json, scopes_json, valid_from
) VALUES (
  'DEF-SMOKE-REPAIR', 1, 'dipole', 'U-SMOKE-REPAIR', 'A-SMOKE-REPAIR', 'active',
  JSON_ARRAY('conversation.read'), JSON_ARRAY(), NOW(3)
);
INSERT INTO agent_tasks (
  task_uuid, definition_uuid, definition_version, tenant_id, principal_uuid, agent_uuid,
  status, trigger_type, trigger_ref, goal
) VALUES (
  'TASK-SMOKE-REPAIR', 'DEF-SMOKE-REPAIR', 1, 'dipole', 'U-SMOKE-REPAIR', 'A-SMOKE-REPAIR',
  'running', 'smoke', 'agent-timeline-repair', 'verify process replay'
);
INSERT INTO agent_runs (
  run_uuid, task_uuid, runtime_id, mode, status, started_at
) VALUES (
  'RUN-SMOKE-REPAIR', 'TASK-SMOKE-REPAIR', 'dipole-agent', 'shadow', 'running', NOW(3)
);
INSERT INTO agent_task_timeline_repairs (
  event_uuid, task_uuid, run_uuid, event_kind, status, occurred_at, next_retry_at
) VALUES (
  'EVENT-SMOKE-REPAIR', 'TASK-SMOKE-REPAIR', 'RUN-SMOKE-REPAIR',
  'model_call', 'completed', NOW(3), NOW(3)
);
SQL

seed_count=$(docker exec "$mysql_container" mysql -N -uroot -prepair-root dipole \
  -e "SELECT COUNT(*) FROM agent_task_timeline_repairs WHERE event_uuid = 'EVENT-SMOKE-REPAIR';")
if [[ "$seed_count" != "1" ]]; then
  printf 'Repair smoke seed was not persisted: count=%q\n' "$seed_count" >&2
  docker exec "$mysql_container" mysql -uroot -prepair-root dipole \
    -e "SELECT DATABASE(), @@port; SELECT COUNT(*) AS definitions FROM agent_definition_versions; SELECT COUNT(*) AS tasks FROM agent_tasks; SELECT COUNT(*) AS runs FROM agent_runs; SELECT COUNT(*) AS repairs FROM agent_task_timeline_repairs;" >&2 || true
  exit 1
fi

set +e
DIPOLE_CONFIG_FILE="$config_file" DIPOLE_MYSQL_HOST=127.0.0.1 DIPOLE_MYSQL_PORT="$mysql_port" \
  DIPOLE_MYSQL_USER=root DIPOLE_MYSQL_PASSWORD=repair-root DIPOLE_MYSQL_DBNAME=dipole \
  timeout 5s "$work_dir/dipole-agent-task-timeline-repair" \
  -batch-size 10 -lease 2s -retry-backoff 1s -interval 100ms \
  -metrics-address 127.0.0.1:0 >/dev/null 2>"$work_dir/repair.log"
worker_status=$?
set -e
if [[ "$worker_status" -ne 124 ]]; then
  cat "$work_dir/repair.log" >&2
  printf 'Expected the bounded smoke worker to be stopped by timeout, got %d\n' "$worker_status" >&2
  exit 1
fi

repair_state=$(docker exec "$mysql_container" mysql -N -uroot -prepair-root dipole \
  -e "SELECT repair_status, COUNT(*) FROM agent_task_timeline_repairs WHERE event_uuid = 'EVENT-SMOKE-REPAIR' GROUP BY repair_status;")
event_count=$(docker exec "$mysql_container" mysql -N -uroot -prepair-root dipole \
  -e "SELECT COUNT(*) FROM agent_task_timeline_events WHERE event_uuid = 'EVENT-SMOKE-REPAIR';")
if [[ "$repair_state" != $'completed\t1' || "$event_count" != "1" ]]; then
  printf 'Repair process did not converge: state=%q events=%q\n' "$repair_state" "$event_count" >&2
  cat "$work_dir/repair.log" >&2
  exit 1
fi

printf 'Agent Timeline repair process smoke passed: isolated worker replayed one intent and converged idempotently.\n'
