#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
kafka_compose="$root_dir/deploy/compose/docker-compose.cluster.yml"
project="dipole-sync-projector-${RANDOM}-$$"
test_binary=$(mktemp /tmp/dipole-sync-projector-test.XXXXXX)
mysql_container="${project}-mysql"

cleanup() {
  local exit_code=$?
  docker rm -f "$mysql_container" >/dev/null 2>&1 || true
  rm -f "$test_binary"
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    docker compose -p "$project" -f "$kafka_compose" down --volumes --remove-orphans >/dev/null 2>&1 || true
  else
    printf 'Sync Projector stack retained: project=%s\n' "$project"
  fi
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

compose() { docker compose -p "$project" -f "$kafka_compose" "$@"; }

printf 'Starting isolated Kafka and MySQL Sync Projector stack: project=%s\n' "$project"
compose up -d --wait kafka-1 kafka-2 kafka-3
docker run -d --name "$mysql_container" --network "${project}_default" \
  -e MYSQL_ROOT_PASSWORD=dipole-root \
  -e MYSQL_DATABASE=dipole \
  --health-cmd='mysqladmin ping -h 127.0.0.1 -pdipole-root --silent' \
  --health-interval=2s --health-timeout=2s --health-retries=40 \
  mysql:8.4 \
  --character-set-server=utf8mb4 --collation-server=utf8mb4_unicode_ci >/dev/null

for _ in $(seq 1 60); do
  [[ "$(docker inspect -f '{{.State.Health.Status}}' "$mysql_container" 2>/dev/null || true)" == "healthy" ]] && break
  sleep 1
done
[[ "$(docker inspect -f '{{.State.Health.Status}}' "$mysql_container")" == "healthy" ]] || {
  docker logs "$mysql_container"
  exit 1
}

(
  cd "$root_dir"
  CGO_ENABLED=0 go test -c -o "$test_binary" ./internal/services/sync/infrastructure/kafka
)
docker run --rm --network "${project}_default" \
  -e "DIPOLE_TEST_SYNC_PROJECTOR_DSN=root:dipole-root@tcp(${mysql_container}:3306)/dipole?parseTime=true&multiStatements=true" \
  -e 'DIPOLE_TEST_SYNC_PROJECTOR_BROKERS=kafka-1:9092,kafka-2:9092,kafka-3:9092' \
  -v "$test_binary:/app/sync-projector-test:ro" \
  -w /app alpine:3.22 \
  /app/sync-projector-test -test.run '^TestKafkaMySQLDualRunIntegration$' -test.v

printf 'Sync Projector smoke passed: earliest backlog and live events converged; retry/DLQ were observable and hot-group fanout stayed disabled.\n'
