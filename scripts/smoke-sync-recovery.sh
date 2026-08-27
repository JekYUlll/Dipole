#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
project="dipole-sync-recovery-${RANDOM}-$$"
network="${project}-network"
mysql_container="${project}-mysql"
migrate_binary=$(mktemp /tmp/dipole-sync-recovery-migrate.XXXXXX)
replay_binary=$(mktemp /tmp/dipole-sync-replay.XXXXXX)
reconcile_binary=$(mktemp /tmp/dipole-sync-reconcile.XXXXXX)
replay_log=$(mktemp /tmp/dipole-sync-replay.XXXXXX.log)
reconcile_report=$(mktemp /tmp/dipole-sync-reconcile.XXXXXX.json)

cleanup() {
  local exit_code=$?
  docker rm -f "$mysql_container" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -f "$migrate_binary" "$replay_binary" "$reconcile_binary" "$replay_log" "$reconcile_report"
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

printf 'Starting isolated MySQL Sync recovery stack: project=%s\n' "$project"
docker network create "$network" >/dev/null
docker run -d --name "$mysql_container" --network "$network" --network-alias mysql \
  -e MYSQL_ROOT_PASSWORD=dipole-root -e MYSQL_ROOT_HOST=% -e MYSQL_DATABASE=dipole \
  mysql:8.4 >/dev/null
for _ in $(seq 1 90); do
  docker exec "$mysql_container" mysqladmin ping -h 127.0.0.1 -uroot -pdipole-root --silent >/dev/null 2>&1 && break
  sleep 1
done
docker exec "$mysql_container" mysqladmin ping -h 127.0.0.1 -uroot -pdipole-root --silent >/dev/null

(
  cd "$root_dir"
  CGO_ENABLED=0 go build -o "$migrate_binary" ./cmd/migrate
  CGO_ENABLED=0 go build -o "$replay_binary" ./cmd/sync-replay
  CGO_ENABLED=0 go build -o "$reconcile_binary" ./cmd/sync-reconcile
)
runtime_args=(
  --network "$network"
  -v "$root_dir/deploy/mysql/sync-recovery-smoke.yaml:/app/configs/config.yaml:ro"
  -w /app
)
docker run --rm "${runtime_args[@]}" -v "$migrate_binary:/app/dipole-migrate:ro" \
  alpine:3.22 /app/dipole-migrate -direction up >/dev/null

docker exec -i "$mysql_container" mysql -uroot -pdipole-root dipole <<'SQL'
INSERT INTO outbox_events (
  aggregate_type, aggregate_id, event_type, topic, message_key, value,
  headers_json, status, retry_count, next_retry_at, created_at, updated_at
) VALUES
  ('message', 'M-SYNC-R1', 'message.direct.created', 'message.direct.created', 'M-SYNC-R1',
   JSON_OBJECT('event_id','E-R1','event_type','message.direct.created','version','v1','source','smoke','occurred_at','2026-08-27T00:00:00Z','payload',
     JSON_OBJECT('message_id','M-SYNC-R1','conversation_key','direct:U1:U2','message_seq',1,'sender_uuid','U1','target_uuid','U2','target_type',0,'recipient_uuids',JSON_ARRAY('U1','U2'),'sync_fanout',true)),
   NULL, 'published', 0, NOW(3), NOW(3), NOW(3)),
  ('message', 'M-SYNC-HOT', 'message.group.created', 'message.group.created', 'M-SYNC-HOT',
   JSON_OBJECT('event_id','E-HOT','event_type','message.group.created','version','v1','source','smoke','occurred_at','2026-08-27T00:00:01Z','payload',
     JSON_OBJECT('message_id','M-SYNC-HOT','conversation_key','group:G-HOT','message_seq',2,'sender_uuid','U1','target_uuid','G-HOT','target_type',1,'recipient_uuids',JSON_ARRAY('U1','U2'),'sync_fanout',false)),
   NULL, 'published', 0, NOW(3), NOW(3), NOW(3)),
  ('message', 'M-SYNC-G1', 'message.group.created', 'message.group.created', 'M-SYNC-G1',
   JSON_OBJECT('event_id','E-G1','event_type','message.group.created','version','v1','source','smoke','occurred_at','2026-08-27T00:00:02Z','payload',
     JSON_OBJECT('message_id','M-SYNC-G1','conversation_key','group:G1','message_seq',3,'sender_uuid','U1','target_uuid','G1','target_type',1,'recipient_uuids',JSON_ARRAY('U1','U3'),'sync_fanout',true)),
   NULL, 'published', 0, NOW(3), NOW(3), NOW(3));

INSERT INTO user_sync_states (user_uuid, created_at, updated_at) VALUES ('U1', NOW(3), NOW(3));
INSERT INTO user_sync_inbox (user_uuid, message_uuid, conversation_key, message_seq, created_at)
VALUES ('U1', 'M-SYNC-R1', 'direct:U1:U2', 1, NOW(3));
SQL

docker run --rm "${runtime_args[@]}" -v "$replay_binary:/app/dipole-sync-replay:ro" \
  alpine:3.22 /app/dipole-sync-replay --job smoke-v1 --owner first --batch-size 2 --lease-seconds 60 \
  >"$replay_log"
grep -q 'processed=3 projected=2 skipped=1' "$replay_log" || { cat "$replay_log"; exit 1; }

docker run --rm "${runtime_args[@]}" -v "$replay_binary:/app/dipole-sync-replay:ro" \
  alpine:3.22 /app/dipole-sync-replay --job smoke-v1 --owner duplicate --batch-size 2 --lease-seconds 60 \
  >"$replay_log"
grep -q 'processed=0 projected=0 skipped=0' "$replay_log" || { cat "$replay_log"; exit 1; }

docker run --rm "${runtime_args[@]}" -v "$reconcile_binary:/app/dipole-sync-reconcile:ro" \
  alpine:3.22 /app/dipole-sync-reconcile --job smoke-v1 --batch-size 2 >"$reconcile_report"
jq -e '.consistent == true and .events == 3 and .expected_rows == 4 and .actual_rows == 4' "$reconcile_report" >/dev/null

docker exec "$mysql_container" mysql -uroot -pdipole-root dipole \
  -e "DELETE FROM user_sync_inbox WHERE user_uuid = 'U2' AND message_uuid = 'M-SYNC-R1';"
set +e
docker run --rm "${runtime_args[@]}" -v "$reconcile_binary:/app/dipole-sync-reconcile:ro" \
  alpine:3.22 /app/dipole-sync-reconcile --job smoke-v1 --batch-size 2 >"$reconcile_report"
reconcile_exit=$?
set -e
if [[ "$reconcile_exit" -ne 2 ]]; then
  printf 'Expected missing Inbox row to return exit 2, got %d\n' "$reconcile_exit" >&2
  cat "$reconcile_report"
  exit 1
fi
jq -e '.consistent == false and .missing_rows == 1' "$reconcile_report" >/dev/null

docker run --rm "${runtime_args[@]}" -v "$replay_binary:/app/dipole-sync-replay:ro" \
  alpine:3.22 /app/dipole-sync-replay --job smoke-repair-v1 --owner repair --batch-size 2 --lease-seconds 60 \
  >"$replay_log"
docker run --rm "${runtime_args[@]}" -v "$reconcile_binary:/app/dipole-sync-reconcile:ro" \
  alpine:3.22 /app/dipole-sync-reconcile --job smoke-repair-v1 --batch-size 2 >"$reconcile_report"
jq -e '.consistent == true and .missing_rows == 0 and .extra_rows == 0 and .locator_mismatches == 0' "$reconcile_report" >/dev/null

docker run --rm "${runtime_args[@]}" -v "$migrate_binary:/app/dipole-migrate:ro" \
  alpine:3.22 /app/dipole-migrate -direction down -steps 1 -allow-destructive >/dev/null
if docker exec "$mysql_container" mysql -N -uroot -pdipole-root dipole \
  -e "SHOW TABLES LIKE 'sync_replay_jobs';" | grep -q sync_replay_jobs; then
  printf 'Migration v10 rollback left sync_replay_jobs behind\n' >&2
  exit 1
fi
docker run --rm "${runtime_args[@]}" -v "$migrate_binary:/app/dipole-migrate:ro" \
  alpine:3.22 /app/dipole-migrate -direction up >/dev/null
docker exec "$mysql_container" mysql -N -uroot -pdipole-root dipole \
  -e "SHOW TABLES LIKE 'sync_replay_jobs';" | grep -q sync_replay_jobs

printf 'Sync replay/reconciliation smoke passed: partial state recovered, deletion returned exit 2, a new snapshot repaired it, and migration v10 rolled back/up cleanly.\n'
