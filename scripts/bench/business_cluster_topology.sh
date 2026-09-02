#!/usr/bin/env bash
set -euo pipefail

# Isolated business topology lifecycle; volumes are retained by default.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
PROJECT="${BUSINESS_CLUSTER_PROJECT:-dipole-business-cluster}"
export DIPOLE_GATEWAY_PORT="${BUSINESS_CLUSTER_GATEWAY_PORT:-18080}"
COMPOSE=(
  docker compose
  --project-directory "${ROOT_DIR}"
  --project-name "${PROJECT}"
  -f "${ROOT_DIR}/deploy/compose/docker-compose.microservices.yml"
  -f "${ROOT_DIR}/deploy/compose/docker-compose.business-cluster.yml"
)

usage() {
  cat <<'EOF'
Usage: scripts/bench/business_cluster_topology.sh <up|status|down|config>

Active login sessions and GPU tasks are recorded for resource planning.
DIPOLE_INTERNAL_RPC_SHARED_SECRET must be set for Compose rendering.
BUSINESS_CLUSTER_GATEWAY_PORT controls the host Gateway port (default: 18080).
EOF
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || { echo "required command not found: $1" >&2; exit 2; }
}

guard_start() {
  local users gpu
  users="$(who 2>/dev/null | wc -l | tr -d ' ')"
  gpu="$(nvidia-smi --query-compute-apps=pid --format=csv,noheader 2>/dev/null | sed '/^[[:space:]]*$/d' | wc -l | tr -d ' ')"
  echo "business cluster resource snapshot: active_users=${users} gpu_processes=${gpu}" >&2
}

require_command docker
[[ -n "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:-}" ]] || {
  echo "DIPOLE_INTERNAL_RPC_SHARED_SECRET is required" >&2
  exit 2
}

case "${1:-}" in
  config)
    "${COMPOSE[@]}" config --quiet
    echo "business cluster config passed: project=${PROJECT}"
    ;;
  up)
    guard_start
    "${COMPOSE[@]}" config --quiet
    "${COMPOSE[@]}" up -d --wait --wait-timeout "${BUSINESS_CLUSTER_READY_TIMEOUT_SECONDS:-180}"
    "${COMPOSE[@]}" ps
    ;;
  status)
    "${COMPOSE[@]}" ps
    ;;
  down)
    "${COMPOSE[@]}" down --remove-orphans
    ;;
  *)
    usage
    exit 2
    ;;
esac
