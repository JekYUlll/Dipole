#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
storage_compose="$root_dir/docker-compose.storage-lab.yml"
project="dipole-search-backfill-${RANDOM}-$$"
mysql_container="${project}-mysql"
migrate_binary=$(mktemp /tmp/dipole-search-backfill-migrate.XXXXXX)
backfill_binary=$(mktemp /tmp/dipole-search-backfill.XXXXXX)
reconcile_binary=$(mktemp /tmp/dipole-search-reconcile.XXXXXX)
reconcile_report=$(mktemp /tmp/dipole-search-reconcile.XXXXXX.json)
target_index="dipole-smoke-messages-v1-build-a"

cleanup() {
  local exit_code=$?
  docker rm -f "$mysql_container" >/dev/null 2>&1 || true
  rm -f "$migrate_binary" "$backfill_binary" "$reconcile_binary" "$reconcile_report"
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    docker compose -p "$project" -f "$storage_compose" down --volumes --remove-orphans >/dev/null 2>&1 || true
  else
    printf 'Search backfill stack retained: project=%s mysql=%s\n' "$project" "$mysql_container"
  fi
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

compose() { docker compose -p "$project" -f "$storage_compose" "$@"; }

printf 'Starting isolated MySQL and Elasticsearch Search backfill stack: project=%s\n' "$project"
compose up -d --wait elasticsearch
compose exec -T elasticsearch curl -fsS -X PUT http://127.0.0.1:9200/_cluster/settings \
  -H 'Content-Type: application/json' \
  -d '{"transient":{"cluster.routing.allocation.disk.threshold_enabled":false}}' >/dev/null

docker run -d --name "$mysql_container" \
  --network "${project}_default" --network-alias mysql \
  -e MYSQL_ROOT_PASSWORD=dipole-root -e MYSQL_ROOT_HOST=% -e MYSQL_DATABASE=dipole \
  --health-cmd='mysqladmin ping -h 127.0.0.1 -uroot -pdipole-root --silent' \
  --health-interval=2s --health-timeout=2s --health-retries=60 --health-start-period=20s \
  mysql:8.4.8 >/dev/null
for _ in $(seq 1 90); do
  [[ "$(docker inspect -f '{{.State.Health.Status}}' "$mysql_container" 2>/dev/null || true)" == healthy ]] && break
  sleep 1
done
[[ "$(docker inspect -f '{{.State.Health.Status}}' "$mysql_container")" == healthy ]]

(
  cd "$root_dir"
  CGO_ENABLED=0 go build -o "$migrate_binary" ./cmd/migrate
  CGO_ENABLED=0 go build -o "$backfill_binary" ./cmd/search-backfill
  CGO_ENABLED=0 go build -o "$reconcile_binary" ./cmd/search-reconcile
)

runtime_args=(
  --network "${project}_default"
  -v "$root_dir/deploy/elasticsearch/search-backfill-smoke.yaml:/app/configs/config.yaml:ro"
  -w /app
)
docker run --rm "${runtime_args[@]}" -v "$migrate_binary:/app/dipole-migrate:ro" \
  alpine:3.22 /app/dipole-migrate -direction up >/dev/null

docker exec -i "$mysql_container" mysql -uroot -pdipole-root dipole <<'SQL'
INSERT INTO outbox_events (
  aggregate_type, aggregate_id, event_type, topic, message_key, value, status, retry_count, created_at, updated_at
) VALUES
('message','M1','message.direct.created','message.direct.created','M1',
'{"event_id":"E1","event_type":"message.direct.created","version":"v1","source":"smoke","occurred_at":"2026-08-27T00:00:00Z","payload":{"mutation_type":"created","revision":1,"actor_uuid":"U1","message_id":"M1","conversation_key":"direct:U1:U2","message_seq":1,"sender_uuid":"U1","target_uuid":"U2","target_type":0,"message_type":0,"content":"first","sent_at":"2026-08-27T00:00:00Z"}}','published',0,NOW(3),NOW(3)),
('message','M2','message.direct.created','message.direct.created','M2',
'{"event_id":"E2","event_type":"message.direct.created","version":"v1","source":"smoke","occurred_at":"2026-08-27T00:00:01Z","payload":{"mutation_type":"created","revision":1,"actor_uuid":"U1","message_id":"M2","conversation_key":"direct:U1:U2","message_seq":2,"sender_uuid":"U1","target_uuid":"U2","target_type":0,"message_type":0,"content":"second","sent_at":"2026-08-27T00:00:01Z"}}','published',0,NOW(3),NOW(3)),
('message','M1@r2','message.direct.edited','message.direct.edited','M1',
'{"event_id":"E3","event_type":"message.direct.edited","version":"v1","source":"smoke","occurred_at":"2026-08-27T00:00:02Z","payload":{"mutation_type":"edited","revision":2,"actor_uuid":"U1","message_id":"M1","conversation_key":"direct:U1:U2","message_seq":1,"sender_uuid":"U1","target_uuid":"U2","target_type":0,"message_type":0,"content":"edited","sent_at":"2026-08-27T00:00:00Z"}}','published',0,NOW(3),NOW(3)),
('message','M3','message.direct.created','message.direct.created','M3',
'{"event_id":"E4","event_type":"message.direct.created","version":"v1","source":"smoke","occurred_at":"2026-08-27T00:00:03Z","payload":{"mutation_type":"created","revision":1,"actor_uuid":"U1","message_id":"M3","conversation_key":"direct:U1:U2","message_seq":3,"sender_uuid":"U1","target_uuid":"U2","target_type":0,"message_type":0,"content":"recall me","sent_at":"2026-08-27T00:00:03Z"}}','published',0,NOW(3),NOW(3)),
('message','M3@r3','message.direct.recalled','message.direct.recalled','M3',
'{"event_id":"E5","event_type":"message.direct.recalled","version":"v1","source":"smoke","occurred_at":"2026-08-27T00:00:04Z","payload":{"mutation_type":"recalled","revision":3,"actor_uuid":"U1","message_id":"M3","target_type":0}}','published',0,NOW(3),NOW(3)),
('user','U1','user.updated','user.updated','U1','{}','published',0,NOW(3),NOW(3));
SQL

docker run --rm "${runtime_args[@]}" -v "$backfill_binary:/app/dipole-search-backfill:ro" \
  alpine:3.22 /app/dipole-search-backfill --job smoke-v1 --target-index "$target_index" \
  --owner smoke-owner --batch-size 2 --lease-seconds 60 >/dev/null

completed_state=$(docker exec "$mysql_container" mysql -N -uroot -pdipole-root dipole \
  -e "SELECT CONCAT(status, ':', last_processed_id, ':', source_high_watermark_id) FROM search_backfill_jobs WHERE job_name = 'smoke-v1';")
[[ "$completed_state" == "completed:5:5" ]] || { printf 'Unexpected Search checkpoint: %s\n' "$completed_state" >&2; exit 1; }

document() {
  local id=$1
  compose exec -T elasticsearch curl -fsS "http://127.0.0.1:9200/${target_index}/_doc/${id}"
}
[[ "$(document M1 | jq -r '._source.revision|tostring')" == 2 ]]
[[ "$(document M1 | jq -r '._source.content')" == edited ]]
[[ "$(document M3 | jq -r '._source.searchable|tostring')" == false ]]

docker run --rm "${runtime_args[@]}" -v "$reconcile_binary:/app/dipole-search-reconcile:ro" \
  alpine:3.22 /app/dipole-search-reconcile --job smoke-v1 --target-index "$target_index" --batch-size 2 \
  >"$reconcile_report"
jq -e '.consistent == true and .source_count == 3 and .target_count == 3 and .hash_matched_count == 3' \
  "$reconcile_report" >/dev/null

current=$(document M2 | jq '._source | .revision = 2 | .payload_hash = "corrupted-hash"')
compose exec -T elasticsearch curl -fsS -X PUT \
  "http://127.0.0.1:9200/${target_index}/_doc/M2?version=2&version_type=external" \
  -H 'Content-Type: application/json' -d "$current" >/dev/null
set +e
docker run --rm "${runtime_args[@]}" -v "$reconcile_binary:/app/dipole-search-reconcile:ro" \
  alpine:3.22 /app/dipole-search-reconcile --job smoke-v1 --target-index "$target_index" --batch-size 2 \
  >"$reconcile_report"
reconcile_exit=$?
set -e
if [[ "$reconcile_exit" -ne 2 ]]; then
  printf 'Expected inconsistent Search reconciliation to exit 2, got %d\n' "$reconcile_exit" >&2
  cat "$reconcile_report"
  exit 1
fi
jq -e '.consistent == false and .hash_mismatch_count == 1' "$reconcile_report" >/dev/null

printf 'Search backfill/reconciliation smoke passed: final mutations rebuilt at Outbox ID 5, clean state matched, and corruption returned exit 2.\n'
