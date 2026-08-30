#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
container="dipole-sync-write-ownership-${RANDOM}-$$"
network="${container}-network"
migrate_binary=$(mktemp /tmp/dipole-sync-write-ownership-migrate.XXXXXX)
report_file=${SMOKE_REPORT_FILE:-/tmp/${container}-receipt.json}
started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
source_revision=$(git -C "$root_dir" rev-parse HEAD)
source_dirty=0
if ! git -C "$root_dir" diff --quiet || ! git -C "$root_dir" diff --cached --quiet; then
  source_dirty=1
fi

write_receipt() {
  local exit_code="$1"
  local status=failed
  local finished_at
  local remaining_containers
  finished_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  [[ "$exit_code" -eq 0 ]] && status=passed
  remaining_containers=$(docker ps --format '{{.Names}}' | grep -E 'dipole-sync-write-ownership|multipart' | wc -l | tr -d ' ')
  mkdir -p "$(dirname "$report_file")"
  local temporary_file
  temporary_file=$(mktemp "${report_file}.tmp.XXXXXX")
  jq -n \
    --arg schema "dipole.sync.write-ownership-smoke-receipt.v1" \
    --arg revision "$source_revision" \
    --arg started_at "$started_at" \
    --arg finished_at "$finished_at" \
    --arg status "$status" \
    --argjson exit_code "$exit_code" \
    --argjson dirty "$source_dirty" \
    --argjson remaining_containers "$remaining_containers" \
    '{schema_version:$schema,source:{revision:$revision,dirty:($dirty == 1)},ownership:{projector_write:true,atomic_rollback:true},rollback:{action:"set DIPOLE_MESSAGE_INBOX_WRITE_MODE=atomic and disable Sync projector",destructive_data_migration:false},result:{status:$status,exit_code:$exit_code,started_at:$started_at,finished_at:$finished_at,remaining_isolated_containers:$remaining_containers}}' \
    >"$temporary_file"
  chmod 0600 "$temporary_file"
  mv -f "$temporary_file" "$report_file"
}

