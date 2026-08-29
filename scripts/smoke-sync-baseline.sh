#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
project="dipole-sync-baseline-${RANDOM}-$$"
network="${project}-network"
mysql_container="${project}-mysql"
migrate_binary=$(mktemp /tmp/dipole-sync-baseline-migrate.XXXXXX)
baseline_binary=$(mktemp /tmp/dipole-sync-baseline.XXXXXX)
report=$(mktemp /tmp/dipole-sync-baseline.XXXXXX.json)
error_log=$(mktemp /tmp/dipole-sync-baseline.XXXXXX.log)

cleanup() {
  local exit_code=$?
  docker rm -f "$mysql_container" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -f "$migrate_binary" "$baseline_binary" "$report" "$error_log"
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

printf 'Starting isolated MySQL Sync baseline stack: project=%s\n' "$project"
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
  CGO_ENABLED=0 go build -o "$migrate_binary" ./cmd/tools/migrate
  CGO_ENABLED=0 go build -o "$baseline_binary" ./cmd/tools/sync-baseline
)
runtime_args=(
  --network "$network"
  -v "$root_dir/deploy/mysql/sync-recovery-smoke.yaml:/app/configs/config.yaml:ro"
  -w /app
)
docker run --rm "${runtime_args[@]}" -v "$migrate_binary:/app/dipole-migrate:ro" \
  alpine:3.22 /app/dipole-migrate -direction up >/dev/null

docker exec -i "$mysql_container" mysql -uroot -pdipole-root dipole <<'SQL'
INSERT INTO user_sync_states (user_uuid, created_at, updated_at) VALUES
  ('U1', NOW(3), NOW(3)), ('U2', NOW(3), NOW(3)), ('U3', NOW(3), NOW(3));
INSERT INTO user_sync_inbox
  (sync_seq, user_uuid, message_uuid, conversation_key, message_seq, created_at) VALUES
  (10, 'U1', 'M-LEGACY', 'group:G1', 4, NOW(3)),
  (11, 'U2', 'M-LEGACY', 'group:G1', 4, NOW(3)),
  (12, 'U3', 'M-EVENT', 'direct:U1:U3', 9, NOW(3));
INSERT INTO outbox_events (
  aggregate_type, aggregate_id, event_type, topic, message_key, value,
  status, retry_count, created_at, updated_at
) VALUES (
  'message', 'M-EVENT', 'message.direct.created', 'message.direct.created',
  'M-EVENT', '{}', 'published', 0, NOW(3), NOW(3)
);
SQL

run_baseline() {
  docker run --rm "${runtime_args[@]}" -v "$baseline_binary:/app/dipole-sync-baseline:ro" \
    alpine:3.22 /app/dipole-sync-baseline "$@"
}

run_baseline --action capture --job smoke-legacy-v1 >"$report"
jq -e '.high_watermark_sync_seq == 12 and .entry_count == 2 and (.entries_sha256 | length) == 64' "$report" >/dev/null
first_digest=$(jq -r '.entries_sha256' "$report")
run_baseline --action capture --job smoke-legacy-v1 >"$report"
test "$(jq -r '.entries_sha256' "$report")" = "$first_digest"

run_baseline --action reconcile --job smoke-legacy-v1 >"$report"
jq -e '.consistent == true and .expected_rows == 2 and .actual_rows == 2' "$report" >/dev/null
docker exec "$mysql_container" mysql -uroot -pdipole-root dipole \
  -e "DELETE FROM user_sync_inbox WHERE sync_seq = 11;"
set +e
run_baseline --action reconcile --job smoke-legacy-v1 >"$report"
reconcile_exit=$?
set -e
test "$reconcile_exit" -eq 2
jq -e '.consistent == false and .missing == 1' "$report" >/dev/null

run_baseline --action restore --job smoke-legacy-v1 >"$report"
jq -e '.consistent == true' "$report" >/dev/null
restored_seq=$(docker exec "$mysql_container" mysql -N -B -uroot -pdipole-root dipole \
  -e "SELECT sync_seq FROM user_sync_inbox WHERE user_uuid='U2' AND message_uuid='M-LEGACY';")
test "$restored_seq" = "11"

docker exec -i "$mysql_container" mysql -uroot -pdipole-root dipole <<'SQL'
DELETE FROM user_sync_inbox WHERE sync_seq = 11;
INSERT INTO user_sync_inbox
  (sync_seq, user_uuid, message_uuid, conversation_key, message_seq, created_at)
VALUES (13, 'U2', 'M-LEGACY', 'group:G1', 4, NOW(3));
SQL
set +e
run_baseline --action restore --job smoke-legacy-v1 >"$report" 2>"$error_log"
restore_exit=$?
set -e
test "$restore_exit" -eq 1
jq -e '.consistent == false and .conflicting == 1' "$report" >/dev/null
grep -q 'missing-only reconciliation' "$error_log"

docker exec -i "$mysql_container" mysql -uroot -pdipole-root dipole <<'SQL'
DELETE FROM user_sync_inbox WHERE sync_seq = 13;
SQL
run_baseline --action restore --job smoke-legacy-v1 >"$report"
jq -e '.consistent == true' "$report" >/dev/null

printf 'Sync historical baseline smoke passed; isolated stack will be removed.\n'
