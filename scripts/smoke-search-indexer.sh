#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
kafka_compose="$root_dir/docker-compose.cluster.yml"
storage_compose="$root_dir/docker-compose.storage-lab.yml"
project="dipole-search-indexer-${RANDOM}-$$"
binary=$(mktemp /tmp/dipole-search-indexer.XXXXXX)
runtime_log=$(mktemp /tmp/dipole-search-indexer.XXXXXX.log)
runtime_container="${project}-runtime"

cleanup() {
  local exit_code=$?
  docker rm -f "$runtime_container" >/dev/null 2>&1 || true
  rm -f "$binary" "$runtime_log"
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    docker compose -p "$project" -f "$kafka_compose" -f "$storage_compose" down --volumes --remove-orphans >/dev/null 2>&1 || true
  fi
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

compose() { docker compose -p "$project" -f "$kafka_compose" -f "$storage_compose" "$@"; }

printf 'Starting isolated Kafka and Elasticsearch Search Indexer stack: project=%s\n' "$project"
compose up -d --wait kafka-1 kafka-2 kafka-3 elasticsearch
compose exec -T elasticsearch curl -fsS -X PUT http://127.0.0.1:9200/_cluster/settings \
  -H 'Content-Type: application/json' \
  -d '{"transient":{"cluster.routing.allocation.disk.threshold_enabled":false}}' >/dev/null

(cd "$root_dir" && CGO_ENABLED=0 go build -o "$binary" ./cmd/services/search-indexer)
docker run -d --name "$runtime_container" --network "${project}_default" \
  -v "$binary:/app/dipole-search-indexer:ro" \
  -v "$root_dir/deploy/elasticsearch/search-indexer-smoke.yaml:/app/configs/config.yaml:ro" \
  -w /app alpine:3.22 /app/dipole-search-indexer >/dev/null

for _ in $(seq 1 60); do
  docker logs "$runtime_container" >"$runtime_log" 2>&1 || true
  grep -q 'Search Indexer started' "$runtime_log" && break
  docker inspect -f '{{.State.Running}}' "$runtime_container" 2>/dev/null | grep -q true || { cat "$runtime_log"; exit 1; }
  sleep 1
done
grep -q 'Search Indexer started' "$runtime_log" || { cat "$runtime_log"; exit 1; }

assignment=""
for _ in $(seq 1 60); do
  assignment=$(compose exec -T kafka-1 /opt/kafka/bin/kafka-consumer-groups.sh \
    --bootstrap-server kafka-1:9092 --group dipole-search-indexer-consumer --describe 2>/dev/null || true)
  grep -q 'dipole.message.direct.created' <<<"$assignment" && break
  sleep 1
done
grep -q 'dipole.message.direct.created' <<<"$assignment" || {
  printf 'Search Indexer consumer group did not receive a partition assignment\n' >&2
  cat "$runtime_log"
  exit 1
}

publish() {
  local topic=$1 event=$2
  printf 'M-SEARCH-SMOKE|%s\n' "$event" | compose exec -T kafka-1 \
    /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server kafka-1:9092 \
    --topic "dipole.${topic}" --property parse.key=true --property 'key.separator=|' >/dev/null
}

source_field() {
  local field=$1
  compose exec -T elasticsearch curl -fsS "http://127.0.0.1:9200/dipole-smoke-messages-write/_doc/M-SEARCH-SMOKE" 2>/dev/null | \
    jq -r --arg field "$field" 'if ._source[$field] == null then "" else (._source[$field] | tostring) end' || true
}

wait_field() {
  local field=$1 expected=$2
  for _ in $(seq 1 60); do
    [[ "$(source_field "$field")" == "$expected" ]] && return 0
    sleep 1
  done
  printf 'Search projection did not reach %s=%s\n' "$field" "$expected" >&2
  cat "$runtime_log"
  return 1
}

created='{"event_id":"E-SEARCH-1","event_type":"message.direct.created","version":"v1","source":"dipole-smoke","occurred_at":"2026-08-27T00:00:00Z","payload":{"mutation_type":"created","revision":1,"actor_uuid":"U1","message_id":"M-SEARCH-SMOKE","conversation_key":"direct:U1:U2","message_seq":1,"sender_uuid":"U1","target_uuid":"U2","target_type":0,"message_type":0,"content":"searchable created","sent_at":"2026-08-27T00:00:00Z"}}'
recalled='{"event_id":"E-SEARCH-3","event_type":"message.direct.recalled","version":"v1","source":"dipole-smoke","occurred_at":"2026-08-27T00:00:02Z","payload":{"mutation_type":"recalled","revision":3,"actor_uuid":"U1","message_id":"M-SEARCH-SMOKE","target_type":0}}'
stale='{"event_id":"E-SEARCH-2","event_type":"message.direct.edited","version":"v1","source":"dipole-smoke","occurred_at":"2026-08-27T00:00:01Z","payload":{"mutation_type":"edited","revision":2,"actor_uuid":"U1","message_id":"M-SEARCH-SMOKE","conversation_key":"direct:U1:U2","message_seq":1,"sender_uuid":"U1","target_uuid":"U2","target_type":0,"message_type":0,"content":"stale edit","sent_at":"2026-08-27T00:00:01Z"}}'

publish message.direct.created "$created"
wait_field revision 1
publish message.direct.recalled "$recalled"
wait_field searchable false
publish message.direct.edited "$stale"
sleep 2
[[ "$(source_field revision)" == "3" && "$(source_field searchable)" == "false" ]] || {
  printf 'Stale edit resurrected a recalled Search document\n' >&2
  exit 1
}

printf 'Search Indexer smoke passed: created, tombstone and stale edit converged to revision 3 searchable=false.\n'
