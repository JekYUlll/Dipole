#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
project="dipole-sync-hydration-${RANDOM}-$$"
mysql_container="${project}-mysql"
migrate_binary=$(mktemp /tmp/dipole-sync-hydration-migrate.XXXXXX)

cleanup() {
  local exit_code=$?
  docker rm -f "$mysql_container" >/dev/null 2>&1 || true
  rm -f "$migrate_binary"
  docker compose -p "$project" -f "$root_dir/docker-compose.storage-lab.yml" down --volumes --remove-orphans >/dev/null 2>&1 || true
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

docker compose -p "$project" -f "$root_dir/docker-compose.storage-lab.yml" up -d --wait cassandra
docker compose -p "$project" -f "$root_dir/docker-compose.storage-lab.yml" exec -T cassandra cqlsh <"$root_dir/db/cassandra/001_timeline.cql"
docker run -d --name "$mysql_container" --network "${project}_default" --network-alias mysql -p 127.0.0.1::3306 \
  --health-cmd='mysqladmin ping -h 127.0.0.1 -uroot -pdipole-root --silent' --health-interval=2s --health-timeout=2s --health-retries=60 --health-start-period=10s \
  -e MYSQL_ROOT_PASSWORD=dipole-root -e MYSQL_DATABASE=dipole mysql:8.4.8 >/dev/null
for _ in $(seq 1 90); do
  if [[ "$(docker inspect -f '{{.State.Health.Status}}' "$mysql_container")" == "healthy" ]]; then break; fi
  sleep 1
done
test "$(docker inspect -f '{{.State.Health.Status}}' "$mysql_container")" = "healthy"
mysql_port=$(docker port "$mysql_container" 3306/tcp | awk -F: 'NR==1 {print $NF}')

(cd "$root_dir" && CGO_ENABLED=0 go build -o "$migrate_binary" ./cmd/migrate)
docker run --rm --network "${project}_default" -v "$root_dir/deploy/cassandra/backfill-smoke.yaml:/app/configs/config.yaml:ro" -v "$migrate_binary:/app/dipole-migrate:ro" -w /app alpine:3.22 /app/dipole-migrate -direction up >/dev/null
(
  cd "$root_dir"
  DIPOLE_TEST_MYSQL_DSN="root:dipole-root@tcp(127.0.0.1:${mysql_port})/dipole?parseTime=true&loc=UTC" \
    DIPOLE_TEST_CASSANDRA_HOSTS=127.0.0.1:19042 LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu \
    go test -count=1 -run '^TestSyncCassandraHydrationShadowContract$' ./internal/data/shadow
  DIPOLE_TEST_CASSANDRA_HOSTS=127.0.0.1:19042 LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu \
    go test -count=1 -run '^TestMessageDuplicateHydrationWithRealCassandra$' ./internal/service
  DIPOLE_TEST_MYSQL_ADMIN_DSN="root:dipole-root@tcp(127.0.0.1:${mysql_port})/?parseTime=true&loc=UTC" \
    LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu \
    go test -count=1 -run '^TestMessageMetadataMigrationBackfillsExistingMessages$' ./internal/data/migration
)

printf 'Sync Cassandra hydration smoke passed: shadow comparison, duplicate response recovery, legacy ID restoration, and Metadata migration backfill verified.\n'
