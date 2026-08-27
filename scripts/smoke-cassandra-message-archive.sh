#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
storage_compose="$root_dir/docker-compose.storage-lab.yml"
project="dipole-cassandra-archive-${RANDOM}-$$"
mysql_container="${project}-mysql"
migrate_binary=$(mktemp /tmp/dipole-cassandra-archive-migrate.XXXXXX)
archive_binary=$(mktemp /tmp/dipole-cassandra-archive.XXXXXX)
backfill_binary=$(mktemp /tmp/dipole-cassandra-archive-backfill.XXXXXX)
reconcile_binary=$(mktemp /tmp/dipole-cassandra-archive-reconcile.XXXXXX)
archive_dir=$(mktemp -d /tmp/dipole-cassandra-message-archive.XXXXXX)
backfill_log=$(mktemp /tmp/dipole-cassandra-archive-backfill.XXXXXX.log)
reconcile_report=$(mktemp /tmp/dipole-cassandra-archive-reconcile.XXXXXX.json)

cleanup() {
  local exit_code=$?
  docker rm -f "$mysql_container" >/dev/null 2>&1 || true
  rm -f "$migrate_binary" "$archive_binary" "$backfill_binary" "$reconcile_binary" "$backfill_log" "$reconcile_report"
  rm -rf "$archive_dir"
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    docker compose -p "$project" -f "$storage_compose" down --volumes --remove-orphans >/dev/null 2>&1 || true
  else
    printf 'Cassandra archive stack retained: project=%s mysql=%s\n' "$project" "$mysql_container"
  fi
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

compose() {
  docker compose -p "$project" -f "$storage_compose" "$@"
}

printf 'Starting isolated MySQL, MinIO, and Cassandra archive stack: project=%s\n' "$project"
compose up -d --wait cassandra minio
compose exec -T cassandra cqlsh <"$root_dir/db/cassandra/001_timeline.cql"

docker run --rm --network "${project}_default" --entrypoint /bin/sh minio/mc:RELEASE.2025-03-12T17-29-24Z -c '
  mc alias set local http://minio:9000 dipoleminio dipoleminiopass &&
  mc mb --ignore-existing --with-lock local/dipole-message-archives &&
  mc version enable local/dipole-message-archives &&
  mc retention set --default governance 30d local/dipole-message-archives
' >/dev/null

docker run -d --name "$mysql_container" \
  --network "${project}_default" --network-alias mysql \
  -e MYSQL_ROOT_PASSWORD=dipole-root -e MYSQL_ROOT_HOST=% -e MYSQL_DATABASE=dipole \
  mysql:8.4.8 >/dev/null
for _ in $(seq 1 90); do
  if docker exec "$mysql_container" mysqladmin ping -h 127.0.0.1 -uroot -pdipole-root --silent >/dev/null 2>&1; then break; fi
  sleep 1
done
docker exec "$mysql_container" mysqladmin ping -h 127.0.0.1 -uroot -pdipole-root --silent >/dev/null

(
  cd "$root_dir"
  CGO_ENABLED=0 go build -o "$migrate_binary" ./cmd/migrate
  CGO_ENABLED=0 go build -o "$archive_binary" ./cmd/cassandra-archive
  CGO_ENABLED=0 go build -o "$backfill_binary" ./cmd/cassandra-backfill
  CGO_ENABLED=0 go build -o "$reconcile_binary" ./cmd/cassandra-reconcile
)

runtime_args=(
  --network "${project}_default"
  -v "$root_dir/deploy/cassandra/backfill-smoke.yaml:/app/configs/config.yaml:ro"
  -v "$archive_dir:/archive"
  -w /app
)
docker run --rm "${runtime_args[@]}" -v "$migrate_binary:/app/dipole-migrate:ro" alpine:3.22 \
  /app/dipole-migrate -direction up >/dev/null

docker exec -i "$mysql_container" mysql -uroot -pdipole-root dipole <<'SQL'
INSERT INTO messages (
  uuid, client_message_id, conversation_key, seq, sender_uuid,
  target_type, target_uuid, message_type, content,
  file_id, file_name, file_size, file_url, file_content_type, file_expires_at, sent_at
) VALUES
  ('M-ARCHIVE-1', 'C-ARCHIVE-1', 'direct:U100:U200', 1, 'U100', 0, 'U200', 0, 'first', '', '', 0, '', '', NULL, '2026-08-27 00:00:01.000'),
  ('M-ARCHIVE-2', 'C-ARCHIVE-2', 'direct:U100:U200', 2, 'U100', 0, 'U200', 1, 'file', 'F-2', 'report.pdf', 42, 's3://files/F-2', 'application/pdf', '2026-09-01 00:00:00.000', '2026-08-27 00:00:02.000'),
  ('M-ARCHIVE-3', 'C-ARCHIVE-3', 'direct:U100:U200', 3, 'U100', 0, 'U200', 0, 'last', '', '', 0, '', '', NULL, '2026-08-27 00:00:03.000');
SQL

docker run --rm "${runtime_args[@]}" -v "$archive_binary:/app/dipole-cassandra-archive:ro" alpine:3.22 \
  /app/dipole-cassandra-archive -action create -manifest /archive/snapshot.json -snapshot-id smoke-snapshot-3 -batch-size 2 >/dev/null
docker run --rm "${runtime_args[@]}" -v "$archive_binary:/app/dipole-cassandra-archive:ro" alpine:3.22 \
  /app/dipole-cassandra-archive -action publish -manifest /archive/snapshot.json -receipt /archive/receipt.json >/dev/null
docker run --rm -v "$archive_dir:/archive" alpine:3.22 chmod 0644 /archive/receipt.json

manifest_version=$(jq -r '.manifest.version_id' "$archive_dir/receipt.json")
manifest_key=$(jq -r '.manifest.object_key' "$archive_dir/receipt.json")
rm -f "$archive_dir/snapshot.json" "$archive_dir/snapshot.ndjson"
mkdir "$archive_dir/restored"
docker run --rm "${runtime_args[@]}" -v "$archive_binary:/app/dipole-cassandra-archive:ro" alpine:3.22 \
  /app/dipole-cassandra-archive -action restore -receipt /archive/receipt.json -destination /archive/restored >/dev/null

docker exec "$mysql_container" mysql -uroot -pdipole-root dipole -e 'DELETE FROM messages;'
remaining_messages=$(docker exec "$mysql_container" mysql -N -uroot -pdipole-root dipole -e 'SELECT COUNT(*) FROM messages;')
[[ "$remaining_messages" == "0" ]] || { printf 'MySQL message bodies were not deleted\n' >&2; exit 1; }

docker run --rm "${runtime_args[@]}" -v "$backfill_binary:/app/dipole-cassandra-backfill:ro" alpine:3.22 \
  /app/dipole-cassandra-backfill --job archive-smoke-v1 --owner archive-owner --source archive \
  --archive-manifest /archive/restored/snapshot.json --batch-size 2 --lease-seconds 60 >"$backfill_log"
grep -q 'processed=3 inserted=3 duplicates=0' "$backfill_log" || { cat "$backfill_log"; exit 1; }

job_state=$(docker exec "$mysql_container" mysql -N -uroot -pdipole-root dipole -e \
  "SELECT CONCAT(status, ':', source_kind, ':', source_snapshot_id, ':', last_processed_id, ':', source_high_watermark_id) FROM cassandra_backfill_jobs WHERE job_name = 'archive-smoke-v1';")
[[ "$job_state" == "completed:message_archive:smoke-snapshot-3:3:3" ]] || { printf 'Unexpected archive job state: %s\n' "$job_state" >&2; exit 1; }

set +e
docker run --rm "${runtime_args[@]}" -v "$backfill_binary:/app/dipole-cassandra-backfill:ro" alpine:3.22 \
  /app/dipole-cassandra-backfill --job archive-smoke-v1 --owner changed-source --source mysql --batch-size 2 --lease-seconds 60 \
  >"$backfill_log" 2>&1
changed_source_exit=$?
set -e
[[ "$changed_source_exit" -ne 0 ]] || { printf 'Expected completed job source change to fail\n' >&2; exit 1; }
grep -q 'source does not match job' "$backfill_log" || { cat "$backfill_log"; exit 1; }

docker run --rm "${runtime_args[@]}" -v "$reconcile_binary:/app/dipole-cassandra-reconcile:ro" alpine:3.22 \
  /app/dipole-cassandra-reconcile --job archive-smoke-v1 --source archive \
  --archive-manifest /archive/restored/snapshot.json --batch-size 2 --sample-modulus 1 >"$reconcile_report"
jq -e '.consistent == true and .source_count == 3 and .target_found_count == 3 and .hash_matched_count == 3 and .sampled_count == 3' \
  "$reconcile_report" >/dev/null

compose exec -T cassandra cqlsh -e "UPDATE dipole_message_shadow.timeline_by_conversation_bucket SET content = 'corrupted', payload_hash = 'corrupted-hash' WHERE conversation_key = 'direct:U100:U200' AND bucket = 0 AND message_seq = 2;"
set +e
docker run --rm "${runtime_args[@]}" -v "$reconcile_binary:/app/dipole-cassandra-reconcile:ro" alpine:3.22 \
  /app/dipole-cassandra-reconcile --job archive-smoke-v1 --source archive \
  --archive-manifest /archive/restored/snapshot.json --batch-size 2 --sample-modulus 1 >"$reconcile_report"
reconcile_exit=$?
set -e
[[ "$reconcile_exit" -eq 2 ]] || { printf 'Expected corrupted reconciliation to exit 2, got %d\n' "$reconcile_exit" >&2; exit 1; }
jq -e '.consistent == false and .hash_mismatch_count == 1 and .sample_mismatch_count == 1' "$reconcile_report" >/dev/null

set +e
docker run --rm --network "${project}_default" --entrypoint /bin/sh minio/mc:RELEASE.2025-03-12T17-29-24Z -c \
  "mc alias set local http://minio:9000 dipoleminio dipoleminiopass >/dev/null && mc rm --version-id '$manifest_version' 'local/dipole-message-archives/$manifest_key'" >/dev/null 2>&1
retention_delete_exit=$?
set -e
[[ "$retention_delete_exit" -ne 0 ]] || { printf 'Expected retained message archive deletion to fail\n' >&2; exit 1; }

printf 'Cassandra message archive smoke passed: pinned restore rebuilt and reconciled three messages after MySQL body deletion; source swap, corruption, and retained deletion were rejected.\n'
