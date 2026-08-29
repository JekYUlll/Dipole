#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root_dir=$(cd "${script_dir}/.." && pwd)
compose_file="${root_dir}/deploy/compose/docker-compose.microservices.yml"
project_name="${COMPOSE_PROJECT_NAME:-dipole-agent-timeline-repair-compose-${RANDOM}-$$}"

if [[ "${BUILD_IMAGE:-0}" == "1" ]]; then
  image_name="${IMAGE_NAME:-dipole-agent-timeline-repair}"
  image_tag="${IMAGE_TAG:-agent-timeline-repair-compose-smoke}"
  "${script_dir}/docker-build.sh" backend
  DIPOLE_AGENT_TIMELINE_REPAIR_IMAGE="${image_name}:${image_tag}" \
    "${script_dir}/docker-build-microservice-images.sh"
  export DIPOLE_AGENT_TIMELINE_REPAIR_IMAGE="${image_name}:${image_tag}"
fi

: "${DIPOLE_AGENT_TIMELINE_REPAIR_IMAGE:=dipole-agent-timeline-repair:latest}"
: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
: "${DIPOLE_AGENT_TIMELINE_REPAIR_MYSQL_PASSWORD:=repair-compose-password}"
export DIPOLE_AGENT_TIMELINE_REPAIR_IMAGE DIPOLE_INTERNAL_RPC_SHARED_SECRET DIPOLE_AGENT_TIMELINE_REPAIR_MYSQL_PASSWORD

compose() {
  docker compose -p "${project_name}" -f "${compose_file}" --profile agent-timeline-repair "$@"
}

cleanup() {
  local status=$?
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  else
    printf 'Agent Timeline repair Compose stack retained: project=%s\n' "${project_name}"
  fi
  exit "${status}"
}
trap cleanup EXIT INT TERM

compose config --quiet
compose up -d --wait mysql
compose run --rm --no-deps migrate

migration_state=$(compose exec -T mysql mysql -N -uroot -proot123 dipole \
  -e 'SELECT MAX(version), COUNT(*) FROM schema_migrations;')
timeline_table=$(compose exec -T mysql mysql -N -uroot -proot123 dipole \
  -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'dipole' AND table_name = 'agent_task_timeline_events';")
timezone_state=$(compose exec -T mysql mysql -N -uroot -proot123 dipole \
  -e 'SELECT @@global.time_zone, @@session.time_zone;')
if [[ "${migration_state}" != $'50\t50' || "${timeline_table}" != "1" || "${timezone_state}" != $'+00:00\t+00:00' ]]; then
  printf 'Compose migration preflight failed: state=%q timeline_table=%q (want version/count 50/50 and table=1)\n' \
    "${migration_state}" "${timeline_table}" >&2
  printf 'Compose timezone preflight failed: timezone_state=%q (want +00:00/+00:00)\n' "${timezone_state}" >&2
  compose logs migrate >&2 || true
  exit 1
fi

compose up -d --wait mysql-permissions

compose exec -T mysql mysql -uroot -proot123 dipole <<'SQL'
INSERT INTO agent_definition_versions (
  definition_uuid, version, tenant_id, owner_uuid, agent_uuid, status,
  permissions_json, scopes_json, valid_from
) VALUES (
  'DEF-SMOKE-COMPOSE-REPAIR', 1, 'dipole', 'U-SMOKE-COMPOSE', 'A-SMOKE-COMPOSE', 'active',
  JSON_ARRAY('conversation.read'), JSON_ARRAY(), NOW(3)
);
INSERT INTO agent_tasks (
  task_uuid, definition_uuid, definition_version, tenant_id, principal_uuid, agent_uuid,
  status, trigger_type, trigger_ref, goal
) VALUES (
  'TASK-SMOKE-COMPOSE-REPAIR', 'DEF-SMOKE-COMPOSE-REPAIR', 1, 'dipole', 'U-SMOKE-COMPOSE', 'A-SMOKE-COMPOSE',
  'running', 'smoke', 'agent-timeline-repair-compose', 'verify compose repair'
);
INSERT INTO agent_runs (
  run_uuid, task_uuid, runtime_id, mode, status, started_at
) VALUES (
  'RUN-SMOKE-COMPOSE-REPAIR', 'TASK-SMOKE-COMPOSE-REPAIR', 'dipole-agent', 'shadow', 'running', NOW(3)
);
INSERT INTO agent_task_timeline_repairs (
  event_uuid, task_uuid, run_uuid, event_kind, status, occurred_at, next_retry_at
) VALUES (
  'EVENT-SMOKE-COMPOSE-REPAIR', 'TASK-SMOKE-COMPOSE-REPAIR', 'RUN-SMOKE-COMPOSE-REPAIR',
  'model_call', 'completed', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)
);
SQL

pending_state=$(compose exec -T mysql mysql -N -uroot -proot123 dipole \
  -e "SELECT repair_status, COUNT(*) FROM agent_task_timeline_repairs WHERE event_uuid = 'EVENT-SMOKE-COMPOSE-REPAIR' GROUP BY repair_status;")
if [[ "${pending_state}" != $'pending\t1' ]]; then
  printf 'Compose repair persistence preflight failed: state=%q (want pending/1 before worker startup)\n' "${pending_state}" >&2
  exit 1
fi

compose up -d --wait agent-timeline-repair
compose exec -T agent-timeline-repair wget -q -O - http://127.0.0.1:9100/readyz | grep -qx ready

for _ in $(seq 1 30); do
  repair_state=$(compose exec -T mysql mysql -N -uroot -proot123 dipole \
    -e "SELECT repair_status, COUNT(*) FROM agent_task_timeline_repairs WHERE event_uuid = 'EVENT-SMOKE-COMPOSE-REPAIR' GROUP BY repair_status;")
  if [[ "${repair_state}" == $'completed\t1' ]]; then
    break
  fi
  sleep 1
done

event_count=$(compose exec -T mysql mysql -N -uroot -proot123 dipole \
  -e "SELECT COUNT(*) FROM agent_task_timeline_events WHERE event_uuid = 'EVENT-SMOKE-COMPOSE-REPAIR';")
if [[ "${repair_state}" != $'completed\t1' || "${event_count}" != "1" ]]; then
  printf 'Compose repair did not converge: state=%q events=%q\n' "${repair_state}" "${event_count}" >&2
  compose logs agent-timeline-repair >&2 || true
  exit 1
fi

printf 'Agent Timeline repair Compose smoke passed: persisted pending intent survived opt-in startup and replayed idempotently.\n'
