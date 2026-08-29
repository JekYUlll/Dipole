#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
storage_compose="$root_dir/deploy/compose/docker-compose.storage-lab.yml"
project="dipole-search-service-${RANDOM}-$$"
test_binary=$(mktemp /tmp/dipole-search-service-test.XXXXXX)

cleanup() {
  local exit_code=$?
  rm -f "$test_binary"
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    docker compose -p "$project" -f "$storage_compose" down --volumes --remove-orphans >/dev/null 2>&1 || true
  else
    printf 'Search Service stack retained: project=%s\n' "$project"
  fi
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

compose() { docker compose -p "$project" -f "$storage_compose" "$@"; }

printf 'Starting isolated Elasticsearch Search Service contract: project=%s\n' "$project"
compose up -d --wait elasticsearch
compose exec -T elasticsearch curl -fsS -X PUT http://127.0.0.1:9200/_cluster/settings \
  -H 'Content-Type: application/json' \
  -d '{"transient":{"cluster.routing.allocation.disk.threshold_enabled":false}}' >/dev/null

(
  cd "$root_dir"
  CGO_ENABLED=0 go test -c -o "$test_binary" ./internal/bootstrap
)
docker run --rm \
  --network "${project}_default" \
  -e DIPOLE_TEST_ELASTICSEARCH_URL=http://elasticsearch:9200 \
  -v "$test_binary:/app/search-service-test:ro" \
  -w /app alpine:3.22 \
  /app/search-service-test -test.run '^TestSearchRuntimeElasticsearchContract$' -test.v

printf 'Search Service contract passed: Core-derived scope, internal RPC, and Elasticsearch 9.5.2 query path.\n'
