#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="$root_dir/deploy/agent/external-mcp-shadow-drill.compose.yml"
project_name="${COMPOSE_PROJECT_NAME:-dipole-agent-mcp-drill-${RANDOM}-$$}"
evidence_path="${DIPOLE_AGENT_MCP_DRILL_EVIDENCE:-$root_dir/agent-runtime/.artifacts/external-mcp-shadow-drill.json}"

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
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

mkdir -p "$(dirname "$evidence_path")"
compose up -d --wait
sleep 5

cd "$root_dir/agent-runtime"
export DIPOLE_AGENT_FULL_STACK_DRILL=true
export DIPOLE_TEST_AGENT_MYSQL_URL="mysql://root:drill-root@127.0.0.1:${DIPOLE_AGENT_DRILL_MYSQL_PORT}/dipole"
export DIPOLE_TEST_AGENT_KAFKA_BROKERS="127.0.0.1:${DIPOLE_AGENT_DRILL_KAFKA_PORT}"
export DIPOLE_AGENT_MCP_DRILL_EVIDENCE="$evidence_path"
npm test -- --run src/runtime/external-mcp-full-stack-drill.integration.test.ts

test -s "$evidence_path"
npm run mcp:shadow-drill:check -- --evidence="$evidence_path"
