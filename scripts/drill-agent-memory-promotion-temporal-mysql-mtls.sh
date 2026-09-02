#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
go_bin="${DIPOLE_GO_BIN:-go}"
node_bin="${DIPOLE_NODE_BIN:-node}"
port="${DIPOLE_AGENT_PROMOTION_MYSQL_PORT:-23316}"
project="dipole-agent-temporal-mysql-mtls-$$"
compose_file="$root_dir/deploy/agent/memory-promotion-mysql-contract.compose.yml"
fixture_dir="$(mktemp -d)"
fixture_pid=""

cleanup() {
  if [[ -n "$fixture_pid" ]]; then
    touch "$fixture_dir/stop"
    wait "$fixture_pid" >/dev/null 2>&1 || true
  fi
  docker compose --project-name "$project" --file "$compose_file" down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$fixture_dir"
}
trap cleanup EXIT

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
command -v "$go_bin" >/dev/null 2>&1 || { echo "Go binary is unavailable: $go_bin" >&2; exit 1; }
command -v "$node_bin" >/dev/null 2>&1 || { echo "Node binary is unavailable: $node_bin" >&2; exit 1; }
export PATH="$(dirname "$node_bin"):$PATH"

docker compose --project-name "$project" --file "$compose_file" up --wait

cd "$root_dir"
DIPOLE_AGENT_TEMPORAL_MYSQL_MTLS_FIXTURE=true \
DIPOLE_AGENT_TEMPORAL_MYSQL_MTLS_READY="$fixture_dir/ready.json" \
DIPOLE_AGENT_TEMPORAL_MYSQL_MTLS_STOP="$fixture_dir/stop" \
DIPOLE_TEST_MYSQL_ADMIN_DSN="root:contract-root@tcp(127.0.0.1:${port})/?parseTime=true&loc=UTC" \
LD_LIBRARY_PATH="/usr/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}" \
  "$go_bin" test ./internal/services/agent/infrastructure/mysql -run '^TestAgentMemoryPromotionTemporalMySQLMTLSFixtureProcess$' -count=1 -v >"$fixture_dir/fixture.log" 2>&1 &
fixture_pid=$!

for _ in $(seq 1 300); do
  [[ -s "$fixture_dir/ready.json" ]] && break
  if ! kill -0 "$fixture_pid" 2>/dev/null; then
    cat "$fixture_dir/fixture.log" >&2
    exit 1
  fi
  sleep 0.1
done
test -s "$fixture_dir/ready.json" || { cat "$fixture_dir/fixture.log" >&2; exit 1; }

cd "$root_dir/services/agent-runtime"
DIPOLE_AGENT_TEMPORAL_MYSQL_MTLS_INTEGRATION=true \
DIPOLE_AGENT_TEMPORAL_MYSQL_MTLS_FIXTURE="$fixture_dir/ready.json" \
  npm test -- --run src/temporal/agent-memory-promotion-mtls-mysql.integration.test.ts

touch "$fixture_dir/stop"
wait "$fixture_pid"
fixture_pid=""
echo "Agent Memory promotion Temporal/MySQL mTLS drill passed"
