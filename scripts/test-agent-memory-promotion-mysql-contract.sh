#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
go_bin="${DIPOLE_GO_BIN:-go}"
port="${DIPOLE_AGENT_PROMOTION_MYSQL_PORT:-23316}"
project="dipole-agent-promotion-contract-$$"
compose_file="$root_dir/deploy/agent/memory-promotion-mysql-contract.compose.yml"

cleanup() {
  docker compose --project-name "$project" --file "$compose_file" down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
command -v "$go_bin" >/dev/null 2>&1 || { echo "Go binary is unavailable: $go_bin" >&2; exit 1; }

docker compose --project-name "$project" --file "$compose_file" up --wait
cd "$root_dir"
DIPOLE_TEST_MYSQL_ADMIN_DSN="root:contract-root@tcp(127.0.0.1:${port})/?parseTime=true&loc=UTC" \
LD_LIBRARY_PATH="/usr/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}" \
  "$go_bin" test ./internal/services/agent/infrastructure/mysql -run '^TestAgentMemoryPromotionReceiptCommitMySQLContract$' -count=1
echo "Agent Memory promotion MySQL receipt contract passed"
