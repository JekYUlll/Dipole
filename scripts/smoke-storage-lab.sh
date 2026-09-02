#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
compose_file="$root_dir/deploy/compose/docker-compose.storage-lab.yml"
project="${COMPOSE_PROJECT_NAME:-dipole-storage-lab-${RANDOM}-$$}"

cleanup() {
  local exit_code=$?
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    docker compose -p "$project" -f "$compose_file" down --volumes --remove-orphans >/dev/null 2>&1 || true
  else
    printf 'Storage lab retained: project=%s\n' "$project"
  fi
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

compose() {
  docker compose -p "$project" -f "$compose_file" "$@"
}

if ! sed -n '/^cassandra:/,/^[^ ]/p' "$root_dir/configs/config.dist.yaml" | rg -q '^  enabled: false$'; then
  printf 'Cassandra must remain disabled in the default application configuration\n' >&2
  exit 1
fi
if ! sed -n '/^message:/,/^[^ ]/p' "$root_dir/configs/config.dist.yaml" | rg -q '^  cassandra_shadow_reads: false$'; then
  printf 'Cassandra Message shadow reads must remain disabled by default\n' >&2
  exit 1
fi
if rg -i 'cassandra' \
	"$root_dir/internal/bootstrap/runtime.go" \
	"$root_dir/internal/bootstrap/gateway_runtime.go"; then
	printf 'Cassandra must stay outside Core and Gateway composition roots\n' >&2
	exit 1
fi
if rg -i 'elasticsearch' \
  "$root_dir/internal/bootstrap/runtime.go" \
  "$root_dir/internal/bootstrap/message_runtime.go" \
  "$root_dir/internal/bootstrap/gateway_runtime.go"; then
  printf 'Elasticsearch must stay outside Core, Message, and Gateway composition roots\n' >&2
  exit 1
fi

printf 'Starting isolated Cassandra and Elasticsearch lab: project=%s\n' "$project"
compose up -d --wait

cassandra_version=$(compose exec -T cassandra cqlsh -e 'SELECT release_version FROM system.local;' | awk '/^[[:space:]]*[0-9]+\./ {gsub(/[[:space:]]/, ""); print; exit}')
if [[ -z "$cassandra_version" ]]; then
  printf 'Could not read Cassandra release version\n' >&2
  exit 1
fi

cassandra_result=$(compose exec -T cassandra cqlsh <<'CQL'
CREATE KEYSPACE IF NOT EXISTS dipole_lab_smoke
WITH replication = {'class': 'NetworkTopologyStrategy', 'datacenter1': 1};
CREATE TABLE IF NOT EXISTS dipole_lab_smoke.timeline_probe (
  conversation_key text,
  message_seq bigint,
  content text,
  PRIMARY KEY (conversation_key, message_seq)
) WITH CLUSTERING ORDER BY (message_seq DESC);
INSERT INTO dipole_lab_smoke.timeline_probe (conversation_key, message_seq, content)
VALUES ('direct:smoke', 1, 'confirmed');
SELECT content FROM dipole_lab_smoke.timeline_probe
WHERE conversation_key = 'direct:smoke' AND message_seq = 1;
CQL
)
if ! grep -q 'confirmed' <<<"$cassandra_result"; then
  printf 'Cassandra smoke row was not readable\n%s\n' "$cassandra_result" >&2
  exit 1
fi
compose exec -T cassandra cqlsh -e 'DROP KEYSPACE dipole_lab_smoke;'

elastic_version=""
for _ in $(seq 1 "${ELASTIC_API_RETRY_ATTEMPTS:-30}"); do
  if elastic_version=$(compose exec -T elasticsearch curl -fsS http://127.0.0.1:9200 | jq -r '.version.number' 2>/dev/null) && [[ -n "$elastic_version" && "$elastic_version" != "null" ]]; then
    break
  fi
  sleep "${ELASTIC_API_RETRY_INTERVAL_SECONDS:-1}"
done
if [[ -z "$elastic_version" || "$elastic_version" == "null" ]]; then
  printf 'Elasticsearch API did not become available after bounded retries\n' >&2
  exit 1
fi
compose exec -T elasticsearch curl -fsS -X PUT http://127.0.0.1:9200/dipole-search-smoke \
  -H 'Content-Type: application/json' \
  -d '{"mappings":{"dynamic":"strict","properties":{"conversation_key":{"type":"keyword"},"message_seq":{"type":"long"},"content":{"type":"text"}}}}' >/dev/null
compose exec -T elasticsearch curl -fsS -X PUT 'http://127.0.0.1:9200/dipole-search-smoke/_doc/M-SMOKE?refresh=wait_for' \
  -H 'Content-Type: application/json' \
  -d '{"conversation_key":"direct:smoke","message_seq":1,"content":"confirmed search projection"}' >/dev/null
elastic_hits=$(compose exec -T elasticsearch curl -fsS \
  -H 'Content-Type: application/json' \
  -d '{"query":{"match":{"content":"confirmed"}}}' \
  http://127.0.0.1:9200/dipole-search-smoke/_search | jq -r '.hits.total.value')
if [[ "$elastic_hits" != "1" ]]; then
  printf 'Expected one Elasticsearch smoke hit, got %s\n' "$elastic_hits" >&2
  exit 1
fi
compose exec -T elasticsearch curl -fsS -X DELETE http://127.0.0.1:9200/dipole-search-smoke >/dev/null

printf 'Storage lab smoke passed: Cassandra %s and Elasticsearch %s completed isolated CRUD with zero production traffic.\n' \
  "$cassandra_version" "$elastic_version"
