#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
COMPOSE_FILE="$ROOT_DIR/deploy/compose/docker-compose.mysql-cluster.yml"
PROJECT_NAME="dipole-mysql-cluster-${RANDOM}"
PROBE_LOG=$(mktemp)

compose() {
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [[ "${KEEP_STACK:-0}" == "1" ]]; then
    echo "keeping Compose project $PROJECT_NAME" >&2
    return
  fi
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

if ! compose up -d --wait; then
  compose logs mysql-cluster-init >&2 || true
  exit 1
fi

export DIPOLE_MYSQL_HOST=127.0.0.1
export DIPOLE_MYSQL_PORT=16446
export DIPOLE_MYSQL_USER=dipole
export DIPOLE_MYSQL_PASSWORD=dipole123
export DIPOLE_MYSQL_DBNAME=dipole
LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu go run ./cmd/tools/migrate -direction up

export DIPOLE_TEST_MYSQL_FAILOVER_DSN='dipole:dipole123@tcp(127.0.0.1:16446)/dipole?parseTime=true&collation=utf8mb4_unicode_ci'
LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu go test -count=1 -run TestMySQLRouterWriterFailover -v ./internal/platform/mysql/config >"$PROBE_LOG" 2>&1 &
probe_pid=$!

for ((i = 1; i <= 60; i++)); do
  if grep -q 'MYSQL_HA_PRIMARY_READY=' "$PROBE_LOG"; then
    break
  fi
  if ! kill -0 "$probe_pid" 2>/dev/null; then
    cat "$PROBE_LOG" >&2
    wait "$probe_pid"
  fi
  sleep 1
done

primary_uuid=$(sed -n 's/.*MYSQL_HA_PRIMARY_READY=\([^[:space:]]*\).*/\1/p' "$PROBE_LOG" | tail -1)
if [[ -z "$primary_uuid" ]]; then
  cat "$PROBE_LOG" >&2
  echo "timed out waiting for failover probe readiness" >&2
  exit 1
fi

primary_host=$(compose exec -T mysql-1 mysql -uroot -proot123 -Nse \
  "SELECT MEMBER_HOST FROM performance_schema.replication_group_members WHERE MEMBER_ID='$primary_uuid'")
case "$primary_host" in
  mysql-1|mysql-2|mysql-3) ;;
  *) echo "unexpected primary host: $primary_host" >&2; exit 1 ;;
esac

compose stop "$primary_host" >/dev/null
wait "$probe_pid"
cat "$PROBE_LOG"

new_primary_uuid=$(sed -n 's/.*MYSQL_HA_FAILOVER_OK=\([^[:space:]]*\).*/\1/p' "$PROBE_LOG" | tail -1)
new_primary_host=$(compose exec -T mysql-router mysql -h127.0.0.1 -P6446 -uroot -proot123 -Nse \
  "SELECT MEMBER_HOST FROM performance_schema.replication_group_members WHERE MEMBER_ID='$new_primary_uuid'")
case "$new_primary_host" in
  mysql-1|mysql-2|mysql-3) ;;
  *) echo "unexpected replacement primary host: $new_primary_host" >&2; exit 1 ;;
esac

compose start "$primary_host" >/dev/null
for ((i = 1; i <= 30; i++)); do
  if compose exec -T "$primary_host" mysqladmin ping -h127.0.0.1 -proot123 >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
compose run --rm --no-deps \
  -e MYSQL_CLUSTER_PRIMARY="$new_primary_host" \
  -e MYSQL_REJOIN_HOST="$primary_host" \
  --entrypoint mysqlsh mysql-cluster-init \
  --no-defaults --js --file /rejoin-instance.js >/dev/null
for ((i = 1; i <= 60; i++)); do
  state=$(compose exec -T mysql-router mysql -h127.0.0.1 -P6446 -uroot -proot123 -Nse \
    "SELECT MEMBER_STATE FROM performance_schema.replication_group_members WHERE MEMBER_HOST='$primary_host'" 2>/dev/null || true)
  if [[ "$state" == "ONLINE" ]]; then
    echo "MySQL cluster smoke passed: Router writer failover preserved committed data and AdminAPI rejoined the stopped member."
    exit 0
  fi
  sleep 2
done

echo "restarted MySQL member did not rejoin" >&2
exit 1
