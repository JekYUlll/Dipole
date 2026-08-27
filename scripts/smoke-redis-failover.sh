#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
compose_file="$root_dir/docker-compose.redis-cluster.yml"
project="dipole-redis-cluster-${RANDOM}-$$"
network="${project}_default"
probe_binary=$(mktemp /tmp/dipole-redis-failover-probe.XXXXXX)
probe_log=$(mktemp /tmp/dipole-redis-failover.XXXXXX.log)
probe_container="${project}-probe"
probe_pid=""

cleanup() {
  local exit_code=$?
  if [[ -n "$probe_pid" ]]; then
    wait "$probe_pid" >/dev/null 2>&1 || true
  fi
  docker rm -f "$probe_container" >/dev/null 2>&1 || true
  rm -f "$probe_binary" "$probe_log"
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    docker compose -p "$project" -f "$compose_file" down --volumes --remove-orphans >/dev/null 2>&1 || true
  else
    printf 'Redis failover stack retained: project=%s\n' "$project"
  fi
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

compose() {
  docker compose -p "$project" -f "$compose_file" "$@"
}

service_for_primary() {
  local primary_host=$1
  local service container_id container_ip
  for service in redis-1 redis-2 redis-3; do
    if [[ "$primary_host" == "$service" ]]; then
      printf '%s\n' "$service"
      return 0
    fi
    container_id=$(compose ps -q "$service")
    container_ip=$(docker inspect -f "{{with index .NetworkSettings.Networks \"$network\"}}{{.IPAddress}}{{end}}" "$container_id")
    if [[ "$primary_host" == "$container_ip" ]]; then
      printf '%s\n' "$service"
      return 0
    fi
  done
  return 1
}

wait_for_probe_line() {
  local prefix=$1
  for _ in $(seq 1 90); do
    if grep -q "^${prefix}" "$probe_log"; then
      grep "^${prefix}" "$probe_log" | tail -1
      return 0
    fi
    if [[ -n "$probe_pid" ]] && ! kill -0 "$probe_pid" >/dev/null 2>&1; then
      cat "$probe_log"
      return 1
    fi
    sleep 1
  done
  cat "$probe_log"
  return 1
}

printf 'Starting isolated Redis Sentinel stack: project=%s\n' "$project"
compose up -d --wait

printf 'Building static Redis failover integration probe\n'
(
  cd "$root_dir"
  CGO_ENABLED=0 go test -c ./internal/store -o "$probe_binary"
)

docker run --rm --name "$probe_container" \
  --network "$network" \
  -v "$probe_binary:/probe:ro" \
  -v "$root_dir/configs/config.dist.yaml:/app/configs/config.yaml:ro" \
  -w /app \
  -e DIPOLE_TEST_REDIS_SENTINELS=sentinel-1:26379,sentinel-2:26379,sentinel-3:26379 \
  -e DIPOLE_PRESENCE_NODE_ID=gateway-failover-probe \
  -e DIPOLE_RATE_LIMIT_ENABLED=true \
  -e DIPOLE_RATE_LIMIT_LOGIN_LIMIT=1 \
  alpine:3.21 \
  /probe -test.run '^TestRedisSentinelFailoverPreservesRealtimeSemantics$' -test.v >"$probe_log" 2>&1 &
probe_pid=$!

ready_line=$(wait_for_probe_line 'REDIS_FAILOVER_PRIMARY_READY=')
original_primary=${ready_line#*=}
original_host=${original_primary%:*}
original_service=$(service_for_primary "$original_host") || {
  printf 'Could not map Sentinel primary %s to a Compose service\n' "$original_primary" >&2
  cat "$probe_log"
  exit 1
}

printf 'Stopping Redis primary: service=%s address=%s\n' "$original_service" "$original_primary"
compose stop "$original_service"

set +e
wait "$probe_pid"
probe_status=$?
set -e
probe_pid=""
cat "$probe_log"
if [[ "$probe_status" -ne 0 ]]; then
  exit "$probe_status"
fi
grep -q '^REDIS_FAILOVER_OK=' "$probe_log"

printf 'Restarting former primary and waiting for replica rejoin\n'
compose start "$original_service"
for _ in $(seq 1 60); do
  role=$(compose exec -T "$original_service" redis-cli role 2>/dev/null | head -1 | tr -d '\r' || true)
  if [[ "$role" == "slave" ]]; then
    printf 'Redis Sentinel failover smoke passed: client, Pub/Sub, Presence, hot-group, and rate-limit semantics recovered; former primary rejoined as replica.\n'
    exit 0
  fi
  sleep 1
done

printf 'Former primary did not rejoin as a replica\n' >&2
compose logs "$original_service" sentinel-1 sentinel-2 sentinel-3
exit 1
