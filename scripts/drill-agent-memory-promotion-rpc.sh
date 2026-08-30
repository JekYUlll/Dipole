#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture_dir="$(mktemp -d)"
fixture_pid=""
fixture_ready="$fixture_dir/ready.json"
fixture_stop="$fixture_dir/stop"
fixture_state="$fixture_dir/state.json"
fixture_stale="$fixture_dir/stale"
fixture_log="$fixture_dir/fixture.log"

cleanup() {
  if [[ -n "$fixture_pid" ]]; then
    touch "$fixture_stop"
    wait "$fixture_pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$fixture_dir"
}
trap cleanup EXIT

INTERNAL_CERT_DIR="$fixture_dir/certs" INTERNAL_CERT_VALID_DAYS=1 \
  "$root_dir/scripts/generate-internal-certs.sh" >/dev/null

LD_LIBRARY_PATH="/usr/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}" \
  go test "$root_dir/internal/bootstrap" -run '^TestAgentMCPRPCDrillFixtureAuthentication$' -count=1

DIPOLE_AGENT_RPC_DRILL_FIXTURE=true \
DIPOLE_AGENT_RPC_DRILL_READY="$fixture_ready" \
DIPOLE_AGENT_RPC_DRILL_STOP="$fixture_stop" \
DIPOLE_AGENT_RPC_DRILL_STATE="$fixture_state" \
DIPOLE_AGENT_RPC_DRILL_STALE="$fixture_stale" \
DIPOLE_AGENT_RPC_DRILL_SECRET="agent-mcp-rpc-drill-secret" \
DIPOLE_AGENT_RPC_DRILL_SERVER_CERT="$fixture_dir/certs/core.pem" \
DIPOLE_AGENT_RPC_DRILL_SERVER_KEY="$fixture_dir/certs/core-key.pem" \
DIPOLE_AGENT_RPC_DRILL_CA="$fixture_dir/certs/ca.pem" \
LD_LIBRARY_PATH="/usr/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}" \
  go test "$root_dir/internal/bootstrap" -run '^TestAgentMCPRPCDrillFixtureProcess$' -count=1 -v >"$fixture_log" 2>&1 &
fixture_pid=$!

for _ in $(seq 1 300); do
  [[ -s "$fixture_ready" ]] && break
  if ! kill -0 "$fixture_pid" 2>/dev/null; then
    cat "$fixture_log" >&2
    exit 1
  fi
  sleep 0.1
done
test -s "$fixture_ready" || {
  cat "$fixture_log" >&2
  exit 1
}

fixture_address="$(jq -er '.address | select(type == "string" and length > 0)' "$fixture_ready")"
cd "$root_dir/services/agent-runtime"
DIPOLE_AGENT_MEMORY_PROMOTION_RPC_DRILL=true \
DIPOLE_TEST_AGENT_PROMOTION_RPC_TARGET="$fixture_address" \
DIPOLE_TEST_AGENT_PROMOTION_RPC_SECRET="agent-mcp-rpc-drill-secret" \
DIPOLE_TEST_AGENT_PROMOTION_RPC_CA_FILE="$fixture_dir/certs/ca.pem" \
DIPOLE_TEST_AGENT_PROMOTION_RPC_CERT_FILE="$fixture_dir/certs/agent.pem" \
DIPOLE_TEST_AGENT_PROMOTION_RPC_KEY_FILE="$fixture_dir/certs/agent-key.pem" \
DIPOLE_TEST_AGENT_PROMOTION_RPC_SERVER_NAME="core" \
  npm test -- --run src/capabilities/agent-memory-promotion-rpc-drill.integration.test.ts

jq -e '.rpc_type == "go_internal_grpc_mtls" and .rpc_authenticated == true and .memory_promotion_commit_count == 1' "$fixture_state" >/dev/null
echo "Agent Memory promotion mTLS RPC drill passed"
