#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
kafka_compose="$root_dir/deploy/compose/docker-compose.cluster.yml"
storage_compose="$root_dir/deploy/compose/docker-compose.storage-lab.yml"
project="dipole-cassandra-projector-${RANDOM}-$$"
projector_binary=$(mktemp /tmp/dipole-cassandra-projector.XXXXXX)
projector_log=$(mktemp /tmp/dipole-cassandra-projector.XXXXXX.log)
projector_container="${project}-runtime"

cleanup() {
  local exit_code=$?
  docker rm -f "$projector_container" >/dev/null 2>&1 || true
  rm -f "$projector_binary" "$projector_log"
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    docker compose -p "$project" -f "$kafka_compose" -f "$storage_compose" down --volumes --remove-orphans >/dev/null 2>&1 || true
  else
    printf 'Cassandra projector stack retained: project=%s\n' "$project"
  fi
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

compose() {
  docker compose -p "$project" -f "$kafka_compose" -f "$storage_compose" "$@"
}

printf 'Starting isolated Kafka and Cassandra projector stack: project=%s\n' "$project"
compose up -d --wait kafka-1 kafka-2 kafka-3 cassandra
compose exec -T cassandra cqlsh <"$root_dir/db/cassandra/001_timeline.cql"

(
  cd "$root_dir"
  CGO_ENABLED=0 go build -o "$projector_binary" ./cmd/tools/cassandra-projector
)

docker run -d --name "$projector_container" \
  --network "${project}_default" \
  -v "$projector_binary:/app/dipole-cassandra-projector:ro" \
  -v "$root_dir/deploy/cassandra/projector-smoke.yaml:/app/configs/config.yaml:ro" \
  -w /app \
  alpine:3.21 \
  /app/dipole-cassandra-projector >/dev/null

for _ in $(seq 1 60); do
  docker logs "$projector_container" >"$projector_log" 2>&1 || true
  if grep -q 'Cassandra projector started' "$projector_log"; then
    break
  fi
  if ! docker inspect -f '{{.State.Running}}' "$projector_container" 2>/dev/null | grep -q true; then
    cat "$projector_log"
    exit 1
  fi
  sleep 1
done
grep -q 'Cassandra projector started' "$projector_log" || {
  cat "$projector_log"
  exit 1
}

assignment=""
for _ in $(seq 1 60); do
  assignment=$(compose exec -T kafka-1 \
    /opt/kafka/bin/kafka-consumer-groups.sh \
    --bootstrap-server kafka-1:9092 \
    --group dipole-cassandra-projector-consumer \
    --describe 2>/dev/null || true)
  if grep -q 'dipole.message.direct.created' <<<"$assignment"; then
    break
  fi
  sleep 1
done
grep -q 'dipole.message.direct.created' <<<"$assignment" || {
  printf 'Cassandra projector consumer group did not receive a partition assignment\n' >&2
  cat "$projector_log"
  exit 1
}

event='{"event_id":"E-PROJECTOR-SMOKE","event_type":"message.direct.created","version":"v1","source":"dipole-smoke","occurred_at":"2026-08-27T00:00:00Z","payload":{"mutation_type":"created","revision":1,"actor_uuid":"U100","message_id":"M-PROJECTOR-SMOKE","client_message_id":"C-PROJECTOR-SMOKE","conversation_key":"direct:U100:U200","message_seq":10001,"sender_uuid":"U100","target_uuid":"U200","target_type":0,"message_type":0,"content":"confirmed Cassandra projection","sent_at":"2026-08-27T00:00:00Z"}}'
for _ in 1 2; do
  printf 'M-PROJECTOR-SMOKE|%s\n' "$event" | compose exec -T kafka-1 \
    /opt/kafka/bin/kafka-console-producer.sh \
    --bootstrap-server kafka-1:9092 \
    --topic dipole.message.direct.created \
    --property parse.key=true \
    --property 'key.separator=|' >/dev/null
done

for _ in $(seq 1 60); do
  row=$(compose exec -T cassandra cqlsh -e "SELECT message_uuid, message_seq, payload_hash FROM dipole_message_shadow.timeline_by_conversation_bucket WHERE conversation_key = 'direct:U100:U200' AND bucket = 1 AND message_seq = 10001;" 2>/dev/null || true)
  if grep -q 'M-PROJECTOR-SMOKE' <<<"$row"; then
    count=$(compose exec -T cassandra cqlsh -e "SELECT COUNT(*) FROM dipole_message_shadow.timeline_by_conversation_bucket WHERE conversation_key = 'direct:U100:U200' AND bucket = 1;" | awk '/^[[:space:]]*[0-9]+[[:space:]]*$/ {gsub(/[[:space:]]/, ""); print; exit}')
    if [[ "$count" != "1" ]]; then
      printf 'Expected one idempotent Cassandra projection row, got %s\n' "$count" >&2
      exit 1
    fi
    printf 'Cassandra projector smoke passed: independent Kafka consumer projected duplicate created events into one immutable Timeline row.\n'
    exit 0
  fi
  sleep 1
done

docker logs "$projector_container"
printf 'Cassandra projector did not materialize the expected Timeline row\n' >&2
exit 1
