#!/usr/bin/env bash
set -euo pipefail

# Isolated business topology lifecycle; volumes are retained by default.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
PROJECT="${BUSINESS_CLUSTER_PROJECT:-dipole-business-cluster}"
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

BUSINESS_CLUSTER_ALLOW_ACTIVE=1 is required for approved active sessions.
DIPOLE_INTERNAL_RPC_SHARED_SECRET must be set for Compose rendering.
EOF
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || { echo "required command not found: $1" >&2; exit 2; }
}

guard_start() {
  local users gpu
  users="$(who 2>/dev/null | wc -l | tr -d ' ')"
  gpu="$(nvidia-smi --query-compute-apps=pid --format=csv,noheader 2>/dev/null | sed '/^[[:space:]]*$/d' | wc -l | tr -d ' ')"
  if [[ "${users}" != "0" && "${BUSINESS_CLUSTER_ALLOW_ACTIVE:-0}" != "1" ]]; then
    echo "business cluster start refused: active_users=${users}; explicit approval is required" >&2
    exit 3
  fi
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
