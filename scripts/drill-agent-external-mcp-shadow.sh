#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
node_bin="${DIPOLE_NODE_BIN:-node}"
node_path="$(command -v "$node_bin")"
node_dir="$(dirname "$node_path")"
npm_bin="${DIPOLE_NPM_BIN:-$node_dir/npm}"
# npm's launcher resolves `node` from PATH. Keep it aligned with DIPOLE_NODE_BIN.
export PATH="$node_dir:$PATH"
compose_file="$root_dir/deploy/agent/external-mcp-shadow-drill.compose.yml"
project_name="${COMPOSE_PROJECT_NAME:-dipole-agent-mcp-drill-${RANDOM}-$$}"
evidence_path="${DIPOLE_AGENT_MCP_DRILL_EVIDENCE:-$root_dir/services/agent-runtime/.artifacts/external-mcp-shadow-drill.json}"
approval_evidence_path="${DIPOLE_AGENT_APPROVAL_DRILL_EVIDENCE:-$root_dir/services/agent-runtime/.artifacts/approval-gate-drill.json}"
fixture_dir="$(mktemp -d)"
fixture_pid=""
fixture_ready="$fixture_dir/ready.json"
fixture_stop="$fixture_dir/stop"
fixture_state="$fixture_dir/state.json"
fixture_stale="$fixture_dir/stale"
fixture_log="$fixture_dir/fixture.log"

pick_port() {
  local port
  while true; do
    port="$((20000 + RANDOM % 30000))"
    if ! ss -Hln "sport = :$port" | grep -q .; then
      printf '%s\n' "$port"
      return
    fi
  done
}

export DIPOLE_AGENT_DRILL_MYSQL_PORT="${DIPOLE_AGENT_DRILL_MYSQL_PORT:-$(pick_port)}"
export DIPOLE_AGENT_DRILL_KAFKA_PORT="${DIPOLE_AGENT_DRILL_KAFKA_PORT:-$(pick_port)}"

compose() {
  docker compose -p "$project_name" -f "$compose_file" "$@"
}

cleanup() {
  if [[ -n "$fixture_pid" ]]; then
    touch "$fixture_stop"
    wait "$fixture_pid" >/dev/null 2>&1 || true
  fi
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$fixture_dir"
}
trap cleanup EXIT

mkdir -p "$(dirname "$evidence_path")"
mkdir -p "$(dirname "$approval_evidence_path")"
compose up -d --wait
sleep 5

INTERNAL_CERT_DIR="$fixture_dir/certs" INTERNAL_CERT_VALID_DAYS=1 \
  "$root_dir/scripts/generate-internal-certs.sh" >/dev/null 2>&1
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
node_install_marker="node_modules/.dipole-node-path"
if [[ ! -x node_modules/.bin/vitest ]] || [[ ! -f "$node_install_marker" ]] || [[ "$(<"$node_install_marker")" != "$node_path" ]]; then
  "$npm_bin" ci --ignore-scripts
  printf '%s\n' "$node_path" >"$node_install_marker"
fi
export DIPOLE_AGENT_FULL_STACK_DRILL=true
export DIPOLE_TEST_AGENT_MYSQL_URL="mysql://root:drill-root@127.0.0.1:${DIPOLE_AGENT_DRILL_MYSQL_PORT}/dipole"
export DIPOLE_TEST_AGENT_KAFKA_BROKERS="127.0.0.1:${DIPOLE_AGENT_DRILL_KAFKA_PORT}"
export DIPOLE_TEST_AGENT_RPC_TARGET="$fixture_address"
export DIPOLE_TEST_AGENT_RPC_SECRET="agent-mcp-rpc-drill-secret"
export DIPOLE_TEST_AGENT_RPC_CA_FILE="$fixture_dir/certs/ca.pem"
export DIPOLE_TEST_AGENT_RPC_CERT_FILE="$fixture_dir/certs/agent.pem"
export DIPOLE_TEST_AGENT_RPC_KEY_FILE="$fixture_dir/certs/agent-key.pem"
export DIPOLE_TEST_AGENT_RPC_SERVER_NAME="core"
export DIPOLE_TEST_AGENT_RPC_STATE_PATH="$fixture_state"
export DIPOLE_TEST_AGENT_RPC_STALE_PATH="$fixture_stale"
export DIPOLE_TEST_AGENT_RPC_IDENTITY_DENIALS_VERIFIED="true"
export DIPOLE_AGENT_MCP_DRILL_EVIDENCE="$evidence_path"
export DIPOLE_AGENT_APPROVAL_GATE_DRILL=true
export DIPOLE_AGENT_APPROVAL_DRILL_EVIDENCE="$approval_evidence_path"
npm test -- --run \
  src/runtime/external-mcp-full-stack-drill.integration.test.ts \
  src/runtime/approval-gate-rpc-drill.integration.test.ts
if ! kill -0 "$fixture_pid" 2>/dev/null; then
  cat "$fixture_log" >&2
  echo "Agent Core RPC drill fixture exited before evidence validation" >&2
  exit 1
fi

test -s "$evidence_path"
npm run mcp:shadow-drill:check -- --evidence="$evidence_path"
npm run approval:drill:check -- --evidence="$approval_evidence_path"
