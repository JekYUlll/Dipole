#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
storage_compose="$root_dir/docker-compose.storage-lab.yml"
project="dipole-cassandra-backfill-${RANDOM}-$$"
mysql_container="${project}-mysql"
migrate_binary=$(mktemp /tmp/dipole-cassandra-backfill-migrate.XXXXXX)
backfill_binary=$(mktemp /tmp/dipole-cassandra-backfill.XXXXXX)
backfill_log=$(mktemp /tmp/dipole-cassandra-backfill.XXXXXX.log)
reconcile_binary=$(mktemp /tmp/dipole-cassandra-reconcile.XXXXXX)
reconcile_report=$(mktemp /tmp/dipole-cassandra-reconcile.XXXXXX.json)

cleanup() {
  local exit_code=$?
  docker rm -f "$mysql_container" >/dev/null 2>&1 || true
  rm -f "$migrate_binary" "$backfill_binary" "$backfill_log" "$reconcile_binary" "$reconcile_report"
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    docker compose -p "$project" -f "$storage_compose" down --volumes --remove-orphans >/dev/null 2>&1 || true
  else
    printf 'Cassandra backfill stack retained: project=%s mysql=%s\n' "$project" "$mysql_container"
  fi
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

compose() {
  docker compose -p "$project" -f "$storage_compose" "$@"
}

printf 'Starting isolated MySQL and Cassandra backfill stack: project=%s\n' "$project"
compose up -d --wait cassandra
compose exec -T cassandra cqlsh <"$root_dir/db/cassandra/001_timeline.cql"

docker run -d --name "$mysql_container" \
  --network "${project}_default" \
  --network-alias mysql \
  -e MYSQL_ROOT_PASSWORD=dipole-root \
  -e MYSQL_ROOT_HOST=% \
  -e MYSQL_DATABASE=dipole \
  mysql:8.4.8 >/dev/null

for _ in $(seq 1 90); do
  if docker exec "$mysql_container" mysqladmin ping -h 127.0.0.1 -uroot -pdipole-root --silent >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$mysql_container" mysqladmin ping -h 127.0.0.1 -uroot -pdipole-root --silent >/dev/null

(
  cd "$root_dir"
  CGO_ENABLED=0 go build -o "$migrate_binary" ./cmd/tools/migrate
  CGO_ENABLED=0 go build -o "$backfill_binary" ./cmd/tools/cassandra-backfill
  CGO_ENABLED=0 go build -o "$reconcile_binary" ./cmd/tools/cassandra-reconcile
)

runtime_args=(
  --network "${project}_default"
  -v "$root_dir/deploy/cassandra/backfill-smoke.yaml:/app/configs/config.yaml:ro"
  -w /app
)
docker run --rm "${runtime_args[@]}" -v "$migrate_binary:/app/dipole-migrate:ro" \
  alpine:3.22 /app/dipole-migrate -direction up >/dev/null

docker exec -i "$mysql_container" mysql -uroot -pdipole-root dipole <<'SQL'
INSERT INTO messages (
  uuid, client_message_id, conversation_key, seq, sender_uuid,
  target_type, target_uuid, message_type, content, file_expires_at, sent_at
) VALUES
  ('M-BACKFILL-1', 'C-BACKFILL-1', 'direct:U100:U200', 1, 'U100', 0, 'U200', 0, 'first', NULL, '2026-08-27 00:00:01.000'),
  ('M-BACKFILL-2', 'C-BACKFILL-2', 'direct:U100:U200', 0, 'U100', 0, 'U200', 0, 'retry', '2026-09-01 00:00:00.000', '2026-08-27 00:00:02.000'),
  ('M-BACKFILL-3', 'C-BACKFILL-3', 'direct:U100:U200', 3, 'U100', 0, 'U200', 0, 'last', NULL, '2026-08-27 00:00:03.000');
SQL

set +e
docker run --rm "${runtime_args[@]}" -v "$backfill_binary:/app/dipole-cassandra-backfill:ro" \
  alpine:3.22 /app/dipole-cassandra-backfill --job smoke-v1 --owner failed-owner --batch-size 2 --lease-seconds 60 \
  >"$backfill_log" 2>&1
first_exit=$?
set -e
if [[ "$first_exit" -eq 0 ]]; then
  printf 'Expected the invalid source sequence to fail the first backfill attempt\n' >&2
  cat "$backfill_log"
  exit 1
fi

failed_state=$(docker exec "$mysql_container" mysql -N -uroot -pdipole-root dipole \
  -e "SELECT CONCAT(status, ':', last_processed_id, ':', source_high_watermark_id, ':', attempt_count) FROM cassandra_backfill_jobs WHERE job_name = 'smoke-v1';")
if [[ "$failed_state" != "failed:0:3:1" ]]; then
  printf 'Unexpected failed checkpoint: %s\n' "$failed_state" >&2
  cat "$backfill_log"
  exit 1
fi

docker exec "$mysql_container" mysql -uroot -pdipole-root dipole \
  -e "UPDATE messages SET seq = 2 WHERE uuid = 'M-BACKFILL-2';"
docker run --rm "${runtime_args[@]}" -v "$backfill_binary:/app/dipole-cassandra-backfill:ro" \
  alpine:3.22 /app/dipole-cassandra-backfill --job smoke-v1 --owner recovery-owner --batch-size 2 --lease-seconds 60 \
  >"$backfill_log" 2>&1

grep -q 'processed=3 inserted=2 duplicates=1' "$backfill_log" || {
  printf 'Recovery did not report the expected idempotent replay\n' >&2
  cat "$backfill_log"
  exit 1
}
completed_state=$(docker exec "$mysql_container" mysql -N -uroot -pdipole-root dipole \
  -e "SELECT CONCAT(status, ':', last_processed_id, ':', source_high_watermark_id, ':', attempt_count) FROM cassandra_backfill_jobs WHERE job_name = 'smoke-v1';")
if [[ "$completed_state" != "completed:3:3:2" ]]; then
  printf 'Unexpected completed checkpoint: %s\n' "$completed_state" >&2
  exit 1
fi

rows=$(compose exec -T cassandra cqlsh -e "SELECT COUNT(*) FROM dipole_message_shadow.timeline_by_conversation_bucket WHERE conversation_key = 'direct:U100:U200' AND bucket = 0;" | awk '/^[[:space:]]*[0-9]+[[:space:]]*$/ {gsub(/[[:space:]]/, ""); print; exit}')
if [[ "$rows" != "3" ]]; then
  printf 'Expected three recovered Cassandra rows, got %s\n' "$rows" >&2
  exit 1
fi

docker run --rm "${runtime_args[@]}" -v "$backfill_binary:/app/dipole-cassandra-backfill:ro" \
  alpine:3.22 /app/dipole-cassandra-backfill --job smoke-v1 --owner completed-owner --batch-size 2 --lease-seconds 60 \
  >"$backfill_log" 2>&1
grep -q 'processed=0 inserted=0 duplicates=0' "$backfill_log" || {
  printf 'Completed backfill did not return as a no-op\n' >&2
  cat "$backfill_log"
  exit 1
}

docker run --rm "${runtime_args[@]}" -v "$reconcile_binary:/app/dipole-cassandra-reconcile:ro" \
  alpine:3.22 /app/dipole-cassandra-reconcile --job smoke-v1 --batch-size 2 --sample-modulus 1 \
  >"$reconcile_report"
jq -e '.consistent == true and .source_count == 3 and .target_found_count == 3 and .hash_matched_count == 3 and .sampled_count == 3' \
  "$reconcile_report" >/dev/null || {
  printf 'Expected a consistent Cassandra reconciliation report\n' >&2
  cat "$reconcile_report"
  exit 1
}

compose exec -T cassandra cqlsh -e "UPDATE dipole_message_shadow.timeline_by_conversation_bucket SET content = 'corrupted', payload_hash = 'corrupted-hash' WHERE conversation_key = 'direct:U100:U200' AND bucket = 0 AND message_seq = 2;"
set +e
docker run --rm "${runtime_args[@]}" -v "$reconcile_binary:/app/dipole-cassandra-reconcile:ro" \
  alpine:3.22 /app/dipole-cassandra-reconcile --job smoke-v1 --batch-size 2 --sample-modulus 1 \
  >"$reconcile_report"
reconcile_exit=$?
set -e
if [[ "$reconcile_exit" -ne 2 ]]; then
  printf 'Expected inconsistent reconciliation to exit 2, got %d\n' "$reconcile_exit" >&2
  cat "$reconcile_report"
  exit 1
fi
jq -e '.consistent == false and .hash_mismatch_count == 1 and .sample_mismatch_count == 1' \
  "$reconcile_report" >/dev/null || {
  printf 'Expected hash and sample mismatch diagnostics\n' >&2
  cat "$reconcile_report"
  exit 1
}

printf 'Cassandra backfill/reconciliation smoke passed: recovery completed at source ID 3, clean data matched, and corruption returned exit 2.\n'
