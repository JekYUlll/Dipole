#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
container="dipole-sync-write-ownership-${RANDOM}-$$"
network="${container}-network"
migrate_binary=$(mktemp /tmp/dipole-sync-write-ownership-migrate.XXXXXX)

cleanup() {
  local exit_code=$?
  docker rm -f "$container" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -f "$migrate_binary"
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

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
    LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu go test ./internal/bootstrap -run '^TestSyncDatabaseBoundaryWithMySQLAccount$' -count=1
  run_go_test env DIPOLE_TEST_MESSAGE_PROJECTOR_MYSQL_DSN="dipole_message_projector:change-me@tcp(127.0.0.1:${port})/dipole?parseTime=true" \
    LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu go test ./internal/bootstrap -run '^TestMessageProjectorDatabaseBoundaryWithMySQLAccount$' -count=1
  run_go_test env DIPOLE_TEST_MESSAGE_ATOMIC_MYSQL_DSN="dipole_message:change-me@tcp(127.0.0.1:${port})/dipole?parseTime=true" \
    LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu go test ./internal/bootstrap -run '^TestMessageAtomicDatabaseBoundaryWithMySQLAccount$' -count=1
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
