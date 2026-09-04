#!/usr/bin/env bash
set -euo pipefail

# Runs only a disposable MySQL contract database. The Go test owns both Core
# mTLS listeners, restarts the listener, and removes its per-test schema.
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
go_bin="${DIPOLE_GO_BIN:-go}"
port="${DIPOLE_AGENT_OAUTH_LIFECYCLE_MYSQL_PORT:-23326}"
project="dipole-agent-oauth-lifecycle-restart-$$"
compose_file="$root_dir/deploy/agent/external-mcp-shadow-drill.compose.yml"

cleanup() {
  docker compose --project-name "$project" --file "$compose_file" down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
command -v "$go_bin" >/dev/null 2>&1 || { echo "Go binary is unavailable: $go_bin" >&2; exit 1; }
[[ "$port" =~ ^[0-9]{2,5}$ ]] || { echo "DIPOLE_AGENT_OAUTH_LIFECYCLE_MYSQL_PORT is invalid" >&2; exit 1; }

export DIPOLE_AGENT_DRILL_MYSQL_PORT="$port"
docker compose --project-name "$project" --file "$compose_file" up --wait mysql

cd "$root_dir"
DIPOLE_AGENT_OAUTH_LIFECYCLE_MTLS_RESTART=true \
DIPOLE_TEST_MYSQL_ADMIN_DSN="root:drill-root@tcp(127.0.0.1:${port})/?parseTime=true&loc=UTC" \
CGO_ENABLED=0 \
  "$go_bin" test ./internal/services/agent/infrastructure/mysql \
    -run '^TestAgentOAuthTokenLifecycleMySQLMTLSRestartContract$' -count=1 -v

echo "Agent OAuth token lifecycle MySQL/mTLS restart drill passed"
