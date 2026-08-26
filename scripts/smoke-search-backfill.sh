#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
storage_compose="$root_dir/docker-compose.storage-lab.yml"
project="dipole-search-backfill-${RANDOM}-$$"
mysql_container="${project}-mysql"
migrate_binary=$(mktemp /tmp/dipole-search-backfill-migrate.XXXXXX)
backfill_binary=$(mktemp /tmp/dipole-search-backfill.XXXXXX)
reconcile_binary=$(mktemp /tmp/dipole-search-reconcile.XXXXXX)
alias_binary=$(mktemp /tmp/dipole-search-alias.XXXXXX)
reconcile_report=$(mktemp /tmp/dipole-search-reconcile.XXXXXX.json)
alias_receipt=$(mktemp /tmp/dipole-search-alias.XXXXXX.json)
target_index="dipole-smoke-messages-v1-build-a"
old_index="dipole-smoke-messages-v1-old"

cleanup() {
  local exit_code=$?
  docker rm -f "$mysql_container" >/dev/null 2>&1 || true
  rm -f "$migrate_binary" "$backfill_binary" "$reconcile_binary" "$alias_binary" "$reconcile_report" "$alias_receipt"
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
  CGO_ENABLED=0 go build -o "$alias_binary" ./cmd/search-alias
)

runtime_args=(
  --network "${project}_default"
  -v "$root_dir/deploy/elasticsearch/search-backfill-smoke.yaml:/app/configs/config.yaml:ro"
  -w /app
)
docker run --rm "${runtime_args[@]}" -v "$migrate_binary:/app/dipole-migrate:ro" \
  alpine:3.22 /app/dipole-migrate -direction up >/dev/null

jq -n \
  --slurpfile mapping "$root_dir/internal/data/elasticsearch/schema/message_search_v1.json" \
  '{settings:{number_of_shards:1,number_of_replicas:0},mappings:$mapping[0],aliases:{"dipole-smoke-messages-read":{},"dipole-smoke-messages-write":{is_write_index:true}}}' | \
  compose exec -T elasticsearch curl -fsS -X PUT "http://127.0.0.1:9200/${old_index}" \
    -H 'Content-Type: application/json' --data-binary @- >/dev/null

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
docker run --rm "${runtime_args[@]}" -v "$backfill_binary:/app/dipole-search-backfill:ro" \
  alpine:3.22 /app/dipole-search-backfill --job smoke-old-v1 --target-index "$old_index" \
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

docker run --rm "${runtime_args[@]}" -v "$alias_binary:/app/dipole-search-alias:ro" \
  alpine:3.22 /app/dipole-search-alias --action switch --job smoke-v1 \
  --from-index "$old_index" --to-index "$target_index" --confirm-maintenance-window \
  >"$alias_receipt"
jq -e --arg from "$old_index" --arg to "$target_index" \
  '.action == "switch" and .from_index == $from and .to_index == $to and .source_high_watermark_id == 5' \
  "$alias_receipt" >/dev/null
compose exec -T elasticsearch curl -fsS \
  'http://127.0.0.1:9200/_alias/dipole-smoke-messages-read,dipole-smoke-messages-write' | \
  jq -e --arg target "$target_index" 'keys == [$target]' >/dev/null

docker run --rm "${runtime_args[@]}" -v "$alias_binary:/app/dipole-search-alias:ro" \
  alpine:3.22 /app/dipole-search-alias --action rollback --job smoke-old-v1 \
  --from-index "$target_index" --to-index "$old_index" --confirm-maintenance-window \
  >"$alias_receipt"
jq -e '.action == "rollback" and .source_high_watermark_id == 5' "$alias_receipt" >/dev/null
compose exec -T elasticsearch curl -fsS \
  'http://127.0.0.1:9200/_alias/dipole-smoke-messages-read,dipole-smoke-messages-write' | \
  jq -e --arg target "$old_index" 'keys == [$target]' >/dev/null

docker exec -i "$mysql_container" mysql -uroot -pdipole-root dipole <<'SQL'
INSERT INTO outbox_events (
  aggregate_type, aggregate_id, event_type, topic, message_key, value, status, retry_count, created_at, updated_at
) VALUES ('message','M4','message.direct.created','message.direct.created','M4',
'{"event_id":"E7","event_type":"message.direct.created","version":"v1","source":"smoke","occurred_at":"2026-08-27T00:00:06Z","payload":{"mutation_type":"created","revision":1,"actor_uuid":"U1","message_id":"M4","conversation_key":"direct:U1:U2","message_seq":4,"sender_uuid":"U1","target_uuid":"U2","target_type":0,"message_type":0,"content":"new after snapshot","sent_at":"2026-08-27T00:00:06Z"}}','published',0,NOW(3),NOW(3));
SQL
set +e
docker run --rm "${runtime_args[@]}" -v "$alias_binary:/app/dipole-search-alias:ro" \
  alpine:3.22 /app/dipole-search-alias --action switch --job smoke-v1 \
  --from-index "$old_index" --to-index "$target_index" --confirm-maintenance-window \
  >"$alias_receipt" 2>&1
stale_exit=$?
set -e
if [[ "$stale_exit" -eq 0 ]] || ! grep -q 'snapshot became stale' "$alias_receipt"; then
  printf 'Expected stale Search snapshot to block Alias switch\n' >&2
  cat "$alias_receipt"
  exit 1
fi
compose exec -T elasticsearch curl -fsS \
  'http://127.0.0.1:9200/_alias/dipole-smoke-messages-read,dipole-smoke-messages-write' | \
  jq -e --arg target "$old_index" 'keys == [$target]' >/dev/null

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

printf 'Search recovery/Alias smoke passed: fixed snapshot matched, forward and rollback switches succeeded, stale cutover was blocked, and corruption returned exit 2.\n'