cleanup() {
  local exit_code=$?
  docker rm -f "$container" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -f "$migrate_binary"
  write_receipt "$exit_code" || true
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

command -v jq >/dev/null || {
  echo 'jq is required to write the ownership smoke receipt' >&2
  exit 2
}

docker network create "$network" >/dev/null
docker run -d --name "$container" --network "$network" --network-alias mysql -p 127.0.0.1::3306 \
  -e MYSQL_ROOT_PASSWORD=dipole-root -e MYSQL_ROOT_HOST=% -e MYSQL_DATABASE=dipole \
  mysql:8.4 >/dev/null
for _ in $(seq 1 90); do
  docker exec "$container" mysqladmin ping -h 127.0.0.1 -uroot -pdipole-root --silent >/dev/null 2>&1 && break
  sleep 1
done
docker exec "$container" mysqladmin ping -h 127.0.0.1 -uroot -pdipole-root --silent >/dev/null
port=$(docker port "$container" 3306/tcp | sed 's/.*://')
admin_dsn="root:dipole-root@tcp(127.0.0.1:${port})/?parseTime=true&multiStatements=true"

(
  cd "$root_dir"
  CGO_ENABLED=0 go build -o "$migrate_binary" ./cmd/tools/migrate
)
docker run --rm --network "$network" \
  -v "$root_dir/deploy/mysql/sync-recovery-smoke.yaml:/app/configs/config.yaml:ro" \
  -v "$migrate_binary:/app/dipole-migrate:ro" -w /app alpine:3.22 \
  /app/dipole-migrate -direction up >/dev/null
mapfile -t migration_checks < <(docker exec "$container" mysql -N -B -uroot -pdipole-root dipole \
  -e "SELECT COUNT(*) FROM schema_migrations WHERE version = 12; SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='dipole' AND table_name='messages'; SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='dipole' AND table_name='message_metadata';")
test "${#migration_checks[@]}" -eq 3
test "${migration_checks[0]}" = "1"
test "${migration_checks[1]}" = "1"
test "${migration_checks[2]}" = "1"
docker exec -i "$container" mysql -uroot -pdipole-root <"$root_dir/configs/mysql/sync-service-grants.dist.sql"
docker exec -i "$container" mysql -uroot -pdipole-root <"$root_dir/configs/mysql/message-service-atomic-grants.dist.sql"
docker exec -i "$container" mysql -uroot -pdipole-root <"$root_dir/configs/mysql/message-service-projector-grants.dist.sql"

run_go_test() {
  local output
  if ! output=$("$@" 2>&1); then
    printf '%s\n' "$output"
    return 1
  fi
  printf '%s\n' "$output"
  if grep -Fq '[no tests to run]' <<<"$output"; then
    echo "smoke test selector matched no tests: $*" >&2
    return 1
  fi
}

(
  cd "$root_dir"
  run_go_test env DIPOLE_TEST_SYNC_MYSQL_DSN="dipole_sync:change-me@tcp(127.0.0.1:${port})/dipole?parseTime=true" \
    LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu go test ./internal/services/sync/bootstrap -run '^TestSyncDatabaseBoundaryWithMySQLAccount$' -count=1
  run_go_test env DIPOLE_TEST_MESSAGE_PROJECTOR_MYSQL_DSN="dipole_message_projector:change-me@tcp(127.0.0.1:${port})/dipole?parseTime=true" \
    LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu go test ./internal/services/message/infrastructure/mysql -run '^TestMessageProjectorDatabaseBoundaryWithMySQLAccount$' -count=1
  run_go_test env DIPOLE_TEST_MESSAGE_ATOMIC_MYSQL_DSN="dipole_message:change-me@tcp(127.0.0.1:${port})/dipole?parseTime=true" \
    LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu go test ./internal/services/message/infrastructure/mysql -run '^TestMessageAtomicDatabaseBoundaryWithMySQLAccount$' -count=1
  run_go_test env DIPOLE_TEST_MESSAGE_PROJECTOR_MYSQL_DSN="dipole_message_projector:change-me@tcp(127.0.0.1:${port})/dipole?parseTime=true" \
    LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu go test ./internal/services/message/infrastructure/mysql -run '^TestMessageProjectorAccountWritesMessageAndOutbox$' -count=1
  run_go_test env DIPOLE_TEST_MESSAGE_ATOMIC_MYSQL_DSN="dipole_message:change-me@tcp(127.0.0.1:${port})/dipole?parseTime=true" \
    LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu go test ./internal/services/message/infrastructure/mysql -run '^TestMessageAtomicAccountWritesMessageOutboxAndInbox$' -count=1
  run_go_test env DIPOLE_TEST_MYSQL_ADMIN_DSN="$admin_dsn" LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu \
    go test ./internal/services/message/infrastructure/mysql -run '^TestMessageInboxWriteOwnershipCanMoveToProjectorAndRollBack$' -count=1
)

test "$(docker exec "$container" mysql -N -B -uroot -pdipole-root dipole -e "SELECT COUNT(*) FROM messages WHERE uuid='M-projector-smoke';")" = "1"
test "$(docker exec "$container" mysql -N -B -uroot -pdipole-root dipole -e "SELECT COUNT(*) FROM message_metadata WHERE message_uuid='M-projector-smoke' AND CHAR_LENGTH(payload_sha256)=64;")" = "1"
test "$(docker exec "$container" mysql -N -B -uroot -pdipole-root dipole -e "SELECT COUNT(*) FROM outbox_events WHERE aggregate_id='M-projector-smoke';")" = "1"
test "$(docker exec "$container" mysql -N -B -uroot -pdipole-root dipole -e "SELECT COUNT(*) FROM user_sync_inbox WHERE message_uuid='M-projector-smoke';")" = "0"
test "$(docker exec "$container" mysql -N -B -uroot -pdipole-root dipole -e "SELECT COUNT(*) FROM user_sync_inbox WHERE message_uuid='M-atomic-smoke';")" = "1"

printf 'Sync and Message atomic/projector least-privilege ownership smoke passed.\n'
